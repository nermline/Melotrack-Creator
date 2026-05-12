package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username string `gorm:"uniqueIndex;not null"`
	Password string `gorm:"not null"` // Password hash
	RoleID   uint   `gorm:"not null"`
	Role     Role   `gorm:"foreignKey:RoleID"`
}
