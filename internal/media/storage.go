// Package media downloads source videos, fetches YouTube metadata and renders
// the short clips that are played during the show.
package media

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Storage is the on-disk layout under DATA_DIR/media.
//
//	source/  one file per source video ({youtubeID}.mp4)
//	thumbs/  YouTube thumbnails ({youtubeID}.jpg)
//	clips/   rendered clips, named {itemID}-{settingsKey}.mp4 so URLs never go stale
//	images/  answer pictures uploaded by editors
//	tmp/     work in progress; wiped on start
type Storage struct {
	Root string
}

func NewStorage(dataDir string) (*Storage, error) {
	s := &Storage{Root: filepath.Join(dataDir, "media")}
	for _, d := range []string{s.SourceDir(), s.ThumbsDir(), s.ClipsDir(), s.ImagesDir(), s.TmpDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Storage) SourceDir() string { return filepath.Join(s.Root, "source") }
func (s *Storage) ThumbsDir() string { return filepath.Join(s.Root, "thumbs") }
func (s *Storage) ClipsDir() string  { return filepath.Join(s.Root, "clips") }
func (s *Storage) ImagesDir() string { return filepath.Join(s.Root, "images") }
func (s *Storage) TmpDir() string    { return filepath.Join(s.Root, "tmp") }

func (s *Storage) Source(name string) string { return filepath.Join(s.SourceDir(), name) }
func (s *Storage) Thumb(name string) string  { return filepath.Join(s.ThumbsDir(), name) }
func (s *Storage) Clip(name string) string   { return filepath.Join(s.ClipsDir(), name) }
func (s *Storage) Image(name string) string  { return filepath.Join(s.ImagesDir(), name) }

// CleanTmp removes leftovers of interrupted work.
func (s *Storage) CleanTmp() {
	entries, _ := os.ReadDir(s.TmpDir())
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(s.TmpDir(), e.Name()))
	}
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir() && fi.Size() > 0
}

// moveFile renames, falling back to copy+delete across filesystems (Docker volumes).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		tmp := dst + ".part"
		out, err := os.Create(tmp)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			os.Remove(tmp)
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
		return os.Remove(src)
	} else {
		return err
	}
}
