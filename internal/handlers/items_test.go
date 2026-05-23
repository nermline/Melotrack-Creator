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
