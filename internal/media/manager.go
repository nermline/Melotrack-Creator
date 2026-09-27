package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/store"
	"gorm.io/gorm"
)

// Notifier is told about changes so connected editors update live.
type Notifier interface {
	MediaChanged(mediaID uint)
	MediaProgress(mediaID uint, progress float64)
	ItemChanged(itemID uint)
	ItemProgress(itemID uint, progress float64)
}

type Options struct {
	DownloadWorkers int
	RenderWorkers   int
	ClipHeight      int
}

// Manager owns every background job: downloads, uploads, metadata and renders.
//
// State lives in the database, not in goroutines: on start, anything that was
// interrupted is picked up again, and each render is keyed by its settings so a
// stale clip is detected by comparing keys rather than by remembering flags.
type Manager struct {
	db     *gorm.DB
	st     *Storage
	yt     *YtDlp
	ff     *FFmpeg
	meta   *MetaFetcher
	notify Notifier
	opt    Options

	downloads *queue
	renders   *queue
	metaSem   chan struct{}

	mu         sync.Mutex
	dlCancel   map[uint]context.CancelFunc
	uploads    map[uint]string // media id -> uploaded file waiting to be processed
	rendering  map[uint]runningRender
	debouncers map[uint]*time.Timer
	gcTimer    *time.Timer
	ctx        context.Context
}

type runningRender struct {
	key    string
	cancel context.CancelFunc
}

func NewManager(db *gorm.DB, st *Storage, yt *YtDlp, ff *FFmpeg, meta *MetaFetcher, opt Options) *Manager {
	return &Manager{
		db: db, st: st, yt: yt, ff: ff, meta: meta, opt: opt,
		notify:     nopNotifier{},
		downloads:  newQueue(),
		renders:    newQueue(),
		metaSem:    make(chan struct{}, 4),
		dlCancel:   map[uint]context.CancelFunc{},
		uploads:    map[uint]string{},
		rendering:  map[uint]runningRender{},
		debouncers: map[uint]*time.Timer{},
		ctx:        context.Background(),
	}
}

func (m *Manager) SetNotifier(n Notifier) { m.notify = n }
func (m *Manager) Storage() *Storage      { return m.st }

// Start launches the workers and resumes everything left unfinished.
func (m *Manager) Start(ctx context.Context) {
	m.ctx = ctx
	m.st.CleanTmp()
	for range m.opt.DownloadWorkers {
		go m.worker(ctx, m.downloads, m.runDownload)
	}
	for range m.opt.RenderWorkers {
		go m.worker(ctx, m.renders, m.runRender)
	}
	m.resume()
	m.ScheduleGC()
}

func (m *Manager) worker(ctx context.Context, q *queue, run func(context.Context, uint)) {
	for {
		id, ok := q.pop(ctx)
		if !ok {
			return
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("media worker panic", "id", id, "panic", r)
				}
			}()
			run(ctx, id)
		}()
	}
}

func (m *Manager) resume() {
	// Media whose download was interrupted, or whose file vanished.
	var medias []store.Media
	m.db.Find(&medias)
	for _, med := range medias {
		hasSource := fileExists(m.st.Source(med.SourceFile))
		switch {
		case IsUploadID(med.YouTubeID) && (med.Status == store.MediaDownloading || med.Status == store.MediaPending):
			// The uploaded file was in tmp/, which is wiped on start: nothing to resume.
			if hasSource {
				m.db.Model(&med).Update("status", store.MediaReady)
			} else {
				m.db.Model(&med).Updates(map[string]any{"status": store.MediaError, "error_code": "missing",
					"error": "Обробку файлу перервав перезапуск сервера, завантажте файл ще раз"})
			}
		case med.Status == store.MediaDownloading || med.Status == store.MediaPending:
			m.db.Model(&med).Update("status", store.MediaPending)
			m.downloads.push(med.ID)
		case med.Status == store.MediaReady && !hasSource:
			if IsUploadID(med.YouTubeID) {
				m.db.Model(&med).Updates(map[string]any{"status": store.MediaError, "error_code": "missing",
					"error": "Файл зник з диска, завантажте його знову"})
				continue
			}
			m.db.Model(&med).Update("status", store.MediaPending)
			m.downloads.push(med.ID)
		}
		if !IsUploadID(med.YouTubeID) && (med.Title == "" || med.ThumbFile == "" || !fileExists(m.st.Thumb(med.ThumbFile))) {
			m.fetchMetaAsync(med.ID)
		}
	}
	// Clips that are missing or stale.
	var ids []uint
	m.db.Model(&store.Item{}).Pluck("id", &ids)
	for _, id := range ids {
		m.Reconcile(id, 0)
	}
}

