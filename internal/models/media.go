package models

import "time"

type Media struct {
	ID        uint      `gorm:"primarykey" json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	YouTubeID string    `gorm:"uniqueIndex" json:"-"` // Фронтенд сам знає ID з посилання
	FilePath  string    `gorm:"not null" json:"-"`
	Status    string    `gorm:"default:'downloading'" json:"status"` // Віддаємо лише статус завантаження
}
