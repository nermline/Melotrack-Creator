package api

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/media"
	"github.com/nermline/Melotrack-Creator/internal/store"
	"golang.org/x/image/draw"
	"gorm.io/gorm"
)

const defaultClipSeconds = 15

func (s *Server) loadItem(c *gin.Context) (*store.Item, uint, bool) {
	iid, ok := idParam(c, "iid")
	if !ok {
		return nil, 0, false
	}
	var it store.Item
	if err := s.db.Preload("Media").First(&it, iid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			notFound(c, "Пісню")
		} else {
			failInternal(c, err)
		}
		return nil, 0, false
	}
	pid, _ := s.projectOfItem(it.ID)
	return &it, pid, true
}

func (s *Server) itemResponse(c *gin.Context, status int, id uint) {
	var it store.Item
	if err := s.db.Preload("Media").First(&it, id).Error; err != nil {
		failInternal(c, err)
		return
	}
	c.JSON(status, toItemDTO(&it))
}

// findOrCreateMedia returns the media row for a YouTube id, creating it when new.
func findOrCreateMedia(tx *gorm.DB, ytID string) (store.Media, error) {
	var m store.Media
	err := tx.Where("youtube_id = ?", ytID).First(&m).Error
	if err == nil {
		return m, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return m, err
	}
	m = store.Media{YouTubeID: ytID, Status: store.MediaPending, SourceKind: "youtube"}
	return m, tx.Create(&m).Error
}

func nextItemPosition(tx *gorm.DB, categoryID uint) int {
	var maxPos *int
	tx.Model(&store.Item{}).Where("category_id = ?", categoryID).Select("MAX(position)").Scan(&maxPos)
	if maxPos == nil {
		return 0
	}
	return *maxPos + 1
}

// parseLinks accepts links one per line (or several per line separated by
// spaces/commas) and reports lines that contain no YouTube link.
func parseLinks(inputs []string) (ids, invalid []string) {
	invalid = []string{}
	for _, raw := range inputs {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			found := false
			for _, tok := range strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == '\t' || r == ',' || r == ';' }) {
				if id := media.ParseYouTubeID(tok); id != "" {
					ids = append(ids, id)
					found = true
				}
			}
			if !found {
				invalid = append(invalid, line)
			}
		}
	}
	return ids, invalid
}

func defaultEnd(m *store.Media) float64 {
	if m.Duration > 0 && m.Duration < defaultClipSeconds {
		return math.Floor(m.Duration*10) / 10
	}
	return defaultClipSeconds
}

// createItems adds one song per YouTube link; several links (one per line) may
// be pasted at once.
func (s *Server) createItems(c *gin.Context) {
	cat, ok := s.loadCategory(c)
	if !ok {
		return
	}
	var in struct {
		URLs []string `json:"urls"`
	}
	if !bindJSON(c, &in) {
		return
	}
	ids, invalid := parseLinks(in.URLs)
	if len(ids) == 0 {
		badRequest(c, "Не знайдено жодного посилання на YouTube")
		return
	}
	if len(ids) > 50 {
		badRequest(c, "Не більше 50 посилань за раз")
		return
	}

	var created []uint
	var mediaIDs []uint
	err := s.db.Transaction(func(tx *gorm.DB) error {
		pos := nextItemPosition(tx, cat.ID)
		for _, ytID := range ids {
			m, err := findOrCreateMedia(tx, ytID)
			if err != nil {
				return err
			}
			it := store.Item{
				CategoryID: cat.ID, Position: pos, MediaID: m.ID,
				StartSec: 0, EndSec: defaultEnd(&m), Frame: store.FrameFit, ClipStatus: store.ClipPending,
			}
			if m.Title != "" {
				it.Answer = media.SuggestAnswer(m.Title, m.Channel)
			}
			if err := tx.Omit("Media").Create(&it).Error; err != nil {
				return err
			}
			pos++
			created = append(created, it.ID)
			mediaIDs = append(mediaIDs, m.ID)
		}
		return nil
	})
	if err != nil {
		failInternal(c, err)
		return
	}
	for i, id := range created {
		s.media.MediaAdded(mediaIDs[i])
		s.media.Reconcile(id, 0)
	}
	s.changed(cat.ProjectID)

	var items []store.Item
	s.db.Preload("Media").Where("id IN ?", created).Order("position").Find(&items)
	out := make([]itemDTO, 0, len(items))
	for i := range items {
		out = append(out, toItemDTO(&items[i]))
	}
	c.JSON(http.StatusCreated, gin.H{"items": out, "invalid": invalid})
}

