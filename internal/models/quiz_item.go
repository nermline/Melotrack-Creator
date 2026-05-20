package models

import "time"

type QuizItem struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	CategoryID uint      `gorm:"not null" json:"category_id"`
	Position   int       `gorm:"not null;default:0" json:"position"`
	ShowVideo  bool      `gorm:"not null;default:false" json:"show_video"`
	Answer     Answer    `gorm:"foreignKey:QuizItemID;constraint:OnDelete:CASCADE;" json:"answer"`
	Video      Video     `gorm:"embedded" json:"video"`
}