// MediaAdded is called after an item referencing the media was created.
func (m *Manager) MediaAdded(mediaID uint) {
	var med store.Media
	if m.db.First(&med, mediaID).Error != nil {
		return
	}
	if med.Status == store.MediaPending {
		m.downloads.push(med.ID)
	}
	if !IsUploadID(med.YouTubeID) && (med.Title == "" || med.ThumbFile == "") {
		m.fetchMetaAsync(med.ID)
	}
}

// RetryDownload re-queues a failed (or finished) download.
func (m *Manager) RetryDownload(mediaID uint) error {
	var med store.Media
	if err := m.db.First(&med, mediaID).Error; err != nil {
		return err
	}
	if IsUploadID(med.YouTubeID) {
		return errors.New("це завантажений файл, його можна лише замінити")
	}
	m.mu.Lock()
	_, running := m.dlCancel[mediaID]
	m.mu.Unlock()
	if running {
		return nil
	}
	m.db.Model(&med).Updates(map[string]any{"status": store.MediaPending, "error": "", "error_code": ""})
	m.notify.MediaChanged(mediaID)
	m.downloads.push(mediaID)
	return nil
}

// IngestUpload replaces the media's source with an uploaded file (the fallback
// when YouTube refuses to serve the server). tmpPath is taken over by the manager.
func (m *Manager) IngestUpload(mediaID uint, tmpPath string) {
	m.mu.Lock()
	if cancel, ok := m.dlCancel[mediaID]; ok {
		cancel()
	}
	if old, ok := m.uploads[mediaID]; ok && old != tmpPath {
		os.Remove(old)
	}
	m.uploads[mediaID] = tmpPath
	m.mu.Unlock()
	m.db.Model(&store.Media{}).Where("id = ?", mediaID).
		Updates(map[string]any{"status": store.MediaDownloading, "error": "", "error_code": ""})
	m.notify.MediaChanged(mediaID)
	m.downloads.push(mediaID)
}

func (m *Manager) runDownload(ctx context.Context, mediaID uint) {
	var med store.Media
	if m.db.First(&med, mediaID).Error != nil {
		return
	}
	m.mu.Lock()
	upload, isUpload := m.uploads[mediaID]
	delete(m.uploads, mediaID)
	dctx, cancel := context.WithCancel(ctx)
	m.dlCancel[mediaID] = cancel
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		delete(m.dlCancel, mediaID)
		m.mu.Unlock()
	}()

	if isUpload {
		m.processUpload(dctx, med, upload)
		return
	}
	if med.Status == store.MediaReady && fileExists(m.st.Source(med.SourceFile)) {
		return
	}
	if IsUploadID(med.YouTubeID) {
		return
	}

	m.db.Model(&med).Updates(map[string]any{"status": store.MediaDownloading, "error": "", "error_code": ""})
	m.notify.MediaChanged(mediaID)

	workDir, err := os.MkdirTemp(m.st.TmpDir(), "dl-"+med.YouTubeID+"-")
	if err != nil {
		m.failMedia(med.ID, "failed", "Не вдалося створити тимчасову теку", err.Error())
		return
	}
	defer os.RemoveAll(workDir)

	progress := throttle(func(f float64) { m.notify.MediaProgress(mediaID, f) })
	slog.Info("download started", "youtube", med.YouTubeID)
	res, err := m.yt.Download(dctx, med.YouTubeID, workDir, progress)
	if err != nil {
		if dctx.Err() != nil {
			return // cancelled: an upload replaced it, or the server is stopping
		}
		var de *DownloadError
		if errors.As(err, &de) {
			slog.Warn("download failed", "youtube", med.YouTubeID, "code", de.Code, "detail", de.Detail)
			m.failMedia(med.ID, de.Code, de.Message, de.Detail)
		} else {
			slog.Warn("download failed", "youtube", med.YouTubeID, "err", err)
			m.failMedia(med.ID, "failed", "Не вдалося завантажити відео", err.Error())
		}
		return
	}
	if err := m.installSource(dctx, med, res.File, "youtube", res.Title); err != nil {
		m.failMedia(med.ID, "failed", "Завантажений файл не вдалося прочитати", err.Error())
	}
}