// createUploadItem adds a song from a local audio/video file (no YouTube at all).
func (s *Server) createUploadItem(c *gin.Context) {
	cat, ok := s.loadCategory(c)
	if !ok {
		return
	}
	tmp, name, ok := s.receiveUpload(c, "file")
	if !ok {
		return
	}
	answer := strings.TrimSpace(c.PostForm("answer"))
	if answer == "" {
		answer = strings.TrimSuffix(name, filepath.Ext(name))
	}
	var it store.Item
	var m store.Media
	err := s.db.Transaction(func(tx *gorm.DB) error {
		m = store.Media{YouTubeID: "file-" + store.NewJoinCode() + store.NewJoinCode(), Status: store.MediaDownloading,
			SourceKind: "upload", Title: truncateRunes(name, 200)}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		it = store.Item{CategoryID: cat.ID, Position: nextItemPosition(tx, cat.ID), MediaID: m.ID,
			Answer: truncateRunes(answer, 300), EndSec: defaultClipSeconds, Frame: store.FrameFit, ClipStatus: store.ClipPending}
		return tx.Omit("Media").Create(&it).Error
	})
	if err != nil {
		os.Remove(tmp)
		failInternal(c, err)
		return
	}
	s.media.IngestUpload(m.ID, tmp)
	s.changed(cat.ProjectID)
	s.itemResponse(c, http.StatusCreated, it.ID)
}

// receiveUpload stores a multipart file in the tmp dir.
func (s *Server) receiveUpload(c *gin.Context, field string) (path, name string, ok bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.cfg.MaxUploadMB<<20)
	fh, err := c.FormFile(field)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			fail(c, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("Файл більший за %d МБ", s.cfg.MaxUploadMB))
			return "", "", false
		}
		badRequest(c, "Файл не отримано")
		return "", "", false
	}
	dst := filepath.Join(s.media.Storage().TmpDir(), fmt.Sprintf("upload-%d%s", time.Now().UnixNano(), strings.ToLower(filepath.Ext(fh.Filename))))
	if err := c.SaveUploadedFile(fh, dst); err != nil {
		failInternal(c, err)
		return "", "", false
	}
	return dst, filepath.Base(fh.Filename), true
}

type itemPatch struct {
	Answer     *string  `json:"answer"`
	ShowVideo  *bool    `json:"show_video"`
	Start      *float64 `json:"start"`
	End        *float64 `json:"end"`
	GainDB     *float64 `json:"gain_db"`
	Frame      *string  `json:"frame"`
	Crop       *rect    `json:"crop"`
	YouTubeURL *string  `json:"youtube_url"`
	CategoryID *uint    `json:"category_id"`
}

