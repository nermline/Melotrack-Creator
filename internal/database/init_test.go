package database

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"gorm.io/gorm"
)

func closeOnCleanup(t *testing.T, db *gorm.DB) {
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestInitDB_SeedsRolesAndUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.db")
	db, err := InitDB(path, "adminpw", "editorpw", "operpw")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	closeOnCleanup(t, db)

	var roles, users int64
	db.Model(&models.Role{}).Count(&roles)
	db.Model(&models.User{}).Count(&users)
	if roles != 3 {
		t.Errorf("roles = %d, want 3", roles)
	}
	if users != 3 {
		t.Errorf("users = %d, want 3", users)
	}

	for _, u := range []struct{ name, role string }{{"admin", "admin"}, {"editor", "editor"}, {"operator", "operator"}} {
		var user models.User
		if err := db.Preload("Role").Where("username = ?", u.name).First(&user).Error; err != nil {
			t.Errorf("user %q not found: %v", u.name, err)
			continue
		}
		if user.Role.Name != u.role {
			t.Errorf("user %q has role %q, want %q", u.name, user.Role.Name, u.role)
		}
	}
}

func TestInitDB_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idem.db")
	db1, err := InitDB(path, "a", "e", "o")
	if err != nil {
		t.Fatalf("first InitDB: %v", err)
	}

	if sqlDB, err := db1.DB(); err == nil {
		_ = sqlDB.Close()
	}
	db, err := InitDB(path, "a", "e", "o")
	if err != nil {
		t.Fatalf("second InitDB: %v", err)
	}
	closeOnCleanup(t, db)
	var users int64
	db.Model(&models.User{}).Count(&users)
	if users != 3 {
		t.Errorf("after second init users = %d, want 3 (no duplicates)", users)
	}
}

func TestMigrateLegacyProjectOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	closeOnCleanup(t, db)

	stmts := []string{
		`CREATE TABLE projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME, updated_at DATETIME,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX idx_user_title ON projects(user_id, title)`,
		`INSERT INTO projects (user_id, title) VALUES (1, 'Legacy A'), (2, 'Legacy B')`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatalf("setup %q: %v", s, err)
		}
	}

	migrateLegacyProjectOwnership(db)

	if db.Migrator().HasColumn(&models.Project{}, "user_id") {
		t.Fatal("user_id column should have been removed")
	}

	var count int64
	db.Table("projects").Count(&count)
	if count != 2 {
		t.Errorf("expected 2 preserved rows, got %d", count)
	}

	if err := db.AutoMigrate(&models.Project{}); err != nil {
		t.Fatalf("AutoMigrate after migration: %v", err)
	}
	if err := db.Create(&models.Project{Title: "Fresh"}).Error; err != nil {
		t.Errorf("create project after migration failed: %v", err)
	}
}

func TestMigrateLegacyProjectOwnership_NoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "noop.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	closeOnCleanup(t, db)

	migrateLegacyProjectOwnership(db)

	if err := db.AutoMigrate(&models.Project{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	migrateLegacyProjectOwnership(db)
	if !db.Migrator().HasTable(&models.Project{}) {
		t.Error("projects table should still exist after no-op migration")
	}
}
