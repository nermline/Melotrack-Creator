package models

import "gorm.io/gorm"

type Media struct {
	gorm.Model
	YouTubeID string `gorm:"uniqueIndex"`
	FilePath  string `gorm:"not null"`
	Title     string
	FileSize  int64
}
