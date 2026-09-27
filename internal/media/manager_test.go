package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/store"
)

func newTestManager(t *testing.T) (*Manager, *Storage) {
	t.Helper()
	dir := t.TempDir()
	st, err := NewStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(dir, "m.db"), store.LegacyImport{})
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(db, st, &YtDlp{Path: "/nonexistent"}, &FFmpeg{FFmpeg: "/nonexistent", FFprobe: "/nonexistent"}, nil,
		Options{DownloadWorkers: 1, RenderWorkers: 1, ClipHeight: 720})
	return m, st
}

// An upload interrupted by a restart must not stay "queued" forever.
func TestResumeInterruptedUpload(t *testing.T) {
	m, _ := newTestManager(t)
	med := store.Media{YouTubeID: "file-abc", Status: store.MediaDownloading, SourceKind: "upload"}
	m.db.Create(&med)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	var got store.Media
	m.db.First(&got, med.ID)
	if got.Status != store.MediaError || got.ErrorCode != "missing" {
		t.Fatalf("status = %s/%s, want error/missing", got.Status, got.ErrorCode)
	}
}

// A ready media whose file vanished is downloaded again.
func TestResumeMissingSource(t *testing.T) {
	m, _ := newTestManager(t)
	med := store.Media{YouTubeID: "dQw4w9WgXcQ", Status: store.MediaReady, SourceKind: "youtube", SourceFile: "gone.mp4", Title: "x", ThumbFile: "x.jpg"}
	m.db.Create(&med)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// The download fails fast (no yt-dlp) and the reason is recorded.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var got store.Media
		m.db.First(&got, med.ID)
		if got.Status == store.MediaError {
			if got.ErrorCode != "ytdlp_missing" {
				t.Fatalf("error code = %s", got.ErrorCode)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("missing source was not re-downloaded")
}

func TestSweepKeepsReferencedAndRecentFiles(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	for _, n := range []string{"keep.mp4", "old.mp4", "fresh.mp4"} {
		p := filepath.Join(dir, n)
		os.WriteFile(p, []byte("x"), 0o644)
		if n != "fresh.mp4" {
			os.Chtimes(p, old, old)
		}
	}
	sweep(dir, map[string]bool{"keep.mp4": true})
	for n, want := range map[string]bool{"keep.mp4": true, "old.mp4": false, "fresh.mp4": true} {
		_, err := os.Stat(filepath.Join(dir, n))
		if (err == nil) != want {
			t.Errorf("%s exists=%v, want %v", n, err == nil, want)
		}
	}
}
