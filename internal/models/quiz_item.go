package models

import "gorm.io/gorm"

type QuizItem struct {
	gorm.Model
	CategoryID uint `gorm:"not null"`
	Position   int  `gorm:"not null;default:0"`

	PagerID *uint `gorm:"index"`
	Pager   Pager `gorm:"foreignKey:PagerID"`

	Answer Answer `gorm:"foreignKey:QuizItemID;constraint:OnDelete:CASCADE;"`

	Video
}
