package handlers

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/nermline/Melotrack-Creator/internal/models"
)

const sampleURL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
const sampleYtID = "dQw4w9WgXcQ"

func createItemBody(title string) CreateQuizItemInput {
	return CreateQuizItemInput{
		ShowVideo: false,
		Video:     VideoInput{YoutubeURL: sampleURL, StartTime: 0, EndTime: 15, Volume: 1},
		Answer:    AnswerInput{Title: title},
	}
}

func itemsURL(pid, cid uint) string {
	return fmt.Sprintf("/api/projects/%d/categories/%d/items", pid, cid)
}

func TestCreateQuizItem_NewMediaTriggersDownload(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)

	w := doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song"))
	mustStatus(t, w, http.StatusCreated)
	item := decode[models.QuizItem](t, w)
	if item.ID == 0 || item.Answer.Title != "Song" {
		t.Errorf("bad item: %+v", item)
	}
	if downloadCount() != 1 {
		t.Errorf("expected 1 download trigger, got %d", downloadCount())
	}

	var media models.Media
	if err := db.Where("you_tube_id = ?", sampleYtID).First(&media).Error; err != nil {
		t.Fatalf("media not created: %v", err)
	}
	if media.Status != "downloading" {
		t.Errorf("media status = %q, want downloading", media.Status)
	}
}

func TestCreateQuizItem_ReusesReadyMediaNoDownload(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)

	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 100})

	mustStatus(t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")), http.StatusCreated)
	if downloadCount() != 0 {
		t.Errorf("expected no download for ready media, got %d", downloadCount())
	}
}

func TestCreateQuizItem_RetriesErroredMedia(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)

	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "error", FilePath: ""})

	mustStatus(t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")), http.StatusCreated)
	if downloadCount() != 1 {
		t.Errorf("errored media should re-trigger download, got %d", downloadCount())
	}
	var media models.Media
	db.Where("you_tube_id = ?", sampleYtID).First(&media)
	if media.Status != "downloading" {
		t.Errorf("errored media status = %q, want downloading", media.Status)
	}
}

func TestCreateQuizItem_InvalidURL(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	body := CreateQuizItemInput{Video: VideoInput{YoutubeURL: "not a youtube link"}, Answer: AnswerInput{Title: "X"}}
	mustStatus(t, doJSON(r, "POST", itemsURL(pid, cid), body), http.StatusBadRequest)
}

func TestCreateQuizItem_CategoryNotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, _ := seedProjectCategory(t, db)
	mustStatus(t, doJSON(r, "POST", itemsURL(pid, 999), createItemBody("X")), http.StatusNotFound)
}

func TestGetQuizItems_EmptyArray(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	w := doJSON(r, "GET", itemsURL(pid, cid), nil)
	mustStatus(t, w, http.StatusOK)
	if w.Body.String() != "[]" {
		t.Errorf("body = %q, want []", w.Body.String())
	}
}

func TestUpdateQuizItem_ParamsResetRenderStatus(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 100})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	db.Model(&models.QuizItem{}).Where("id = ?", item.ID).Update("render_status", "ready")

	body := map[string]any{"video": map[string]any{"start_time": 5.0, "end_time": 20.0}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
	got := decode[models.QuizItem](t, w)
	if got.Video.StartTime != 5 || got.Video.EndTime != 20 {
		t.Errorf("times not updated: %+v", got.Video)
	}
	if got.Video.RenderStatus != "unrendered" {
		t.Errorf("changing params should reset render_status to unrendered, got %q", got.Video.RenderStatus)
	}
}

func TestUpdateQuizItem_CropValidation(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 100, Height: 100, Duration: 30})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	body := map[string]any{"video": map[string]any{"crop_x": 80, "crop_y": 0, "crop_width": 50, "crop_height": 50}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_StartBeyondDuration(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 100, Height: 100, Duration: 30})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	body := map[string]any{"video": map[string]any{"start_time": 40.0}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_Reorder(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	i0 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I0")))
	i1 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I1")))
	i2 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I2")))

	body := map[string]any{"position": 0}
	mustStatus(t, doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), i2.ID), body), http.StatusOK)

	items := decode[[]models.QuizItem](t, doJSON(r, "GET", itemsURL(pid, cid), nil))
	pos := map[uint]int{}
	for _, it := range items {
		pos[it.ID] = it.Position
	}
	if pos[i2.ID] != 0 || pos[i0.ID] != 1 || pos[i1.ID] != 2 {
		t.Errorf("positions = i0:%d i1:%d i2:%d, want 1,2,0", pos[i0.ID], pos[i1.ID], pos[i2.ID])
	}
}