func (s *Server) updateItem(c *gin.Context) {
	it, pid, ok := s.loadItem(c)
	if !ok {
		return
	}
	var in itemPatch
	if !bindJSON(c, &in) {
		return
	}

	newMedia := false
	moved := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if in.YouTubeURL != nil {
			ytID := media.ParseYouTubeID(*in.YouTubeURL)
			if ytID == "" {
				return validationError("Це не схоже на посилання YouTube")
			}
			if ytID != it.Media.YouTubeID {
				m, err := findOrCreateMedia(tx, ytID)
				if err != nil {
					return err
				}
				it.MediaID, it.Media = m.ID, m
				it.Frame, it.CropX, it.CropY, it.CropW, it.CropH = store.FrameFit, 0, 0, 0, 0
				it.StartSec, it.EndSec = 0, defaultEnd(&m)
				newMedia = true
			}
		}
		if in.CategoryID != nil && *in.CategoryID != it.CategoryID {
			var target store.Category
			if err := tx.First(&target, *in.CategoryID).Error; err != nil || target.ProjectID != pid {
				return validationError("Категорію не знайдено в цьому проєкті")
			}
			old := it.CategoryID
			it.CategoryID = target.ID
			it.Position = nextItemPosition(tx, target.ID)
			moved = true
			defer func() { _ = renumber(tx, &store.Item{}, "category_id", old) }()
		}
		if in.Answer != nil {
			a := strings.TrimSpace(*in.Answer)
			if utf8.RuneCountInString(a) > 300 {
				return validationError("Відповідь задовга (до 300 символів)")
			}
			it.Answer = a
		}
		if in.ShowVideo != nil {
			it.ShowVideo = *in.ShowVideo
		}
		if in.Start != nil {
			it.StartSec = round1(*in.Start)
		}
		if in.End != nil {
			it.EndSec = round1(*in.End)
		}
		if in.GainDB != nil {
			it.GainDB = round1(*in.GainDB)
		}
		if in.Frame != nil {
			it.Frame = *in.Frame
		}
		if in.Crop != nil {
			it.CropX, it.CropY, it.CropW, it.CropH = in.Crop.X, in.Crop.Y, in.Crop.W, in.Crop.H
		}
		if err := validateItem(it); err != nil {
			return err
		}
		return tx.Omit("Media").Save(it).Error
	})
	var ve validationError
	if errors.As(err, &ve) {
		badRequest(c, string(ve))
		return
	}
	if err != nil {
		failInternal(c, err)
		return
	}
	if newMedia {
		s.media.MediaAdded(it.MediaID)
		s.media.ScheduleGC()
	}
	// Debounced: dragging a slider produces one render, not twenty.
	s.media.Reconcile(it.ID, 800*time.Millisecond)
	if newMedia || moved {
		s.changed(pid)
	} else {
		s.ItemChanged(it.ID)
	}
	s.itemResponse(c, http.StatusOK, it.ID)
}

type validationError string

func (v validationError) Error() string { return string(v) }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func validateItem(it *store.Item) error {
	if it.StartSec < 0 {
		return validationError("Початок не може бути від'ємним")
	}
	if it.EndSec-it.StartSec < 1 {
		return validationError("Фрагмент має тривати щонайменше 1 секунду")
	}
	if it.EndSec-it.StartSec > 600 {
		return validationError("Фрагмент не може бути довшим за 10 хвилин")
	}
	if d := it.Media.Duration; d > 0 {
		if it.StartSec >= d {
			return validationError(fmt.Sprintf("Початок виходить за межі відео (%.1f с)", d))
		}
		if it.EndSec > d+0.05 {
			it.EndSec = math.Floor(d*10) / 10
		}
	}
	if it.GainDB < -20 || it.GainDB > 20 {
		return validationError("Підсилення: від −20 до +20 дБ")
	}
	switch it.Frame {
	case store.FrameFit:
		it.CropX, it.CropY, it.CropW, it.CropH = 0, 0, 0, 0
	case store.FrameCrop:
		if it.Media.Width > 0 && it.Media.Height > 0 {
			x, y, w, h, ok := media.NormalizeCrop(it.CropX, it.CropY, it.CropW, it.CropH, it.Media.Width, it.Media.Height)
			if !ok {
				return validationError("Рамка обрізання замала або поза кадром")
			}
			it.CropX, it.CropY, it.CropW, it.CropH = x, y, w, h
		} else if it.CropW <= 0 || it.CropH <= 0 {
			return validationError("Вкажіть рамку обрізання")
		}
	default:
		return validationError("Невідомий режим кадру")
	}
	return nil
}

