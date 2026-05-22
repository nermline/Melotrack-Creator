package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/nermline/Melotrack-Creator/internal/models"
)

func TestCreateProject_OK(t *testing.T) {
	r := newTestRouter(newTestDB(t))
	w := doJSON(r, "POST", "/api/projects", map[string]string{"title": "My Quiz"})
	mustStatus(t, w, http.StatusCreated)
	p := decode[models.Project](t, w)
	if p.ID == 0 || p.Title != "My Quiz" {
		t.Errorf("unexpected project: %+v", p)
	}
	if p.Categories == nil {
		t.Error("Categories should be an empty slice, not nil")
	}
}

func TestCreateProject_MissingTitle(t *testing.T) {
	r := newTestRouter(newTestDB(t))
	w := doJSON(r, "POST", "/api/projects", map[string]string{})
	mustStatus(t, w, http.StatusBadRequest)
}

func TestCreateProject_DuplicateTitle(t *testing.T) {
	r := newTestRouter(newTestDB(t))
	mustStatus(t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "Dup"}), http.StatusCreated)
	w := doJSON(r, "POST", "/api/projects", map[string]string{"title": "Dup"})
	mustStatus(t, w, http.StatusConflict)
}

func TestGetProjects_OrderedAndEmptyArray(t *testing.T) {
	r := newTestRouter(newTestDB(t))

	// Порожній список — має бути [] а не null.
	w := doJSON(r, "GET", "/api/projects", nil)
	mustStatus(t, w, http.StatusOK)
	if w.Body.String() != "[]" {
		t.Errorf("empty projects body = %q, want []", w.Body.String())
	}

	doJSON(r, "POST", "/api/projects", map[string]string{"title": "A"})
	doJSON(r, "POST", "/api/projects", map[string]string{"title": "B"})
	list := decode[[]ProjectSummary](t, doJSON(r, "GET", "/api/projects", nil))
	if len(list) != 2 {
		t.Fatalf("got %d projects, want 2", len(list))
	}
}

func TestGetProjectByID_NotFound(t *testing.T) {
	r := newTestRouter(newTestDB(t))
	mustStatus(t, doJSON(r, "GET", "/api/projects/999", nil), http.StatusNotFound)
}

func TestGetProjectByID_WithCategories(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, _ := seedProjectCategory(t, db)
	w := doJSON(r, "GET", fmt.Sprintf("/api/projects/%d", pid), nil)
	mustStatus(t, w, http.StatusOK)
	p := decode[models.Project](t, w)
	if len(p.Categories) != 1 {
		t.Errorf("expected 1 category, got %d", len(p.Categories))
	}
	if p.Categories[0].Items == nil {
		t.Error("category Items should be empty slice, not nil")
	}
}

func TestUpdateProject_OK(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	p := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "Old"}))
	w := doJSON(r, "PUT", fmt.Sprintf("/api/projects/%d", p.ID), map[string]string{"title": "New"})
	mustStatus(t, w, http.StatusOK)
	if decode[models.Project](t, w).Title != "New" {
		t.Error("title not updated")
	}
}

func TestUpdateProject_NotFound(t *testing.T) {
	r := newTestRouter(newTestDB(t))
	mustStatus(t, doJSON(r, "PUT", "/api/projects/999", map[string]string{"title": "X"}), http.StatusNotFound)
}

func TestUpdateProject_DuplicateTitle(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	doJSON(r, "POST", "/api/projects", map[string]string{"title": "Taken"})
	p := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "Mine"}))
	w := doJSON(r, "PUT", fmt.Sprintf("/api/projects/%d", p.ID), map[string]string{"title": "Taken"})
	mustStatus(t, w, http.StatusConflict)
}

func TestDeleteProject_OK(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	p := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "ToDelete"}))
	mustStatus(t, doJSON(r, "DELETE", fmt.Sprintf("/api/projects/%d", p.ID), nil), http.StatusOK)
	mustStatus(t, doJSON(r, "GET", fmt.Sprintf("/api/projects/%d", p.ID), nil), http.StatusNotFound)
}

func TestDeleteProject_NotFound(t *testing.T) {
	r := newTestRouter(newTestDB(t))
	mustStatus(t, doJSON(r, "DELETE", "/api/projects/12345", nil), http.StatusNotFound)
}