func TestUpdateQuizItem_NotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	w := doJSON(r, "PUT", fmt.Sprintf("%s/999", itemsURL(pid, cid)), map[string]any{"show_video": true})
	mustStatus(t, w, http.StatusNotFound)
}

func TestRenderQuizItem_MediaNotReady(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "downloading", FilePath: ""})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	w := doJSON(r, "POST", fmt.Sprintf("%s/%d/render", itemsURL(pid, cid), item.ID), nil)
	mustStatus(t, w, http.StatusConflict)
	if renderCount() != 0 {
		t.Error("render should not start when media not ready")
	}
}

func TestRenderQuizItem_Success(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 100})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	w := doJSON(r, "POST", fmt.Sprintf("%s/%d/render", itemsURL(pid, cid), item.ID), nil)
	mustStatus(t, w, http.StatusOK)
	if renderCount() != 1 {
		t.Errorf("expected 1 render trigger, got %d", renderCount())
	}
	var got models.QuizItem
	db.Where("id = ?", item.ID).First(&got)
	if got.Video.RenderStatus != "rendering" {
		t.Errorf("render_status = %q, want rendering", got.Video.RenderStatus)
	}
}

func TestRenderQuizItem_ItemNotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	w := doJSON(r, "POST", fmt.Sprintf("%s/999/render", itemsURL(pid, cid)), nil)
	mustStatus(t, w, http.StatusNotFound)
}

func TestRetryDownload_ErroredMedia(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "error", FilePath: ""})

	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))
	resetBg()

	w := doJSON(r, "POST", fmt.Sprintf("%s/%d/redownload", itemsURL(pid, cid), item.ID), nil)
	mustStatus(t, w, http.StatusOK)
	if downloadCount() != 1 {
		t.Errorf("retry should trigger download, got %d", downloadCount())
	}
	var media models.Media
	db.Where("you_tube_id = ?", sampleYtID).First(&media)
	if media.Status != "downloading" {
		t.Errorf("media status after retry = %q, want downloading", media.Status)
	}
}

func TestRetryDownload_AlreadyInProgress(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	var got models.QuizItem
	db.Preload("Video.Media").Where("id = ?", item.ID).First(&got)
	mID := *got.Video.MediaID

	activeDownloadWorkers.Store(mID, context.CancelFunc(func() {}))
	defer activeDownloadWorkers.Delete(mID)

	w := doJSON(r, "POST", fmt.Sprintf("%s/%d/redownload", itemsURL(pid, cid), item.ID), nil)
	mustStatus(t, w, http.StatusConflict)
}

func TestRetryDownload_ItemNotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	w := doJSON(r, "POST", fmt.Sprintf("%s/999/redownload", itemsURL(pid, cid)), nil)
	mustStatus(t, w, http.StatusNotFound)
}

func TestDeleteQuizItem_ShiftsPositions(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	i0 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I0")))
	i1 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I1")))

	mustStatus(t, doJSON(r, "DELETE", fmt.Sprintf("%s/%d", itemsURL(pid, cid), i0.ID), nil), http.StatusOK)

	items := decode[[]models.QuizItem](t, doJSON(r, "GET", itemsURL(pid, cid), nil))
	if len(items) != 1 || items[0].ID != i1.ID || items[0].Position != 0 {
		t.Errorf("after delete: %+v (i1 should remain at position 0)", items)
	}
}

func TestDeleteQuizItem_NotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	mustStatus(t, doJSON(r, "DELETE", fmt.Sprintf("%s/999", itemsURL(pid, cid)), nil), http.StatusNotFound)
}

// ── direct unit tests for validateVideoCropParams ──────────────────────────

func TestValidateVideoCropParams_SkipsWhenDimensionsUnknown(t *testing.T) {
	v := models.Video{StartTime: -99, EndTime: 5, CropX: -1, CropWidth: 100, CropHeight: 100}
	m := models.Media{Width: 0, Height: 0, Duration: 0}
	if err := validateVideoCropParams(v, m); err != nil {
		t.Errorf("expected nil when media dimensions unknown, got %v", err)
	}
}

func TestValidateVideoCropParams_NegativeStartTime(t *testing.T) {
	v := models.Video{StartTime: -1, EndTime: 10}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error for negative start_time")
	}
}

func TestValidateVideoCropParams_StartTimeEqualsDuration(t *testing.T) {
	v := models.Video{StartTime: 60}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when start_time >= duration")
	}
}

