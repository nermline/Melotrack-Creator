package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var defaulAdminPassword string = "admin"
var defauldAdminUsername string = "admin"

func InitDB(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("initDB(): gorm.Open() Failed to open DB: %v", err)
	}

	err = db.AutoMigrate(&models.Role{}, &models.User{})
	if err != nil {
		return nil, fmt.Errorf("initDB(): db.AutoMigrate() Failed to migrate: %v", err)
	}

	seedRoles(db)

	if err := seedAdmin(db); err != nil {
		return nil, fmt.Errorf("InitDB(): %v", err)
	}

	return db, nil
}

func seedRoles(db *gorm.DB) {
	roles := []string{"admin", "editor", "operator"}
	for _, name := range roles {
		db.FirstOrCreate(&models.Role{}, models.Role{Name: name})
	}
}

func seedAdmin(db *gorm.DB) error {
	var adminRole models.Role

	if err := db.Where("name = ?", defauldAdminUsername).First(&adminRole).Error; err != nil {
		return fmt.Errorf("seedAdmin(): db.Where(): Failed to find admin role in DB: %v", err)
	}

	var count int64
	db.Model(&models.User{}).Where("username = ?", defauldAdminUsername).Count(&count)

	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(defaulAdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("seedAdmin(): bcrypt.GenerateFromPassword(): Failed to generate hash: %v", err)
		}

		admin := models.User{
			Username: defauldAdminUsername,
			Password: string(hash),
			RoleID:   adminRole.ID,
		}

		db.FirstOrCreate(&models.User{}, admin)
	}

	return nil
}