func (m *Manager) processUpload(ctx context.Context, med store.Media, upload string) {
	defer os.Remove(upload)
	probe, err := m.ff.Probe(ctx, upload)
	if err != nil {
		m.failMedia(med.ID, "bad_upload", "Файл не схожий на відео чи аудіо", err.Error())
		return
	}
	out := filepath.Join(m.st.TmpDir(), fmt.Sprintf("up-%d-%d.mp4", med.ID, time.Now().UnixNano()))
	defer os.Remove(out)
	progress := throttle(func(f float64) { m.notify.MediaProgress(med.ID, f) })
	if err := m.ff.Run(ctx, NormalizeArgs(upload, out, probe), probe.Duration, progress); err != nil {
		if ctx.Err() == nil {
			m.failMedia(med.ID, "bad_upload", "Не вдалося обробити файл", err.Error())
		}
		return
	}
	if err := m.installSource(ctx, med, out, "upload", ""); err != nil {
		m.failMedia(med.ID, "bad_upload", "Не вдалося обробити файл", err.Error())
	}
}

// installSource moves a finished file into place and marks the media ready.
func (m *Manager) installSource(ctx context.Context, med store.Media, file, kind, title string) error {
	probe, err := m.ff.Probe(ctx, file)
	if err != nil {
		return err
	}
	name := med.YouTubeID + ".mp4"
	if err := moveFile(file, m.st.Source(name)); err != nil {
		return err
	}
	updates := map[string]any{
		"status": store.MediaReady, "error": "", "error_code": "",
		"source_file": name, "source_kind": kind,
		"width": probe.Width, "height": probe.Height, "duration": probe.Duration,
		"version": gorm.Expr("version + 1"),
	}
	if med.Title == "" && title != "" {
		updates["title"] = title
	}
	if err := m.db.Model(&store.Media{}).Where("id = ?", med.ID).Updates(updates).Error; err != nil {
		return err
	}
	slog.Info("source ready", "media", med.YouTubeID, "kind", kind, "w", probe.Width, "h", probe.Height, "dur", probe.Duration)
	if med.Title == "" && title != "" {
		m.fillAnswers(med.ID, SuggestAnswer(title, ""))
	}
	m.notify.MediaChanged(med.ID)
	m.rerenderMedia(med.ID)
	return nil
}

func (m *Manager) failMedia(mediaID uint, code, msg, detail string) {
	if detail != "" && !strings.Contains(msg, detail) {
		msg = msg + "\n" + truncate(detail, 400)
	}
	m.db.Model(&store.Media{}).Where("id = ?", mediaID).
		Updates(map[string]any{"status": store.MediaError, "error_code": code, "error": msg})
	m.notify.MediaChanged(mediaID)
}

func (m *Manager) fetchMetaAsync(mediaID uint) {
	if m.meta == nil {
		return
	}
	go func() {
		m.metaSem <- struct{}{}
		defer func() { <-m.metaSem }()
		m.fetchMeta(mediaID)
	}()
}

