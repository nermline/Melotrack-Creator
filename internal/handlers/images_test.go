package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nermline/Melotrack-Creator/internal/models"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 32), uint8(y * 32), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func multipartUpload(t *testing.T, r http.Handler, url, field, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	fw.Write(content)
	mw.Close()

	req := httptest.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUploadAnswerImage_OK(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	url := fmt.Sprintf("%s/%d/image", itemsURL(pid, cid), item.ID)
	w := multipartUpload(t, r, url, "image", "cover.png", pngBytes(t))
	mustStatus(t, w, http.StatusOK)

	var answer models.Answer
	if err := db.Where("quiz_item_id = ?", item.ID).First(&answer).Error; err != nil {
		t.Fatalf("answer not found: %v", err)
	}
	wantPath := fmt.Sprintf("/answers/%d/%d/%d.jpg", pid, cid, item.ID)
	if answer.ImagePath != wantPath {
		t.Errorf("image_path = %q, want %q", answer.ImagePath, wantPath)
	}
}

func TestUploadAnswerImage_NoFile(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	url := fmt.Sprintf("%s/%d/image", itemsURL(pid, cid), item.ID)

	req := httptest.NewRequest("POST", url, bytes.NewBufferString(""))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUploadAnswerImage_CorruptFile(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	url := fmt.Sprintf("%s/%d/image", itemsURL(pid, cid), item.ID)
	w := multipartUpload(t, r, url, "image", "bad.png", []byte("this is not an image"))
	mustStatus(t, w, http.StatusBadRequest)
}

func TestUploadAnswerImage_CategoryNotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, _ := seedProjectCategory(t, db)
	url := fmt.Sprintf("/api/projects/%d/categories/999/items/1/image", pid)
	w := multipartUpload(t, r, url, "image", "cover.png", pngBytes(t))
	mustStatus(t, w, http.StatusNotFound)
}

func TestDeleteQuizItemImage_OK(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	url := fmt.Sprintf("%s/%d/image", itemsURL(pid, cid), item.ID)
	mustStatus(t, multipartUpload(t, r, url, "image", "cover.png", pngBytes(t)), http.StatusOK)

	mustStatus(t, doJSON(r, "DELETE", url, nil), http.StatusOK)

	var answer models.Answer
	db.Where("quiz_item_id = ?", item.ID).First(&answer)
	if answer.ImagePath != "" {
		t.Errorf("image_path should be cleared, got %q", answer.ImagePath)
	}
}

func TestUploadAnswerImage_ProjectNotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	r := newTestRouter(newTestDB(t))
	w := multipartUpload(t, r, "/api/projects/999/categories/1/items/1/image", "image", "cover.png", pngBytes(t))
	mustStatus(t, w, http.StatusNotFound)
}

func TestUploadAnswerImage_WrongFieldName(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	url := fmt.Sprintf("%s/%d/image", itemsURL(pid, cid), item.ID)
	// field name "file" instead of required "image"
	w := multipartUpload(t, r, url, "file", "cover.png", pngBytes(t))
	mustStatus(t, w, http.StatusBadRequest)
}

func TestDeleteQuizItemImage_CategoryNotFound(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, _ := seedProjectCategory(t, db)
	url := fmt.Sprintf("/api/projects/%d/categories/999/items/1/image", pid)
	mustStatus(t, doJSON(r, "DELETE", url, nil), http.StatusNotFound)
}

func TestDeleteQuizItemImage_NoImageIsNoOp(t *testing.T) {
	t.Chdir(t.TempDir())
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	db.Create(&models.Media{YouTubeID: sampleYtID, Status: "ready", FilePath: "raw.mp4"})
	item := decode[models.QuizItem](t, doJSON(r, "POST", itemsURL(pid, cid), createItemBody("Song")))

	url := fmt.Sprintf("%s/%d/image", itemsURL(pid, cid), item.ID)
	mustStatus(t, doJSON(r, "DELETE", url, nil), http.StatusOK)

	var answer models.Answer
	db.Where("quiz_item_id = ?", item.ID).First(&answer)
	if answer.ImagePath != "" {
		t.Errorf("image_path should remain empty, got %q", answer.ImagePath)
	}
}
