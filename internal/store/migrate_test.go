package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

// A database created by the first v2 release (schema 1) keeps its data when
// registration codes and team bonuses are dropped and negative points are cleared.
func TestMigrateFromSchema1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	stmts := append(splitSQL(migrations[0]),
		`INSERT INTO projects (id, title, theme, think_seconds, registration_open, join_code, game_state)
			VALUES (7, 'Мелотрек', 'vinyl', 12, 1, 'abc123', '{"phase":"answers"}')`,
		`INSERT INTO categories (id, project_id, title) VALUES (3, 7, 'Хіти')`,
		`INSERT INTO teams (id, project_id, name, name_key, bonus) VALUES (5, 7, 'Бітли', 'бітли', 2)`,
		`INSERT INTO media (id, youtube_id) VALUES (1, 'dQw4w9WgXcQ')`,
		`INSERT INTO items (id, category_id, media_id) VALUES (9, 3, 1), (10, 3, 1)`,
		`INSERT INTO scores (team_id, item_id, points) VALUES (5, 9, -1.5), (5, 10, 2)`,
		`PRAGMA user_version = 1`)
	for _, s := range stmts {
		if _, err := raw.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	raw.Close()

	db, err := Open(path, LegacyImport{})
	if err != nil {
		t.Fatal(err)
	}
	var p Project
	if err := db.First(&p, 7).Error; err != nil || p.Title != "Мелотрек" || p.Theme != "vinyl" || p.ThinkSeconds != 12 || p.GameState == "" {
		t.Fatalf("project = %+v, err %v", p, err)
	}
	for table, column := range map[string]string{"projects": "join_code", "teams": "bonus"} {
		if db.Migrator().HasColumn(table, column) {
			t.Errorf("%s.%s should be gone", table, column)
		}
	}
	var team Team
	if err := db.First(&team, 5).Error; err != nil || team.Name != "Бітли" {
		t.Fatalf("team = %+v, err %v", team, err)
	}
	var pts []float64
	db.Model(&Score{}).Order("item_id").Pluck("points", &pts)
	if len(pts) != 2 || pts[0] != 0 || pts[1] != 2 {
		t.Fatalf("negative points must become 0, got %v", pts)
	}
	// Foreign keys still point at the rebuilt table: deleting cascades.
	db.Delete(&Project{}, 7)
	var n int64
	db.Model(&Category{}).Count(&n)
	if n != 0 {
		t.Fatal("categories were not removed with their project")
	}
	db.Model(&Team{}).Count(&n)
	if n != 0 {
		t.Fatal("teams were not removed with their project")
	}
	// A new project needs no registration code.
	if err := db.Create(&Project{Title: "Новий", Theme: "neon", ThinkSeconds: 10}).Error; err != nil {
		t.Fatal(err)
	}
}
