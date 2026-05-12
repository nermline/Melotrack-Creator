package models

import "gorm.io/gorm"

type Pager struct {
	gorm.Model
	Name      string `gorm:"not null"`
	LocalPath string

	Video
}
