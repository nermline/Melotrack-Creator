package models

import "time"

type Media struct {
	ID        uint      `gorm:"primarykey" json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	YouTubeID string    `gorm:"uniqueIndex" json:"-"`
	FilePath  string    `gorm:"not null" json:"-"`
	Status    string    `gorm:"default:'downloading'" json:"status"`

	Width    int     `json:"width,omitempty"`
	Height   int     `json:"height,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}
