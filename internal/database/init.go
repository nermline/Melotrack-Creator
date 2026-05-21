package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func InitDB(path string, adminPassword, editorPassword, operatorPassword string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("initDB(): gorm.Open() Failed to open DB: %v", err)
	}

	// Прибираємо застарілу схему власності проєктів до AutoMigrate.
	migrateLegacyProjectOwnership(db)

	err = db.AutoMigrate(
		&models.Role{},
		&models.User{},
		&models.Media{},
		&models.Project{},
		&models.Category{},
		&models.QuizItem{},
		&models.Answer{},
	)

	if err != nil {
		return nil, fmt.Errorf("initDB(): db.AutoMigrate() Failed to migrate: %v", err)
	}

	seedRoles(db)

	// Початкові акаунти: адмін (повний доступ), редактор (створення/редагування),
	// оператор (лише читання — для показу як екран або пульт).
	seeds := []struct{ username, password, role string }{
		{"admin", adminPassword, "admin"},
		{"editor", editorPassword, "editor"},
		{"operator", operatorPassword, "operator"},
	}
	for _, s := range seeds {
		if err := seedUser(db, s.username, s.password, s.role); err != nil {
			return nil, fmt.Errorf("InitDB(): %v", err)
		}
	}

	return db, nil
}

// migrateLegacyProjectOwnership видаляє застарілий стовпець user_id та композитний
// унікальний індекс (user_id, title) з таблиці projects. Раніше проєкти належали
// конкретному користувачу; тепер вони спільні для всіх (self-hosted).
func migrateLegacyProjectOwnership(db *gorm.DB) {
	m := db.Migrator()
	if !m.HasTable(&models.Project{}) {
		return
	}
	if m.HasColumn(&models.Project{}, "user_id") {
		// Спершу прибираємо індекс, що залежить від стовпця, потім сам стовпець.
		_ = db.Exec("DROP INDEX IF EXISTS idx_user_title").Error
		_ = m.DropColumn(&models.Project{}, "user_id")
	}
}

func seedRoles(db *gorm.DB) {
	roles := []string{"admin", "editor", "operator"}
	for _, name := range roles {
		db.FirstOrCreate(&models.Role{}, models.Role{Name: name})
	}
}

// seedUser створює користувача з заданою роллю, якщо його ще немає.
func seedUser(db *gorm.DB, username, password, roleName string) error {
	var role models.Role
	if err := db.Where("name = ?", roleName).First(&role).Error; err != nil {
		return fmt.Errorf("seedUser(%q): role %q not found: %v", username, roleName, err)
	}

	var count int64
	db.Model(&models.User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("seedUser(%q): GenerateFromPassword: %v", username, err)
	}

	user := models.User{
		Username: username,
		Password: string(hash),
		RoleID:   role.ID,
	}
	return db.FirstOrCreate(&models.User{}, user).Error
}
