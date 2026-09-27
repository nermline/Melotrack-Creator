// Package store owns the SQLite schema, migrations and the data model.
package store

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open opens (creating if needed) the database and brings the schema up to date.
// legacy describes where pre-v2 files live; it is only used for the one-time import.
func Open(path string, legacy LegacyImport) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: create db dir: %w", err)
		}
	}
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// SQLite serialises writers anyway; one connection rules out "database is locked".
	// Consequence: never touch the outer *gorm.DB inside a Transaction callback.
	sqlDB.SetMaxOpenConns(1)

	if err := migrate(db, path, legacy); err != nil {
		return nil, err
	}
	return db, nil
}

var migrations = []string{
	// 1: v2 schema.
	`
CREATE TABLE projects (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME, updated_at DATETIME,
	title TEXT NOT NULL UNIQUE,
	theme TEXT NOT NULL DEFAULT 'neon',
	think_seconds INTEGER NOT NULL DEFAULT 10,
	registration_open INTEGER NOT NULL DEFAULT 0,
	join_code TEXT NOT NULL UNIQUE,
	game_state TEXT NOT NULL DEFAULT ''
);
CREATE TABLE categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME, updated_at DATETIME,
	project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_categories_project ON categories(project_id, position);
CREATE TABLE media (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME, updated_at DATETIME,
	youtube_id TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL DEFAULT '',
	channel TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending',
	error TEXT NOT NULL DEFAULT '',
	error_code TEXT NOT NULL DEFAULT '',
	source_file TEXT NOT NULL DEFAULT '',
	source_kind TEXT NOT NULL DEFAULT '',
	thumb_file TEXT NOT NULL DEFAULT '',
	width INTEGER NOT NULL DEFAULT 0,
	height INTEGER NOT NULL DEFAULT 0,
	duration REAL NOT NULL DEFAULT 0,
	version INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME, updated_at DATETIME,
	category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
	position INTEGER NOT NULL DEFAULT 0,
	media_id INTEGER NOT NULL REFERENCES media(id),
	answer TEXT NOT NULL DEFAULT '',
	image_file TEXT NOT NULL DEFAULT '',
	show_video INTEGER NOT NULL DEFAULT 0,
	start_sec REAL NOT NULL DEFAULT 0,
	end_sec REAL NOT NULL DEFAULT 15,
	gain_db REAL NOT NULL DEFAULT 0,
	frame TEXT NOT NULL DEFAULT 'fit',
	crop_x INTEGER NOT NULL DEFAULT 0,
	crop_y INTEGER NOT NULL DEFAULT 0,
	crop_w INTEGER NOT NULL DEFAULT 0,
	crop_h INTEGER NOT NULL DEFAULT 0,
	clip_status TEXT NOT NULL DEFAULT 'pending',
	clip_key TEXT NOT NULL DEFAULT '',
	clip_file TEXT NOT NULL DEFAULT '',
	clip_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_items_category ON items(category_id, position);
CREATE INDEX idx_items_media ON items(media_id);
CREATE TABLE teams (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME, updated_at DATETIME,
	project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	name_key TEXT NOT NULL,
	bonus REAL NOT NULL DEFAULT 0,
	UNIQUE (project_id, name_key)
);
CREATE TABLE scores (
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	points REAL NOT NULL DEFAULT 0,
	updated_at DATETIME,
	PRIMARY KEY (team_id, item_id)
);
CREATE INDEX idx_scores_item ON scores(item_id);
`,
}

func migrate(db *gorm.DB, path string, legacy LegacyImport) error {
	var version int
	if err := db.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		return fmt.Errorf("store: read schema version: %w", err)
	}
	if version >= len(migrations) {
		return nil
	}

	isLegacy := version == 0 && db.Migrator().HasTable("quiz_items")
	if isLegacy {
		backup := path + ".pre-v2.bak"
		if err := copyFile(path, backup); err != nil {
			return fmt.Errorf("store: backup legacy db: %w", err)
		}
		slog.Info("legacy database detected, backup written", "backup", backup)
	}

	// Foreign keys must be off while legacy tables are renamed and copied.
	if err := db.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return err
	}
	defer db.Exec("PRAGMA foreign_keys = ON")

	var moves []fileMove
	err := db.Transaction(func(tx *gorm.DB) error {
		if isLegacy {
			if err := renameLegacyTables(tx); err != nil {
				return err
			}
		}
		for v := version; v < len(migrations); v++ {
			for _, stmt := range splitSQL(migrations[v]) {
				if err := tx.Exec(stmt).Error; err != nil {
					return fmt.Errorf("store: migration %d: %w", v+1, err)
				}
			}
		}
		if isLegacy {
			var err error
			if moves, err = importLegacy(tx, legacy); err != nil {
				return fmt.Errorf("store: legacy import: %w", err)
			}
		}
		return tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations))).Error
	})
	if err != nil {
		return err
	}
	if isLegacy {
		finishLegacyFiles(db, moves)
	}
	return nil
}

func splitSQL(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// NewJoinCode returns a short, URL-safe code without ambiguous characters.
func NewJoinCode() string {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// IsUniqueViolation reports whether err came from a UNIQUE constraint.
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