func (m *Manager) fetchMeta(mediaID uint) {
	var med store.Media
	if m.db.First(&med, mediaID).Error != nil {
		return
	}
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()
	changed := false
	if med.Title == "" {
		if meta, err := m.meta.OEmbed(ctx, med.YouTubeID); err == nil && meta.Title != "" {
			m.db.Model(&med).Updates(map[string]any{"title": meta.Title, "channel": meta.Channel})
			m.fillAnswers(med.ID, SuggestAnswer(meta.Title, meta.Channel))
			changed = true
		} else if err != nil {
			slog.Info("oembed unavailable", "youtube", med.YouTubeID, "err", err)
		}
	}
	if med.ThumbFile == "" || !fileExists(m.st.Thumb(med.ThumbFile)) {
		name := med.YouTubeID + ".jpg"
		if err := m.meta.Thumbnail(ctx, med.YouTubeID, m.st.Thumb(name)); err == nil {
			m.db.Model(&med).Update("thumb_file", name)
			changed = true
		} else {
			slog.Info("thumbnail unavailable", "youtube", med.YouTubeID, "err", err)
		}
	}
	if changed {
		m.notify.MediaChanged(mediaID)
	}
}

// fillAnswers pre-fills answers the editor has not typed yet.
func (m *Manager) fillAnswers(mediaID uint, answer string) {
	if answer == "" {
		return
	}
	var ids []uint
	m.db.Model(&store.Item{}).Where("media_id = ? AND answer = ''", mediaID).Pluck("id", &ids)
	if len(ids) == 0 {
		return
	}
	m.db.Model(&store.Item{}).Where("id IN ? AND answer = ''", ids).Update("answer", answer)
	for _, id := range ids {
		m.notify.ItemChanged(id)
	}
}

func (m *Manager) rerenderMedia(mediaID uint) {
	var ids []uint
	m.db.Model(&store.Item{}).Where("media_id = ?", mediaID).Pluck("id", &ids)
	for _, id := range ids {
		m.Reconcile(id, 0)
	}
}

// SpecFor builds the render spec for an item's current settings.
func (m *Manager) SpecFor(it *store.Item, med *store.Media) ClipSpec {
	return ClipSpec{
		Source: m.st.Source(med.SourceFile),
		Start:  it.StartSec, End: it.EndSec, GainDB: it.GainDB,
		Frame: it.Frame, CropX: it.CropX, CropY: it.CropY, CropW: it.CropW, CropH: it.CropH,
		SrcW: med.Width, SrcH: med.Height,
		HasVideo: med.Width > 0 && med.Height > 0,
		HasAudio: true,
		Height:   m.opt.ClipHeight,
	}
}

// DesiredKey is the clip key matching the item's current settings.
func (m *Manager) DesiredKey(it *store.Item, med *store.Media) string {
	return ClipKey(med.ID, med.Version, m.SpecFor(it, med))
}

// Reconcile brings an item's clip in line with its settings: nothing to do when the
// clip matches, otherwise the item is marked pending and a render is scheduled
// after `delay` (so a burst of edits causes a single render).
func (m *Manager) Reconcile(itemID uint, delay time.Duration) {
	var it store.Item
	if m.db.Preload("Media").First(&it, itemID).Error != nil {
		return
	}
	if it.Media.Status != store.MediaReady {
		if it.ClipStatus != store.ClipPending {
			m.db.Model(&it).Updates(map[string]any{"clip_status": store.ClipPending, "clip_error": ""})
			m.notify.ItemChanged(itemID)
		}
		return
	}
	key := m.DesiredKey(&it, &it.Media)
	if it.ClipKey == key && fileExists(m.st.Clip(it.ClipFile)) {
		if it.ClipStatus != store.ClipReady {
			m.db.Model(&it).Updates(map[string]any{"clip_status": store.ClipReady, "clip_error": ""})
			m.notify.ItemChanged(itemID)
		}
		return
	}
	m.mu.Lock()
	if r, ok := m.rendering[itemID]; ok && r.key != key {
		r.cancel()
	}
	if t, ok := m.debouncers[itemID]; ok {
		t.Stop()
	}
	m.debouncers[itemID] = time.AfterFunc(delay, func() {
		m.mu.Lock()
		delete(m.debouncers, itemID)
		m.mu.Unlock()
		m.renders.push(itemID)
	})
	m.mu.Unlock()
	if it.ClipStatus != store.ClipPending && it.ClipStatus != store.ClipRendering {
		m.db.Model(&it).Updates(map[string]any{"clip_status": store.ClipPending, "clip_error": ""})
		m.notify.ItemChanged(itemID)
	}
}

