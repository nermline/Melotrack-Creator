package models

import "time"

type Project struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Title      string     `gorm:"unique;not null" json:"title"`
	Categories []Category `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE;" json:"categories"`
}
