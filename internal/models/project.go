package models

import "time"

type Project struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Title      string     `gorm:"uniqueIndex:idx_user_title;not null" json:"title"`
	UserID     uint       `gorm:"uniqueIndex:idx_user_title;not null" json:"-"`
	User       User       `gorm:"foreignKey:UserID" json:"-"`
	Categories []Category `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE;" json:"categories"`
}