func (s *Server) deleteItem(c *gin.Context) {
	it, pid, ok := s.loadItem(c)
	if !ok {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&store.Item{}, it.ID).Error; err != nil {
			return err
		}
		return renumber(tx, &store.Item{}, "category_id", it.CategoryID)
	})
	if err != nil {
		failInternal(c, err)
		return
	}
	s.media.ItemDeleted(it.ID)
	s.changed(pid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) orderItems(c *gin.Context) {
	cat, ok := s.loadCategory(c)
	if !ok {
		return
	}
	var in struct {
		IDs []uint `json:"ids"`
	}
	if !bindJSON(c, &in) {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		return applyOrder(tx, &store.Item{}, "category_id", cat.ID, in.IDs)
	})
	if errors.Is(err, errOrderMismatch) {
		fail(c, http.StatusConflict, "stale", "Список змінився, оновіть сторінку")
		return
	}
	if err != nil {
		failInternal(c, err)
		return
	}
	s.changed(cat.ProjectID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// uploadImage stores the answer picture (re-encoded, at most 1000px).
func (s *Server) uploadImage(c *gin.Context) {
	it, _, ok := s.loadItem(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 25<<20)
	fh, err := c.FormFile("image")
	if err != nil {
		badRequest(c, "Зображення не отримано (до 25 МБ)")
		return
	}
	f, err := fh.Open()
	if err != nil {
		failInternal(c, err)
		return
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		badRequest(c, "Непідтримуваний формат зображення (потрібен JPEG, PNG або GIF)")
		return
	}
	img = fitImage(img, 1000)
	name := fmt.Sprintf("%d-%s.jpg", it.ID, store.NewJoinCode())
	path := s.media.Storage().Image(name)
	out, err := os.Create(path)
	if err != nil {
		failInternal(c, err)
		return
	}
	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: 88}); err != nil {
		out.Close()
		os.Remove(path)
		failInternal(c, err)
		return
	}
	if err := out.Close(); err != nil {
		failInternal(c, err)
		return
	}
	if err := s.db.Model(&store.Item{}).Where("id = ?", it.ID).Update("image_file", name).Error; err != nil {
		failInternal(c, err)
		return
	}
	s.media.ScheduleGC()
	s.ItemChanged(it.ID)
	s.itemResponse(c, http.StatusOK, it.ID)
}

func fitImage(src image.Image, maxSide int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return src
	}
	scale := float64(maxSide) / float64(max(w, h))
	dst := image.NewRGBA(image.Rect(0, 0, max(int(float64(w)*scale), 1), max(int(float64(h)*scale), 1)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func (s *Server) deleteImage(c *gin.Context) {
	it, _, ok := s.loadItem(c)
	if !ok {
		return
	}
	if err := s.db.Model(&store.Item{}).Where("id = ?", it.ID).Update("image_file", "").Error; err != nil {
		failInternal(c, err)
		return
	}
	s.media.ScheduleGC()
	s.ItemChanged(it.ID)
	s.itemResponse(c, http.StatusOK, it.ID)
}

// uploadSource replaces the item's source video with a file — the way around
// YouTube refusing to serve the server.
func (s *Server) uploadSource(c *gin.Context) {
	it, _, ok := s.loadItem(c)
	if !ok {
		return
	}
	tmp, _, ok := s.receiveUpload(c, "file")
	if !ok {
		return
	}
	s.media.IngestUpload(it.MediaID, tmp)
	s.itemResponse(c, http.StatusAccepted, it.ID)
}

func (s *Server) retryDownload(c *gin.Context) {
	it, _, ok := s.loadItem(c)
	if !ok {
		return
	}
	if err := s.media.RetryDownload(it.MediaID); err != nil {
		badRequest(c, err.Error())
		return
	}
	s.itemResponse(c, http.StatusAccepted, it.ID)
}

func (s *Server) forceRender(c *gin.Context) {
	it, _, ok := s.loadItem(c)
	if !ok {
		return
	}
	s.media.ForceRender(it.ID)
	s.itemResponse(c, http.StatusAccepted, it.ID)
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
