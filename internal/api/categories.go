package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/store"
	"gorm.io/gorm"
)

func (s *Server) loadCategory(c *gin.Context) (*store.Category, bool) {
	cid, ok := idParam(c, "cid")
	if !ok {
		return nil, false
	}
	var cat store.Category
	if err := s.db.First(&cat, cid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			notFound(c, "Категорію")
		} else {
			failInternal(c, err)
		}
		return nil, false
	}
	return &cat, true
}

func (s *Server) createCategory(c *gin.Context) {
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
		badRequest(c, "Вкажіть назву категорії (до 120 символів)")
		return
	}
	cat := store.Category{ProjectID: pid, Title: title}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var maxPos *int
		tx.Model(&store.Category{}).Where("project_id = ?", pid).Select("MAX(position)").Scan(&maxPos)
		if maxPos != nil {
			cat.Position = *maxPos + 1
		}
		return tx.Omit("Items").Create(&cat).Error
	})
	if err != nil {
		failInternal(c, err)
		return
	}
	s.changed(pid)
	c.JSON(http.StatusCreated, categoryDTO{ID: cat.ID, Title: cat.Title, Position: cat.Position, Items: []itemDTO{}})
}

func (s *Server) updateCategory(c *gin.Context) {
	cat, ok := s.loadCategory(c)
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
		badRequest(c, "Вкажіть назву категорії (до 120 символів)")
		return
	}
	if err := s.db.Model(cat).Update("title", title).Error; err != nil {
		failInternal(c, err)
		return
	}
	s.changed(cat.ProjectID)
	c.JSON(http.StatusOK, gin.H{"id": cat.ID, "title": title})
}

func (s *Server) deleteCategory(c *gin.Context) {
	cat, ok := s.loadCategory(c)
	if !ok {
		return
	}
	var itemIDs []uint
	s.db.Model(&store.Item{}).Where("category_id = ?", cat.ID).Pluck("id", &itemIDs)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&store.Category{}, cat.ID).Error; err != nil {
			return err
		}
		return renumber(tx, &store.Category{}, "project_id", cat.ProjectID)
	})
	if err != nil {
		failInternal(c, err)
		return
	}
	for _, id := range itemIDs {
		s.media.ItemDeleted(id)
	}
	s.changed(cat.ProjectID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) orderCategories(c *gin.Context) {
	pid, ok := s.projectID(c)
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
		return applyOrder(tx, &store.Category{}, "project_id", pid, in.IDs)
	})
	if errors.Is(err, errOrderMismatch) {
		fail(c, http.StatusConflict, "stale", "Список змінився, оновіть сторінку")
		return
	}
	if err != nil {
		failInternal(c, err)
		return
	}
	s.changed(pid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

var errOrderMismatch = errors.New("order does not match")

// applyOrder sets positions from a complete list of ids belonging to the parent.
func applyOrder(tx *gorm.DB, model any, parentCol string, parentID uint, ids []uint) error {
	var existing []uint
	if err := tx.Model(model).Where(parentCol+" = ?", parentID).Pluck("id", &existing).Error; err != nil {
		return err
	}
	if len(existing) != len(ids) {
		return errOrderMismatch
	}
	set := make(map[uint]bool, len(existing))
	for _, id := range existing {
		set[id] = true
	}
	for _, id := range ids {
		if !set[id] {
			return errOrderMismatch
		}
		delete(set, id)
	}
	for pos, id := range ids {
		if err := tx.Model(model).Where("id = ?", id).UpdateColumn("position", pos).Error; err != nil {
			return err
		}
	}
	return nil
}

// renumber closes gaps in positions after a deletion.
func renumber(tx *gorm.DB, model any, parentCol string, parentID uint) error {
	var ids []uint
	if err := tx.Model(model).Where(parentCol+" = ?", parentID).Order("position, id").Pluck("id", &ids).Error; err != nil {
		return err
	}
	for pos, id := range ids {
		if err := tx.Model(model).Where("id = ?", id).UpdateColumn("position", pos).Error; err != nil {
			return err
		}
	}
	return nil
}
