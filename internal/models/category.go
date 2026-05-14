package models

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	ProjectID uint       `gorm:"uniqueIndex:idx_project_title;not null"`
	Title     string     `gorm:"uniqueIndex:idx_project_title;not null"`
	Position  int        `gorm:"not null;default:0"`
	Items     []QuizItem `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE;" json:"items,omitempty"`
}
