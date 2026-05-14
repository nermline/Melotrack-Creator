package models

import "gorm.io/gorm"

type Project struct {
	gorm.Model
	Title      string     `gorm:"uniqueIndex:idx_user_title;not null"`
	UserID     uint       `gorm:"uniqueIndex:idx_user_title;not null" json:"-"`
	User       User       `gorm:"foreignKey:UserID" json:"-"`
	Categories []Category `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE;"`
}
