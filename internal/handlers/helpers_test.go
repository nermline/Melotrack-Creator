package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"github.com/nermline/Melotrack-Creator/ws"
	"gorm.io/gorm"
)

var (
	bgMu          sync.Mutex
	downloadCalls []uint
	renderCalls   []uint
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	triggerDownload = func(_ *gorm.DB, mediaID uint, _ string) {
		bgMu.Lock()
		downloadCalls = append(downloadCalls, mediaID)
		bgMu.Unlock()
	}
	triggerRender = func(_ *gorm.DB, _ *ws.Hub, item models.QuizItem, _, _ string) {
		bgMu.Lock()
		renderCalls = append(renderCalls, item.ID)
		bgMu.Unlock()
	}
	os.Exit(m.Run())
}

func resetBg() {
	bgMu.Lock()
	downloadCalls = nil
	renderCalls = nil
	bgMu.Unlock()
}

func downloadCount() int {
	bgMu.Lock()
	defer bgMu.Unlock()
	return len(downloadCalls)
}

func renderCount() int {
	bgMu.Lock()
	defer bgMu.Unlock()
	return len(renderCalls)
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.Role{}, &models.User{}, &models.Media{},
		&models.Project{}, &models.Category{}, &models.QuizItem{}, &models.Answer{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newTestRouter(db *gorm.DB) *gin.Engine {
	hub := ws.NewHub()
	r := gin.New()
	api := r.Group("/api")
	api.GET("/projects", GetProjects(db))
	api.GET("/projects/:pid", GetProjectByID(db))
	api.POST("/projects", CreateProject(db))
	api.PUT("/projects/:pid", UpdateProject(db))
	api.DELETE("/projects/:pid", DeleteProject(db))
	api.GET("/projects/:pid/categories", GetCategories(db))
	api.POST("/projects/:pid/categories", CreateCategory(db))
	api.PUT("/projects/:pid/categories/:cid", UpdateCategory(db))
	api.DELETE("/projects/:pid/categories/:cid", DeleteCategory(db))
	api.GET("/projects/:pid/categories/:cid/items", GetQuizItems(db))
	api.POST("/projects/:pid/categories/:cid/items", CreateQuizItem(db, hub))
	api.PUT("/projects/:pid/categories/:cid/items/:iid", UpdateQuizItem(db, hub))
	api.DELETE("/projects/:pid/categories/:cid/items/:iid", DeleteQuizItem(db, hub))
	api.POST("/projects/:pid/categories/:cid/items/:iid/render", RenderQuizItem(db, hub))
	api.POST("/projects/:pid/categories/:cid/items/:iid/redownload", RetryDownload(db, hub))
	api.POST("/projects/:pid/categories/:cid/items/:iid/image", UploadAnswerImage(db, hub))
	api.DELETE("/projects/:pid/categories/:cid/items/:iid/image", DeleteQuizItemImage(db, hub))
	return r
}

func doJSON(r *gin.Engine, method, url string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return v
}

func seedProjectCategory(t *testing.T, db *gorm.DB) (uint, uint) {
	t.Helper()
	p := models.Project{Title: "Project"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed project: %v", err)
	}
	c := models.Category{ProjectID: p.ID, Title: "Category", Position: 0}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return p.ID, c.ID
}

func mustStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, want, w.Body.String())
	}
}

var _ = http.StatusOK