// ForceRender drops the current clip key so the item is rendered again.
func (m *Manager) ForceRender(itemID uint) {
	m.db.Model(&store.Item{}).Where("id = ?", itemID).Update("clip_key", "")
	m.Reconcile(itemID, 0)
}

// ItemDeleted cancels work for a deleted item and schedules cleanup of its files.
func (m *Manager) ItemDeleted(itemID uint) {
	m.mu.Lock()
	if r, ok := m.rendering[itemID]; ok {
		r.cancel()
	}
	if t, ok := m.debouncers[itemID]; ok {
		t.Stop()
		delete(m.debouncers, itemID)
	}
	m.mu.Unlock()
	m.ScheduleGC()
}

func (m *Manager) runRender(ctx context.Context, itemID uint) {
	var it store.Item
	if m.db.Preload("Media").First(&it, itemID).Error != nil {
		return
	}
	med := it.Media
	if med.Status != store.MediaReady {
		return // rendered once the download finishes
	}
	spec := m.SpecFor(&it, &med)
	key := ClipKey(med.ID, med.Version, spec)
	if it.ClipKey == key && fileExists(m.st.Clip(it.ClipFile)) {
		return
	}

	rctx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.rendering[itemID] = runningRender{key: key, cancel: cancel}
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		if r, ok := m.rendering[itemID]; ok && r.key == key {
			delete(m.rendering, itemID)
		}
		m.mu.Unlock()
	}()

	m.db.Model(&it).Updates(map[string]any{"clip_status": store.ClipRendering, "clip_error": ""})
	m.notify.ItemChanged(itemID)

	name := fmt.Sprintf("%d-%s.mp4", it.ID, key)
	tmp := filepath.Join(m.st.TmpDir(), name)
	spec.Out = tmp
	started := time.Now()
	progress := throttle(func(f float64) { m.notify.ItemProgress(itemID, f) })
	err := m.ff.Run(rctx, ClipArgs(spec), spec.End-spec.Start, progress)
	if err != nil {
		os.Remove(tmp)
		if rctx.Err() != nil {
			return // superseded by newer settings or shutting down
		}
		slog.Warn("render failed", "item", itemID, "err", err)
		m.db.Model(&store.Item{}).Where("id = ?", itemID).
			Updates(map[string]any{"clip_status": store.ClipError, "clip_error": truncate(err.Error(), 400)})
		m.notify.ItemChanged(itemID)
		return
	}
	if err := moveFile(tmp, m.st.Clip(name)); err != nil {
		os.Remove(tmp)
		m.db.Model(&store.Item{}).Where("id = ?", itemID).
			Updates(map[string]any{"clip_status": store.ClipError, "clip_error": err.Error()})
		m.notify.ItemChanged(itemID)
		return
	}

	// Settings may have changed while rendering; only publish a matching clip.
	var cur store.Item
	if m.db.Preload("Media").First(&cur, itemID).Error != nil {
		os.Remove(m.st.Clip(name))
		return
	}
	if m.DesiredKey(&cur, &cur.Media) != key {
		os.Remove(m.st.Clip(name))
		m.Reconcile(itemID, 0)
		return
	}
	old := cur.ClipFile
	m.db.Model(&store.Item{}).Where("id = ?", itemID).Updates(map[string]any{
		"clip_status": store.ClipReady, "clip_key": key, "clip_file": name, "clip_error": "",
	})
	if old != "" && old != name {
		m.ScheduleGC() // the old clip may still be shared with a duplicated project
	}
	slog.Info("clip rendered", "item", itemID, "took", time.Since(started).Round(time.Millisecond))
	m.notify.ItemChanged(itemID)
}

