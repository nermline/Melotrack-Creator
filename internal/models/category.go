package models

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	ProjectID uint   `gorm:"not null"`
	Title     string `gorm:"not null"`
	Position  int    `gorm:"not null;default:0"`

	InitialPagerID *uint `gorm:"index"`
	InitialPager   Pager `gorm:"foreignKey:InitialPagerID"`

	DefaultPagerID *uint `gorm:"index"`
	DefaultPager   Pager `gorm:"foreignKey:DefaultPagerID"`

	Items []QuizItem `gorm:"foreignKey:CategoryID;constraint:OnDelete:CASCADE;"`
}