func TestValidateVideoCropParams_StartTimeBeyondDuration(t *testing.T) {
	v := models.Video{StartTime: 100}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when start_time > duration")
	}
}

func TestValidateVideoCropParams_EndTimeBeforeStart(t *testing.T) {
	v := models.Video{StartTime: 10, EndTime: 5}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when end_time < start_time")
	}
}

func TestValidateVideoCropParams_EndTimeEqualsStartTime(t *testing.T) {
	v := models.Video{StartTime: 10, EndTime: 10}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when end_time == start_time")
	}
}

func TestValidateVideoCropParams_EndTimeBeyondDuration(t *testing.T) {
	v := models.Video{StartTime: 0, EndTime: 100}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when end_time > duration")
	}
}

func TestValidateVideoCropParams_NegativeCropX(t *testing.T) {
	v := models.Video{CropX: -1, CropY: 0, CropWidth: 100, CropHeight: 100}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error for negative crop_x")
	}
}

func TestValidateVideoCropParams_NegativeCropY(t *testing.T) {
	v := models.Video{CropX: 0, CropY: -1, CropWidth: 100, CropHeight: 100}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error for negative crop_y")
	}
}

func TestValidateVideoCropParams_CropWidthZeroWithHeightSet(t *testing.T) {
	v := models.Video{CropX: 0, CropY: 0, CropWidth: 0, CropHeight: 100}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when crop_height>0 but crop_width=0")
	}
}

func TestValidateVideoCropParams_CropHeightZeroWithWidthSet(t *testing.T) {
	v := models.Video{CropX: 0, CropY: 0, CropWidth: 100, CropHeight: 0}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when crop_width>0 but crop_height=0")
	}
}

func TestValidateVideoCropParams_CropExceedsRightEdge(t *testing.T) {
	v := models.Video{CropX: 1900, CropY: 0, CropWidth: 100, CropHeight: 100}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	// 1900+100=2000 > 1920
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when crop exceeds right edge")
	}
}

func TestValidateVideoCropParams_CropExceedsBottomEdge(t *testing.T) {
	v := models.Video{CropX: 0, CropY: 1000, CropWidth: 100, CropHeight: 100}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	// 1000+100=1100 > 1080
	if err := validateVideoCropParams(v, m); err == nil {
		t.Error("expected error when crop exceeds bottom edge")
	}
}

func TestValidateVideoCropParams_CropAtExactRightEdge(t *testing.T) {
	// x(1820)+w(100) == width(1920) → valid (condition is >, not >=)
	v := models.Video{CropX: 1820, CropY: 0, CropWidth: 100, CropHeight: 100}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err != nil {
		t.Errorf("expected nil for crop at exact right edge, got %v", err)
	}
}

func TestValidateVideoCropParams_FitSkipsCropCheck(t *testing.T) {
	v := models.Video{Fit: true, CropX: -999, CropY: -999, CropWidth: 99999, CropHeight: 99999}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err != nil {
		t.Errorf("fit mode should skip crop validation, got %v", err)
	}
}

func TestValidateVideoCropParams_ZeroCropDimensions_Valid(t *testing.T) {
	v := models.Video{CropX: 0, CropY: 0, CropWidth: 0, CropHeight: 0}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err != nil {
		t.Errorf("zero crop (no crop applied) should be valid, got %v", err)
	}
}

func TestValidateVideoCropParams_Valid(t *testing.T) {
	v := models.Video{StartTime: 5, EndTime: 30, CropX: 100, CropY: 100, CropWidth: 200, CropHeight: 200}
	m := models.Media{Width: 1920, Height: 1080, Duration: 60}
	if err := validateVideoCropParams(v, m); err != nil {
		t.Errorf("expected nil for fully valid params, got %v", err)
	}
}

// ── additional HTTP handler tests ──────────────────────────────────────────

func TestCreateQuizItem_ProjectNotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	w := doJSON(r, "POST", "/api/projects/999/categories/1/items", createItemBody("Song"))
	mustStatus(t, w, http.StatusNotFound)
}

func TestCreateQuizItem_MissingBody(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	mustStatus(t, doJSON(r, "POST", itemsURL(pid, cid), nil), http.StatusBadRequest)
}

func TestGetQuizItems_ProjectNotFound(t *testing.T) {
	r := newTestRouter(newTestDB(t))
	mustStatus(t, doJSON(r, "GET", "/api/projects/999/categories/1/items", nil), http.StatusNotFound)
}

func TestGetQuizItems_CategoryNotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, _ := seedProjectCategory(t, db)
	mustStatus(t, doJSON(r, "GET", fmt.Sprintf("/api/projects/%d/categories/999/items", pid), nil), http.StatusNotFound)
}