// QueueSizes reports how many downloads and renders are waiting.
func (m *Manager) QueueSizes() (downloads, renders int) {
	return m.downloads.len(), m.renders.len()
}

// ScheduleGC removes unreferenced media and files a few seconds after deletions.
func (m *Manager) ScheduleGC() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gcTimer != nil {
		m.gcTimer.Stop()
	}
	m.gcTimer = time.AfterFunc(5*time.Second, m.gc)
}

func (m *Manager) gc() {
	// Media no longer used by any item.
	var orphans []store.Media
	m.db.Where("id NOT IN (SELECT DISTINCT media_id FROM items)").Find(&orphans)
	for _, med := range orphans {
		m.mu.Lock()
		if cancel, ok := m.dlCancel[med.ID]; ok {
			cancel()
		}
		m.mu.Unlock()
		if m.db.Delete(&store.Media{}, med.ID).Error == nil {
			slog.Info("media removed (unused)", "media", med.YouTubeID)
		}
	}

	keep := func(table, column string) map[string]bool {
		var names []string
		m.db.Table(table).Where(column+" != ''").Pluck(column, &names)
		set := make(map[string]bool, len(names))
		for _, n := range names {
			set[n] = true
		}
		return set
	}
	sweep(m.st.SourceDir(), keep("media", "source_file"))
	sweep(m.st.ThumbsDir(), keep("media", "thumb_file"))
	sweep(m.st.ClipsDir(), keep("items", "clip_file"))
	sweep(m.st.ImagesDir(), keep("items", "image_file"))
}

// sweep deletes files not in keep. Very recent files are spared: they may belong
// to work that finished a moment ago and is not in the database yet.
func sweep(dir string, keep map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || keep[e.Name()] {
			continue
		}
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < 10*time.Minute {
			continue
		}
		if os.Remove(filepath.Join(dir, e.Name())) == nil {
			slog.Info("removed unused file", "file", filepath.Join(filepath.Base(dir), e.Name()))
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// throttle limits progress callbacks to a few per second.
func throttle(fn func(float64)) func(float64) {
	var mu sync.Mutex
	var last time.Time
	return func(f float64) {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(last) < 400*time.Millisecond && f < 1 {
			return
		}
		last = time.Now()
		fn(f)
	}
}

type nopNotifier struct{}

func (nopNotifier) MediaChanged(uint)           {}
func (nopNotifier) MediaProgress(uint, float64) {}
func (nopNotifier) ItemChanged(uint)            {}
func (nopNotifier) ItemProgress(uint, float64)  {}

// queue is a FIFO of ids without duplicates.
type queue struct {
	mu     sync.Mutex
	ids    []uint
	set    map[uint]struct{}
	signal chan struct{}
}

func newQueue() *queue {
	return &queue{set: map[uint]struct{}{}, signal: make(chan struct{}, 1)}
}

func (q *queue) push(id uint) {
	q.mu.Lock()
	if _, ok := q.set[id]; !ok {
		q.set[id] = struct{}{}
		q.ids = append(q.ids, id)
	}
	q.mu.Unlock()
	q.wake()
}

func (q *queue) wake() {
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

func (q *queue) pop(ctx context.Context) (uint, bool) {
	for {
		q.mu.Lock()
		if len(q.ids) > 0 {
			id := q.ids[0]
			q.ids = q.ids[1:]
			delete(q.set, id)
			more := len(q.ids) > 0
			q.mu.Unlock()
			if more {
				q.wake()
			}
			return id, true
		}
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return 0, false
		case <-q.signal:
		}
	}
}

func (q *queue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.ids)
}
