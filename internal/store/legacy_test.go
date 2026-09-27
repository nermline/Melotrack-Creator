package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

// The exact schema produced by the pre-v2 version (GORM AutoMigrate).
const legacySchema = `
CREATE TABLE roles (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, deleted_at datetime, name text NOT NULL);
CREATE UNIQUE INDEX idx_roles_name ON roles(name);
CREATE TABLE users (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, deleted_at datetime, username text NOT NULL, password text NOT NULL, role_id integer NOT NULL, CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES roles(id));
CREATE TABLE media (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, you_tube_id text, file_path text NOT NULL, status text DEFAULT "downloading", width integer, height integer, duration real);
CREATE UNIQUE INDEX idx_media_you_tube_id ON media(you_tube_id);
CREATE TABLE projects (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, title text NOT NULL, CONSTRAINT uni_projects_title UNIQUE (title));
CREATE TABLE categories (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, project_id integer NOT NULL, title text NOT NULL, position integer NOT NULL DEFAULT 0, CONSTRAINT fk_projects_categories FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE);
CREATE UNIQUE INDEX idx_project_title ON categories(project_id, title);
CREATE TABLE answers (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, quiz_item_id integer NOT NULL, title text, image_path text, CONSTRAINT fk_quiz_items_answer FOREIGN KEY (quiz_item_id) REFERENCES quiz_items(id) ON DELETE CASCADE);
CREATE TABLE quiz_items (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, category_id integer NOT NULL, position integer NOT NULL DEFAULT 0, show_video numeric NOT NULL DEFAULT false, you_tube_url text, media_id integer, start_time real, end_time real, volume real DEFAULT 1, crop_x integer, crop_y integer, crop_width integer, crop_height integer, fit numeric NOT NULL DEFAULT false, render_status text DEFAULT "unrendered", ready_file_path text, CONSTRAINT fk_quiz_items_media FOREIGN KEY (media_id) REFERENCES media(id), CONSTRAINT fk_categories_items FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE);
CREATE INDEX idx_quiz_items_media_id ON quiz_items(media_id);

INSERT INTO roles (id, name) VALUES (1, 'admin');
INSERT INTO users (id, username, password, role_id) VALUES (1, 'admin', 'x', 1);
INSERT INTO projects (id, created_at, updated_at, title) VALUES (1, '2026-05-24 18:31:00', '2026-05-24 18:31:00', 'Мелотрек');
INSERT INTO categories (id, project_id, title, position) VALUES (1, 1, 'Музика з ігор', 0);
INSERT INTO media (id, you_tube_id, file_path, status, width, height, duration) VALUES
	(2, 'FQSHcl6TJb4', './downloads/raw/FQSHcl6TJb4.mp4', 'ready', 1920, 1080, 146.4),
	(3, 'BTthtlT80Rc', './downloads/raw/BTthtlT80Rc.mp4', 'ready', 1080, 1080, 149.6);
INSERT INTO quiz_items (id, category_id, position, show_video, you_tube_url, media_id, start_time, end_time, volume, crop_x, crop_y, crop_width, crop_height, fit, render_status) VALUES
	(2, 1, 0, 0, 'https://www.youtube.com/watch?v=FQSHcl6TJb4', 2, 0, 15, 1.23, 0, 0, 0, 0, 0, 'ready'),
	(3, 1, 1, 1, 'https://www.youtube.com/watch?v=BTthtlT80Rc', 3, 108, 123, 1, 0, 0, 1080, 608, 0, 'ready');
INSERT INTO answers (quiz_item_id, title, image_path) VALUES (2, 'Stardew', '/answers/1/1/2.jpg'), (3, 'Minecraft', '');
`

func TestLegacyImport(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.sql")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range splitSQL(legacySchema) {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	raw.Close()

	downloads := filepath.Join(dir, "downloads")
	os.MkdirAll(filepath.Join(downloads, "raw"), 0o755)
	os.MkdirAll(filepath.Join(downloads, "answers", "1", "1"), 0o755)
	os.WriteFile(filepath.Join(downloads, "raw", "FQSHcl6TJb4.mp4"), []byte("video"), 0o644)
	os.WriteFile(filepath.Join(downloads, "answers", "1", "1", "2.jpg"), []byte("jpeg"), 0o644)
	li := LegacyImport{DownloadsDir: downloads, SourceDir: filepath.Join(dir, "src"), ImagesDir: filepath.Join(dir, "img")}

	db, err := Open(dbPath, li)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath + ".pre-v2.bak"); err != nil {
		t.Fatal("backup not written")
	}

	var p Project
	if err := db.First(&p, 1).Error; err != nil || p.Title != "Мелотрек" || p.JoinCode == "" || p.Theme != "neon" {
		t.Fatalf("project = %+v, err %v", p, err)
	}
	var items []Item
	db.Preload("Media").Order("position").Find(&items)
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	a, b := items[0], items[1]
	if a.Answer != "Stardew" || a.GainDB != 1.8 || a.Frame != FrameFit || a.ImageFile == "" {
		t.Fatalf("item a = %+v", a)
	}
	if a.Media.Status != MediaReady || a.Media.SourceFile != "FQSHcl6TJb4.mp4" {
		t.Fatalf("media with file should be ready: %+v", a.Media)
	}
	if b.Frame != FrameCrop || b.CropW != 1080 || b.StartSec != 108 || !b.ShowVideo {
		t.Fatalf("item b = %+v", b)
	}
	if b.Media.Status != MediaPending {
		t.Fatalf("media without file should be re-downloaded: %+v", b.Media)
	}
	if db.Migrator().HasTable("legacy_quiz_items") || db.Migrator().HasTable("users") {
		t.Fatal("legacy tables left behind")
	}
	// Re-opening must be a no-op.
	sqlDB, _ := db.DB()
	sqlDB.Close()
	if _, err := Open(dbPath, li); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}

func TestFreshDatabase(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "x", "m.db"), LegacyImport{})
	if err != nil {
		t.Fatal(err)
	}
	p := Project{Title: "A", Theme: "neon", ThinkSeconds: 10, JoinCode: NewJoinCode()}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	dup := Project{Title: "A", Theme: "neon", ThinkSeconds: 10, JoinCode: NewJoinCode()}
	if err := db.Create(&dup).Error; !IsUniqueViolation(err) {
		t.Fatalf("expected unique violation, got %v", err)
	}
	// Foreign keys are enforced: deleting the project removes its categories.
	c := Category{ProjectID: p.ID, Title: "c"}
	db.Omit("Items").Create(&c)
	db.Delete(&Project{}, p.ID)
	var n int64
	db.Model(&Category{}).Count(&n)
	if n != 0 {
		t.Fatal("cascade delete did not happen — foreign keys are off")
	}
}