func TestGetQuizItems_OrderedByPosition(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	i0 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("First")))
	i1 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Second")))
	i2 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Third")))

	doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), i2.ID), map[string]any{"position": 0})

	items := decode[[]models.QuizItem](t, doJSON(r, "GET", itemsURL(pid, cid), nil))
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].ID != i2.ID || items[1].ID != i0.ID || items[2].ID != i1.ID {
		t.Errorf("order wrong: got IDs %d,%d,%d, want %d,%d,%d",
			items[0].ID, items[1].ID, items[2].ID, i2.ID, i0.ID, i1.ID)
	}
}

func TestUpdateQuizItem_InvalidYouTubeURL(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	body := map[string]any{"video": map[string]any{"youtube_url": "not-a-youtube-url"}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_NegativeStartTime(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 60})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	body := map[string]any{"video": map[string]any{"start_time": -5.0}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_EndTimeEqualsStartTime(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 60})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	body := map[string]any{"video": map[string]any{"start_time": 10.0, "end_time": 10.0}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_EndTimeExceedsDuration(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 30})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	body := map[string]any{"video": map[string]any{"end_time": 50.0}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_NegativeCropX(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 60})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	body := map[string]any{"video": map[string]any{"crop_x": -1, "crop_y": 0, "crop_width": 100, "crop_height": 100}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_NegativeCropY(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 60})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	body := map[string]any{"video": map[string]any{"crop_x": 0, "crop_y": -1, "crop_width": 100, "crop_height": 100}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_CropExceedsBottomEdge(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 100, Height: 100, Duration: 30})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	// y(80)+h(50)=130 > height(100)
	body := map[string]any{"video": map[string]any{"crop_x": 0, "crop_y": 80, "crop_width": 50, "crop_height": 50}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUpdateQuizItem_CropAtExactRightEdge(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 100, Height: 100, Duration: 30})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	// x(50)+w(50)==width(100) → exactly on edge, valid
	body := map[string]any{"video": map[string]any{"crop_x": 50, "crop_y": 0, "crop_width": 50, "crop_height": 50}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
}

func TestUpdateQuizItem_FitModeSkipsCropValidation(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 100, Height: 100, Duration: 30})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	// fit=true → out-of-bounds crop values are ignored
	body := map[string]any{"video": map[string]any{"fit": true, "crop_x": 9999, "crop_y": 9999, "crop_width": 9999, "crop_height": 9999}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
	if got := decode[models.QuizItem](t, w); !got.Video.Fit {
		t.Error("fit should be true")
	}
}

func TestUpdateQuizItem_ShowVideoOnlyDoesNotResetRenderStatus(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))
	db.Model(&models.QuizItem{}).Where("id = ?", item.ID).Update("render_status", "ready")

	body := map[string]any{"show_video": true}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
	got := decode[models.QuizItem](t, w)
	if got.Video.RenderStatus != "ready" {
		t.Errorf("show_video-only change should not reset render_status, got %q", got.Video.RenderStatus)
	}
	if !got.ShowVideo {
		t.Error("show_video should be true after update")
	}
}

func TestUpdateQuizItem_EmptyBodyIsNoOp(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Original")))

	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), map[string]any{})
	mustStatus(t, w, http.StatusOK)
	if got := decode[models.QuizItem](t, w); got.Answer.Title != "Original" {
		t.Errorf("answer title changed unexpectedly: %q", got.Answer.Title)
	}
}

func TestUpdateQuizItem_AnswerTitleChange(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Old")))

	body := map[string]any{"answer": map[string]any{"title": "New"}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
	got := decode[models.QuizItem](t, w)
	if got.Answer.Title != "New" {
		t.Errorf("answer title = %q, want New", got.Answer.Title)
	}
	if got.Video.RenderStatus != "unrendered" {
		t.Errorf("answer-only update must not reset render_status, got %q", got.Video.RenderStatus)
	}
}

func TestUpdateQuizItem_PositionClampedAboveMax(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	i0 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I0")))
	doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I1"))

	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), i0.ID), map[string]any{"position": 999})
	mustStatus(t, w, http.StatusOK)
	if got := decode[models.QuizItem](t, w); got.Position != 1 {
		t.Errorf("position should clamp to 1, got %d", got.Position)
	}
}

