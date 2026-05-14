package models

import "gorm.io/gorm"

type Answer struct {
	gorm.Model
	QuizItemID      uint `gorm:"not null"`
	Title           string
	ImagePath       string
	ImageCropX      int
	ImageCropY      int
	ImageCropWidth  int
	ImageCropHeight int
}
