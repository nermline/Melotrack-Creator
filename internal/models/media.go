package models

import "gorm.io/gorm"

type Media struct {
	gorm.Model
	YouTubeID string `gorm:"uniqueIndex" json:"-"`
	FilePath  string `gorm:"not null" json:"-"`
	Status    string `gorm:"default:'downloading'"` // "downloading", "ready", "error"
}
