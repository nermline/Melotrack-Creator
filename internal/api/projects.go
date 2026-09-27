package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/store"
	"gorm.io/gorm"
)

type projectSummary struct {
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	Theme      string    `json:"theme"`
	CreatedAt  time.Time `json:"created_at"`
	Categories int       `json:"categories"`
	Items      int       `json:"items"`
	Teams      int       `json:"teams"`
}

func (s *Server) listProjects(c *gin.Context) {
	out := []projectSummary{}
	err := s.db.Raw(`SELECT p.id, p.title, p.theme, p.created_at,
			(SELECT COUNT(*) FROM categories c WHERE c.project_id = p.id) AS categories,
			(SELECT COUNT(*) FROM items i JOIN categories c ON c.id = i.category_id WHERE c.project_id = p.id) AS items,
			(SELECT COUNT(*) FROM teams t WHERE t.project_id = p.id) AS teams
		FROM projects p ORDER BY p.created_at DESC, p.id DESC`).Scan(&out).Error
	if err != nil {
		failInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// projectID parses :pid and checks the project exists.
func (s *Server) projectID(c *gin.Context) (uint, bool) {
	pid, ok := idParam(c, "pid")
	if !ok {
		return 0, false
	}
	var n int64
	if err := s.db.Model(&store.Project{}).Where("id = ?", pid).Count(&n).Error; err != nil {
		failInternal(c, err)
		return 0, false
	}
	if n == 0 {
		notFound(c, "Проєкт")
		return 0, false
	}
	return pid, true
}

func (s *Server) loadProject(pid uint) (*store.Project, error) {
	var p store.Project
	err := s.db.
		Preload("Categories", func(d *gorm.DB) *gorm.DB { return d.Order("position, id") }).
		Preload("Categories.Items", func(d *gorm.DB) *gorm.DB { return d.Order("position, id") }).
		Preload("Categories.Items.Media").
		Preload("Teams", func(d *gorm.DB) *gorm.DB { return d.Order("created_at, id") }).
		First(&p, pid).Error
	return &p, err
}

func (s *Server) getProject(c *gin.Context) {
	pid, ok := idParam(c, "pid")
	if !ok {
		return
	}
	p, err := s.loadProject(pid)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		notFound(c, "Проєкт")
		return
	}
	if err != nil {
		failInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, toProjectDTO(p))
}

func cleanTitle(s string, maxLen int) (string, bool) {
	s = strings.Join(strings.Fields(s), " ")
	return s, s != "" && utf8.RuneCountInString(s) <= maxLen
}

func (s *Server) createProject(c *gin.Context) {
	var in struct {
		Title string `json:"title"`
	}
	if !bindJSON(c, &in) {
		return
	}
	title, ok := cleanTitle(in.Title, 120)
	if !ok {
		badRequest(c, "Вкажіть назву (до 120 символів)")
		return
	}
	p := store.Project{Title: title, Theme: "neon", ThinkSeconds: 10, JoinCode: store.NewJoinCode()}
	if err := s.db.Create(&p).Error; err != nil {
		if store.IsUniqueViolation(err) {
			fail(c, http.StatusConflict, "duplicate", "Проєкт з такою назвою вже є")
			return
		}
		failInternal(c, err)
		return
	}
	c.JSON(http.StatusCreated, toProjectDTO(&p))
}

func (s *Server) updateProject(c *gin.Context) {
	pid, ok := s.projectID(c)
	if !ok {
		return
	}
	var in struct {
		Title            *string `json:"title"`
		Theme            *string `json:"theme"`
		ThinkSeconds     *int    `json:"think_seconds"`
		RegistrationOpen *bool   `json:"registration_open"`
	}
	if !bindJSON(c, &in) {
		return
	}
	upd := map[string]any{}
	if in.Title != nil {
		title, ok := cleanTitle(*in.Title, 120)
		if !ok {
			badRequest(c, "Вкажіть назву (до 120 символів)")
			return
		}
		upd["title"] = title
	}
	if in.Theme != nil {
		if !slices.Contains(store.Themes, *in.Theme) {
			badRequest(c, "Невідома тема оформлення")
			return
		}
		upd["theme"] = *in.Theme
	}
	if in.ThinkSeconds != nil {
		if *in.ThinkSeconds < 3 || *in.ThinkSeconds > 120 {
			badRequest(c, "Час на роздуми: від 3 до 120 секунд")
			return
		}
		upd["think_seconds"] = *in.ThinkSeconds
	}
	if in.RegistrationOpen != nil {
		upd["registration_open"] = *in.RegistrationOpen
	}
	if len(upd) > 0 {
		if err := s.db.Model(&store.Project{}).Where("id = ?", pid).Updates(upd).Error; err != nil {
			if store.IsUniqueViolation(err) {
				fail(c, http.StatusConflict, "duplicate", "Проєкт з такою назвою вже є")
				return
			}
			failInternal(c, err)
			return
		}
	}
	p, err := s.loadProject(pid)
	if err != nil {
		failInternal(c, err)
		return
	}
	s.games.Refresh(pid)
	s.changed(pid)
	c.JSON(http.StatusOK, toProjectDTO(p))
}

func (s *Server) deleteProject(c *gin.Context) {
	pid, ok := s.projectID(c)
	if !ok {
		return
	}
	var itemIDs []uint
	s.db.Raw(`SELECT i.id FROM items i JOIN categories c ON c.id = i.category_id WHERE c.project_id = ?`, pid).Scan(&itemIDs)
	if err := s.db.Delete(&store.Project{}, pid).Error; err != nil {
		failInternal(c, err)
		return
	}
	s.games.Drop(pid)
	for _, id := range itemIDs {
		s.media.ItemDeleted(id)
	}
	s.media.ScheduleGC()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// duplicateProject copies categories and songs (not teams or scores) — handy
// when the next melotrack reuses a previous one as a template. Media and
// rendered clips are shared, so the copy is ready instantly.
func (s *Server) duplicateProject(c *gin.Context) {
	pid, ok := s.projectID(c)
	if !ok {
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if !bindJSON(c, &in) {
		return
	}
	title, ok := cleanTitle(in.Title, 120)
	if !ok {
		badRequest(c, "Вкажіть назву (до 120 символів)")
		return
	}
	src, err := s.loadProject(pid)
	if err != nil {
		failInternal(c, err)
		return
	}
	dst := store.Project{Title: title, Theme: src.Theme, ThinkSeconds: src.ThinkSeconds, JoinCode: store.NewJoinCode()}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Categories", "Teams").Create(&dst).Error; err != nil {
			return err
		}
		for _, cat := range src.Categories {
			nc := store.Category{ProjectID: dst.ID, Title: cat.Title, Position: cat.Position}
			if err := tx.Omit("Items").Create(&nc).Error; err != nil {
				return err
			}
			for _, it := range cat.Items {
				ni := it
				ni.ID, ni.CategoryID, ni.CreatedAt, ni.UpdatedAt = 0, nc.ID, time.Time{}, time.Time{}
				if err := tx.Omit("Media").Create(&ni).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			fail(c, http.StatusConflict, "duplicate", "Проєкт з такою назвою вже є")
			return
		}
		failInternal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": dst.ID})
}
