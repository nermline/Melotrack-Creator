package models

import "time"

// Project — спільний для всіх користувачів (self-hosted). Власника немає:
// admin/editor керують вмістом, operator лише переглядає й керує показом.
type Project struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Title      string     `gorm:"unique;not null" json:"title"`
	Categories []Category `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE;" json:"categories"`
}
