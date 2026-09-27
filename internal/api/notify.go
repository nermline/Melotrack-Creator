package api

import (
	"github.com/nermline/Melotrack-Creator/internal/store"
)

// Server implements media.Notifier: background job updates are pushed to the
// editors of every project that uses the media/item.

func (s *Server) projectsOfMedia(mediaID uint) []uint {
	var ids []uint
	s.db.Raw(`SELECT DISTINCT c.project_id FROM items i JOIN categories c ON c.id = i.category_id
		WHERE i.media_id = ?`, mediaID).Scan(&ids)
	return ids
}

func (s *Server) projectOfItem(itemID uint) (uint, bool) {
	var pid uint
	err := s.db.Raw(`SELECT c.project_id FROM items i JOIN categories c ON c.id = i.category_id
		WHERE i.id = ?`, itemID).Scan(&pid).Error
	return pid, err == nil && pid != 0
}

func (s *Server) MediaChanged(mediaID uint) {
	var m store.Media
	if s.db.First(&m, mediaID).Error != nil {
		return
	}
	dto := toMediaDTO(&m)
	for _, pid := range s.projectsOfMedia(mediaID) {
		s.hub.Publish(pid, "media", dto)
	}
}

func (s *Server) MediaProgress(mediaID uint, progress float64) {
	for _, pid := range s.projectsOfMedia(mediaID) {
		s.hub.Publish(pid, "media_progress", map[string]any{"id": mediaID, "progress": progress})
	}
}

func (s *Server) ItemChanged(itemID uint) {
	var it store.Item
	if s.db.Preload("Media").First(&it, itemID).Error != nil {
		return
	}
	if pid, ok := s.projectOfItem(itemID); ok {
		s.hub.Publish(pid, "item", toItemDTO(&it))
	}
}

func (s *Server) ItemProgress(itemID uint, progress float64) {
	if pid, ok := s.projectOfItem(itemID); ok {
		s.hub.Publish(pid, "item_progress", map[string]any{"id": itemID, "progress": progress})
	}
}

// changed tells editors to refetch the project after a structural change.
func (s *Server) changed(projectID uint) {
	s.hub.Publish(projectID, "project", map[string]any{"id": projectID})
}
