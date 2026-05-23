package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nermline/Melotrack-Creator/internal/models"
	"github.com/nermline/Melotrack-Creator/ws"
	"gorm.io/gorm"
)

func StartDownloadWorker(ctx context.Context, db *gorm.DB, mediaID uint, youtubeURL string) {
	var mediaFile models.Media
	if err := db.First(&mediaFile, mediaID).Error; err != nil {
		return
	}

	rawPath, _, err := DownloadRawVideo(ctx, youtubeURL)
	if err != nil {
		db.Model(&mediaFile).Update("status", "error")
		return
	}

	width, height, duration := GetVideoMetadata(rawPath)

	db.Model(&mediaFile).Updates(map[string]interface{}{
		"status":    "ready",
		"file_path": rawPath,
		"width":     width,
		"height":    height,
		"duration":  duration,
	})
}

func StartRenderWorker(ctx context.Context, db *gorm.DB, hub *ws.Hub, item models.QuizItem, projectID string, categoryID string) {
	failProcessing := func(err error) {

		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		db.Model(&item).Update("render_status", "error")

		hub.SystemBroadcast(categoryID, ws.EditorMessage{
			Action: "item_render_error",
			ItemID: item.ID,
			Data:   map[string]string{"render_status": "error"},
		})

		fmt.Printf("Помилка рендерингу для Item %d: %v\n", item.ID, err)
	}

	outputDir := filepath.Join(".", "downloads", "processed", projectID, fmt.Sprintf("%d", item.CategoryID))
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		failProcessing(fmt.Errorf("не вдалося створити директорію: %w", err))
		return
	}

	newClipPath := filepath.Join(outputDir, fmt.Sprintf("%d.mp4", item.ID))
	tmpClipPath := newClipPath + ".tmp"

	_ = os.Remove(newClipPath)
	_ = os.Remove(tmpClipPath)

	if err := RunFFmpegCropAndTrim(ctx, item.Video.Media.FilePath, tmpClipPath, item.Video); err != nil {
		_ = os.Remove(tmpClipPath)
		failProcessing(err)
		return
	}

	if ctx.Err() == nil {
		if err := os.Rename(tmpClipPath, newClipPath); err != nil {
			_ = os.Remove(tmpClipPath)
			failProcessing(fmt.Errorf("не вдалося перейменувати фінальний кліп: %v", err))
			return
		}

		webURL := fmt.Sprintf("/media/%s/%d/%d.mp4", projectID, item.CategoryID, item.ID)

		db.Model(&item).Updates(map[string]interface{}{
			"render_status":   "ready",
			"ready_file_path": webURL,
		})

		hub.SystemBroadcast(categoryID, ws.EditorMessage{
			Action: "item_render_ready",
			ItemID: item.ID,
			Data:   map[string]string{"render_status": "ready"},
		})

		fmt.Printf("Рендер успішно завершено для Item %d\n", item.ID)
	} else {

		_ = os.Remove(tmpClipPath)
	}
}
