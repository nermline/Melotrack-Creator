// Package audit keeps the action log: who did what and when, and what
// happened on its own (downloads, cutting, the show moving on). Every entry is
// a line in the server output ("msg=audit") and is also appended to a monthly
// file in DATA_DIR/logs, which outlives container updates.
package audit

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// System is the author of things nobody did by hand.
const System = "system"

type logger struct {
	mu    sync.Mutex
	dir   string
	month string
	file  *os.File
	text  slog.Handler
}

var std logger

// Open starts appending entries to monthly files in dir (audit-2026-11.log).
func Open(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	std.mu.Lock()
	defer std.mu.Unlock()
	std.dir = dir
	return nil
}

// Close closes the current file; later entries go to the server output only.
func Close() {
	std.mu.Lock()
	defer std.mu.Unlock()
	if std.file != nil {
		std.file.Close()
	}
	std.dir, std.file, std.text, std.month = "", nil, nil, ""
}

// Record writes one entry. who is the person's email ("password" for the
// shared password, System for automatic events); attrs are key-value pairs.
func Record(who, action string, attrs ...any) {
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "audit", 0)
	r.Add("who", who, "action", action)
	r.Add(attrs...)
	if h := slog.Default().Handler(); h.Enabled(context.Background(), slog.LevelInfo) {
		_ = h.Handle(context.Background(), r.Clone())
	}
	std.write(r)
}

func (l *logger) write(r slog.Record) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.dir == "" {
		return
	}
	if month := r.Time.Format("2006-01"); month != l.month {
		if l.file != nil {
			l.file.Close()
		}
		f, err := os.OpenFile(filepath.Join(l.dir, "audit-"+month+".log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			// Warn once a month, not on every entry; the server output still has them.
			slog.Warn("audit log file", "err", err)
			l.file, l.text, l.month = nil, nil, month
			return
		}
		l.file, l.text, l.month = f, slog.NewTextHandler(f, nil), month
	}
	if l.text != nil {
		_ = l.text.Handle(context.Background(), r)
	}
}
