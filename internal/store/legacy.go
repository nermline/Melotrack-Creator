package store

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// LegacyImport tells the migration where pre-v2 files live and where v2 keeps them.
type LegacyImport struct {
	DownloadsDir string // old ./downloads
	SourceDir    string // new source video dir
	ImagesDir    string // new answer image dir
}

var legacyTables = []string{"roles", "users", "projects", "categories", "media", "quiz_items", "answers"}

func renameLegacyTables(tx *gorm.DB) error {
	for _, t := range legacyTables {
		if !tx.Migrator().HasTable(t) {
			continue
		}
		if err := tx.Exec(fmt.Sprintf("ALTER TABLE %q RENAME TO %q", t, "legacy_"+t)).Error; err != nil {
			return fmt.Errorf("rename %s: %w", t, err)
		}
	}
	return nil
}

type fileMove struct {
	kind   string // "source" or "image"
	id     uint   // media or item id
	from   string
	toDir  string
	toName string
}

type legacyItem struct {
	ID         uint
	CategoryID uint
	Position   int
	ShowVideo  bool
	MediaID    *uint
	StartTime  float64
	EndTime    float64
	Volume     float64
	CropX      int
	CropY      int
	CropWidth  int
	CropHeight int
	Fit        bool
	Answer     string
	ImagePath  string
}

func importLegacy(tx *gorm.DB, li LegacyImport) ([]fileMove, error) {
	stmts := []string{
		`INSERT INTO projects (id, created_at, updated_at, title, join_code)
			SELECT id, created_at, updated_at, title, lower(hex(randomblob(3))) FROM legacy_projects`,
		`INSERT INTO categories (id, created_at, updated_at, project_id, title, position)
			SELECT id, created_at, updated_at, project_id, title, position FROM legacy_categories
			WHERE project_id IN (SELECT id FROM projects)`,
		`INSERT INTO media (id, created_at, updated_at, youtube_id, status, source_kind, width, height, duration)
			SELECT id, created_at, updated_at, you_tube_id, 'pending', 'youtube',
				COALESCE(width, 0), COALESCE(height, 0), COALESCE(duration, 0)
			FROM legacy_media WHERE you_tube_id IS NOT NULL AND you_tube_id != ''`,
	}
	for _, s := range stmts {
		if err := tx.Exec(s).Error; err != nil {
			return nil, err
		}
	}

	var moves []fileMove

	var medias []Media
	if err := tx.Find(&medias).Error; err != nil {
		return nil, err
	}
	for _, m := range medias {
		moves = append(moves, fileMove{
			kind: "source", id: m.ID,
			from:  filepath.Join(li.DownloadsDir, "raw", m.YouTubeID+".mp4"),
			toDir: li.SourceDir, toName: m.YouTubeID + ".mp4",
		})
	}

	var olds []legacyItem
	err := tx.Raw(`SELECT q.id, q.category_id, q.position, q.show_video, q.media_id,
			COALESCE(q.start_time, 0) AS start_time, COALESCE(q.end_time, 0) AS end_time,
			COALESCE(q.volume, 1) AS volume,
			COALESCE(q.crop_x, 0) AS crop_x, COALESCE(q.crop_y, 0) AS crop_y,
			COALESCE(q.crop_width, 0) AS crop_width, COALESCE(q.crop_height, 0) AS crop_height,
			q.fit, COALESCE(a.title, '') AS answer, COALESCE(a.image_path, '') AS image_path
		FROM legacy_quiz_items q
		LEFT JOIN legacy_answers a ON a.quiz_item_id = q.id
		WHERE q.category_id IN (SELECT id FROM categories)`).Scan(&olds).Error
	if err != nil {
		return nil, err
	}
	for _, o := range olds {
		if o.MediaID == nil {
			continue
		}
		it := Item{
			ID: o.ID, CategoryID: o.CategoryID, Position: o.Position, MediaID: *o.MediaID,
			Answer: o.Answer, ShowVideo: o.ShowVideo,
			StartSec: o.StartTime, EndSec: o.EndTime, GainDB: volumeToDB(o.Volume),
			Frame: FrameFit, ClipStatus: ClipPending,
		}
		if it.EndSec <= it.StartSec {
			it.EndSec = it.StartSec + 15
		}
		if !o.Fit && o.CropWidth > 0 && o.CropHeight > 0 {
			it.Frame = FrameCrop
			it.CropX, it.CropY, it.CropW, it.CropH = o.CropX, o.CropY, o.CropWidth, o.CropHeight
		}
		if err := tx.Omit("Media").Create(&it).Error; err != nil {
			return nil, fmt.Errorf("item %d: %w", o.ID, err)
		}
		if o.ImagePath != "" {
			rel := strings.TrimPrefix(o.ImagePath, "/answers/")
			moves = append(moves, fileMove{
				kind: "image", id: it.ID,
				from:  filepath.Join(li.DownloadsDir, "answers", filepath.FromSlash(rel)),
				toDir: li.ImagesDir, toName: fmt.Sprintf("%d-%s.jpg", it.ID, NewJoinCode()),
			})
		}
	}

	for i := len(legacyTables) - 1; i >= 0; i-- {
		if err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", "legacy_"+legacyTables[i])).Error; err != nil {
			return nil, err
		}
	}
	return moves, nil
}

// finishLegacyFiles copies old files into the v2 layout. Anything missing is simply
// re-downloaded later (media stays "pending"); a missing image is dropped.
func finishLegacyFiles(db *gorm.DB, moves []fileMove) {
	for _, m := range moves {
		if err := os.MkdirAll(m.toDir, 0o755); err != nil {
			slog.Warn("legacy import: mkdir", "dir", m.toDir, "err", err)
			continue
		}
		dst := filepath.Join(m.toDir, m.toName)
		if err := copyFile(m.from, dst); err != nil {
			slog.Warn("legacy import: file not carried over", "from", m.from, "err", err)
			continue
		}
		switch m.kind {
		case "source":
			db.Model(&Media{}).Where("id = ?", m.id).Updates(map[string]any{
				"source_file": m.toName, "status": MediaReady, "version": 1,
			})
		case "image":
			db.Model(&Item{}).Where("id = ?", m.id).Update("image_file", m.toName)
		}
	}
	slog.Info("legacy import finished; the old downloads directory can be deleted once everything looks right")
}

func volumeToDB(v float64) float64 {
	if v <= 0 || v == 1 {
		return 0
	}
	db := 20 * math.Log10(v)
	return math.Round(db*10) / 10
}
