package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/store"
)

func newTestManager(t *testing.T) (*Manager, *Storage) {
	t.Helper()
	dir := t.TempDir()
	st, err := NewStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(dir, "m.db"), store.LegacyImport{})
	if err != nil {
		t.Fatal(err)
	}
	ff := &FFmpeg{FFmpeg: "/nonexistent", FFprobe: "/nonexistent"}
	m := NewManager(db, st, ff, nil, LocalDownloader{YT: &YtDlp{Path: "/nonexistent"}}, LocalRenderer{FF: ff},
		Options{DownloadWorkers: 1, RenderWorkers: 1, ClipHeight: 720})
	return m, st
}

// An upload interrupted by a restart must not stay "queued" forever.
func TestResumeInterruptedUpload(t *testing.T) {
	m, _ := newTestManager(t)
	med := store.Media{YouTubeID: "file-abc", Status: store.MediaDownloading, SourceKind: "upload"}
	m.db.Create(&med)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	var got store.Media
	m.db.First(&got, med.ID)
	if got.Status != store.MediaError || got.ErrorCode != "missing" {
		t.Fatalf("status = %s/%s, want error/missing", got.Status, got.ErrorCode)
	}
}

// A ready media whose file vanished is downloaded again.
func TestResumeMissingSource(t *testing.T) {
	m, _ := newTestManager(t)
	med := store.Media{YouTubeID: "dQw4w9WgXcQ", Status: store.MediaReady, SourceKind: "youtube", SourceFile: "gone.mp4", Title: "x", ThumbFile: "x.jpg"}
	m.db.Create(&med)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// The download fails fast (no yt-dlp) and the reason is recorded.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var got store.Media
		m.db.First(&got, med.ID)
		if got.Status == store.MediaError {
			if got.ErrorCode != "ytdlp_missing" {
				t.Fatalf("error code = %s", got.ErrorCode)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("missing source was not re-downloaded")
}

func TestSweepKeepsReferencedAndRecentFiles(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	for _, n := range []string{"keep.mp4", "old.mp4", "fresh.mp4"} {
		p := filepath.Join(dir, n)
		os.WriteFile(p, []byte("x"), 0o644)
		if n != "fresh.mp4" {
			os.Chtimes(p, old, old)
		}
	}
	sweep(dir, map[string]bool{"keep.mp4": true})
	for n, want := range map[string]bool{"keep.mp4": true, "old.mp4": false, "fresh.mp4": true} {
		_, err := os.Stat(filepath.Join(dir, n))
		if (err == nil) != want {
			t.Errorf("%s exists=%v, want %v", n, err == nil, want)
		}
	}
}

type offlineDownloader struct{}

func (offlineDownloader) Download(context.Context, string, string, func(float64)) (DownloadResult, error) {
	return DownloadResult{}, ErrWorkerUnavailable
}

// When the download worker is offline the song waits instead of failing.
func TestWorkerOfflineKeepsDownloadPending(t *testing.T) {
	m, _ := newTestManager(t)
	m.dl = offlineDownloader{}
	m.opt.RetryDelay = time.Hour
	med := store.Media{YouTubeID: "dQw4w9WgXcQ", Status: store.MediaPending, SourceKind: "youtube"}
	m.db.Create(&med)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var got store.Media
		m.db.First(&got, med.ID)
		if got.ErrorCode == "worker_offline" {
			if got.Status != store.MediaPending {
				t.Fatalf("status = %s, want pending", got.Status)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("offline worker was not reported")
}

// A pasted song is not cut until the organiser saves it in the editor.
func TestUnconfiguredItemIsNotRendered(t *testing.T) {
	m, st := newTestManager(t)
	os.WriteFile(filepath.Join(st.SourceDir(), "src.mp4"), []byte("x"), 0o644)
	med := store.Media{YouTubeID: "dQw4w9WgXcQ", Status: store.MediaReady, SourceKind: "youtube", SourceFile: "src.mp4"}
	m.db.Create(&med)
	proj := store.Project{Title: "p"}
	m.db.Create(&proj)
	cat := store.Category{ProjectID: proj.ID, Title: "c"}
	m.db.Create(&cat)
	it := store.Item{CategoryID: cat.ID, MediaID: med.ID, EndSec: 15, Frame: store.FrameFit, ClipStatus: store.ClipPending}
	m.db.Omit("Media").Create(&it)

	m.Reconcile(it.ID, 0)
	if _, renders := m.QueueSizes(); renders != 0 {
		t.Fatalf("unconfigured item queued for rendering")
	}
	m.db.Model(&it).Update("configured", true)
	m.Reconcile(it.ID, 0)
	time.Sleep(50 * time.Millisecond)
	if _, renders := m.QueueSizes(); renders != 1 {
		t.Fatalf("configured item not queued (renders = %d)", renders)
	}
}