func TestUpdateQuizItem_PositionClampedBelowZero(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I0"))
	i1 := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I1")))

	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), i1.ID), map[string]any{"position": -10})
	mustStatus(t, w, http.StatusOK)
	if got := decode[models.QuizItem](t, w); got.Position != 0 {
		t.Errorf("position should clamp to 0, got %d", got.Position)
	}
}

func TestUpdateQuizItem_SamePositionIsNoOp(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("I0")))

	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), map[string]any{"position": 0})
	mustStatus(t, w, http.StatusOK)
	if got := decode[models.QuizItem](t, w); got.Position != 0 {
		t.Errorf("position should stay 0, got %d", got.Position)
	}
}

func TestUpdateQuizItem_URLChangedToNewMediaTriggersDownload(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))
	resetBg()

	body := map[string]any{"video": map[string]any{"youtube_url": "https://www.youtube.com/watch?v=jNQXAC9IVRw"}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
	if downloadCount() != 1 {
		t.Errorf("new URL should trigger download, got %d", downloadCount())
	}
}

func TestUpdateQuizItem_URLChangedToExistingReadyMediaNoDownload(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	const secondID = "jNQXAC9IVRw"
	db.Create(&models.Media{YouTubeID: secondID, Status: "ready", FilePath: "raw2.mp4"})
	resetBg()

	body := map[string]any{"video": map[string]any{"youtube_url": "https://www.youtube.com/watch?v=" + secondID}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
	if downloadCount() != 0 {
		t.Errorf("existing ready media should not trigger download, got %d", downloadCount())
	}
}

func TestUpdateQuizItem_URLChangedToErroredMediaRetriggers(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	const erroredID = "jNQXAC9IVRw"
	db.Create(&models.Media{YouTubeID: erroredID, Status: "error", FilePath: ""})
	resetBg()

	body := map[string]any{"video": map[string]any{"youtube_url": "https://www.youtube.com/watch?v=" + erroredID}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
	if downloadCount() != 1 {
		t.Errorf("errored media on URL change should re-trigger download, got %d", downloadCount())
	}
}

func TestUpdateQuizItem_URLChangedResetsRenderStatus(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))
	db.Model(&models.QuizItem{}).Where("id = ?", item.ID).Update("render_status", "ready")
	resetBg()

	body := map[string]any{"video": map[string]any{"youtube_url": "https://www.youtube.com/watch?v=jNQXAC9IVRw"}}
	w := doJSON(r, "PUT", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), body)
	mustStatus(t, w, http.StatusOK)
	if got := decode[models.QuizItem](t, w); got.Video.RenderStatus != "unrendered" {
		t.Errorf("URL change should reset render_status to unrendered, got %q", got.Video.RenderStatus)
	}
}

func TestRenderQuizItem_CropValidationFails(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 100, Height: 100, Duration: 30})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	// inject out-of-bounds crop directly in DB
	db.Model(&models.QuizItem{}).Where("id = ?", item.ID).Updates(map[string]any{
		"crop_x": 80, "crop_y": 0, "crop_width": 50, "crop_height": 50,
	})

	w := doJSON(r, "POST", fmt.Sprintf("%s/%d/render", itemsURL(pid, cid), item.ID), nil)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestRenderQuizItem_DoubleRenderCancelsFirst(t *testing.T) {
	resetBg()
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4", Width: 1920, Height: 1080, Duration: 100})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	mustStatus(t, doJSON(r, "POST", fmt.Sprintf("%s/%d/render", itemsURL(pid, cid), item.ID), nil), http.StatusOK)
	mustStatus(t, doJSON(r, "POST", fmt.Sprintf("%s/%d/render", itemsURL(pid, cid), item.ID), nil), http.StatusOK)
	if renderCount() != 2 {
		t.Errorf("expected 2 render triggers (second cancels first), got %d", renderCount())
	}
}

func TestRetryDownload_NoMediaAssociated(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	db.Model(&models.QuizItem{}).Where("id = ?", item.ID).Update("media_id", nil)

	w := doJSON(r, "POST", fmt.Sprintf("%s/%d/redownload", itemsURL(pid, cid), item.ID), nil)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestDeleteQuizItem_CancelsActiveRender(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	cancelled := false
	activeRenderWorkers.Store(item.ID, context.CancelFunc(func() { cancelled = true }))

	mustStatus(t, doJSON(r, "DELETE", fmt.Sprintf("%s/%d", itemsURL(pid, cid), item.ID), nil), http.StatusOK)

	if !cancelled {
		t.Error("active render worker should be cancelled on item delete")
	}
	if _, exists := activeRenderWorkers.Load(item.ID); exists {
		t.Error("render worker entry should be removed after delete")
	}
}
