package models

import "time"

type Category struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ProjectID uint       `gorm:"uniqueIndex:idx_project_title;not null" json:"project_id"`
	Title     string     `gorm:"uniqueIndex:idx_project_title;not null" json:"title"`
	Position  int        `gorm:"not null;default:0" json:"position"`
	Items     []QuizItem `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE;" json:"items"`
}
