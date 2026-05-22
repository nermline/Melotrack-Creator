package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/nermline/Melotrack-Creator/internal/models"
)

func TestCreateCategory_AutoPosition(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	p := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "P"}))

	c0 := decode[models.Category](t, doJSON(r, "POST", fmt.Sprintf("/api/projects/%d/categories", p.ID), map[string]string{"title": "C0"}))
	c1 := decode[models.Category](t, doJSON(r, "POST", fmt.Sprintf("/api/projects/%d/categories", p.ID), map[string]string{"title": "C1"}))
	if c0.Position != 0 || c1.Position != 1 {
		t.Errorf("positions = %d,%d want 0,1", c0.Position, c1.Position)
	}
	if c0.Items == nil {
		t.Error("Items should be empty slice not nil")
	}
}

func TestCreateCategory_MissingProject(t *testing.T) {
	r := newTestRouter(newTestDB(t))
	w := doJSON(r, "POST", "/api/projects/999/categories", map[string]string{"title": "X"})
	mustStatus(t, w, http.StatusNotFound)
}

func TestCreateCategory_MissingTitle(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, _ := seedProjectCategory(t, db)
	w := doJSON(r, "POST", fmt.Sprintf("/api/projects/%d/categories", pid), map[string]string{})
	mustStatus(t, w, http.StatusBadRequest)
}

func TestCreateCategory_DuplicateWithinProject(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	p := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "P"}))
	doJSON(r, "POST", fmt.Sprintf("/api/projects/%d/categories", p.ID), map[string]string{"title": "Same"})
	w := doJSON(r, "POST", fmt.Sprintf("/api/projects/%d/categories", p.ID), map[string]string{"title": "Same"})
	mustStatus(t, w, http.StatusConflict)
}

func TestCreateCategory_SameTitleDifferentProjects(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	p1 := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "P1"}))
	p2 := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "P2"}))
	mustStatus(t, doJSON(r, "POST", fmt.Sprintf("/api/projects/%d/categories", p1.ID), map[string]string{"title": "Shared"}), http.StatusCreated)
	// Та сама назва в іншому проєкті — дозволено.
	mustStatus(t, doJSON(r, "POST", fmt.Sprintf("/api/projects/%d/categories", p2.ID), map[string]string{"title": "Shared"}), http.StatusCreated)
}

func TestGetCategories_EmptyArray(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	p := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "P"}))
	w := doJSON(r, "GET", fmt.Sprintf("/api/projects/%d/categories", p.ID), nil)
	mustStatus(t, w, http.StatusOK)
	if w.Body.String() != "[]" {
		t.Errorf("body = %q, want []", w.Body.String())
	}
}

func TestUpdateCategory_Title(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, cid := seedProjectCategory(t, db)
	title := "Renamed"
	w := doJSON(r, "PUT", fmt.Sprintf("/api/projects/%d/categories/%d", pid, cid), map[string]any{"title": title})
	mustStatus(t, w, http.StatusOK)
	if decode[models.Category](t, w).Title != title {
		t.Error("title not updated")
	}
}

func TestUpdateCategory_NotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, _ := seedProjectCategory(t, db)
	w := doJSON(r, "PUT", fmt.Sprintf("/api/projects/%d/categories/999", pid), map[string]any{"title": "X"})
	mustStatus(t, w, http.StatusNotFound)
}

func TestUpdateCategory_Reorder(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	p := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "P"}))
	base := fmt.Sprintf("/api/projects/%d/categories", p.ID)
	c0 := decode[models.Category](t, doJSON(r, "POST", base, map[string]string{"title": "C0"}))
	c1 := decode[models.Category](t, doJSON(r, "POST", base, map[string]string{"title": "C1"}))
	c2 := decode[models.Category](t, doJSON(r, "POST", base, map[string]string{"title": "C2"}))

	// Переміщуємо c2 (поз.2) на початок (поз.0).
	pos := 0
	mustStatus(t, doJSON(r, "PUT", fmt.Sprintf("%s/%d", base, c2.ID), map[string]any{"position": pos}), http.StatusOK)

	cats := decode[[]models.Category](t, doJSON(r, "GET", base, nil))
	got := map[uint]int{}
	for _, c := range cats {
		got[c.ID] = c.Position
	}
	if got[c2.ID] != 0 || got[c0.ID] != 1 || got[c1.ID] != 2 {
		t.Errorf("after reorder positions = c0:%d c1:%d c2:%d, want 1,2,0", got[c0.ID], got[c1.ID], got[c2.ID])
	}
}

func TestDeleteCategory_ShiftsPositions(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	p := decode[models.Project](t, doJSON(r, "POST", "/api/projects", map[string]string{"title": "P"}))
	base := fmt.Sprintf("/api/projects/%d/categories", p.ID)
	c0 := decode[models.Category](t, doJSON(r, "POST", base, map[string]string{"title": "C0"}))
	c1 := decode[models.Category](t, doJSON(r, "POST", base, map[string]string{"title": "C1"}))

	mustStatus(t, doJSON(r, "DELETE", fmt.Sprintf("%s/%d", base, c0.ID), nil), http.StatusOK)

	cats := decode[[]models.Category](t, doJSON(r, "GET", base, nil))
	if len(cats) != 1 || cats[0].ID != c1.ID || cats[0].Position != 0 {
		t.Errorf("after delete: %+v (c1 should remain at position 0)", cats)
	}
}

func TestDeleteCategory_NotFound(t *testing.T) {
	db := newTestDB(t)
	r := newTestRouter(db)
	pid, _ := seedProjectCategory(t, db)
	mustStatus(t, doJSON(r, "DELETE", fmt.Sprintf("/api/projects/%d/categories/999", pid), nil), http.StatusNotFound)
}
