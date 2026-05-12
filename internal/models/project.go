package models

import "gorm.io/gorm"

type Project struct {
	gorm.Model
	Title      string     `gorm:"not null"`
	Categories []Category `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE;"`
}
