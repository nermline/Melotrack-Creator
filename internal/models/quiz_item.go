package models

import "gorm.io/gorm"

type QuizItem struct {
	gorm.Model
	CategoryID uint   `gorm:"not null"`
	Position   int    `gorm:"not null;default:0"`
	ShowVideo  bool   `gorm:"not null;default:false"`
	Answer     Answer `gorm:"foreignKey:QuizItemID;constraint:OnDelete:CASCADE;"`
	Video      Video  `gorm:"embedded"`
}
