package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nermline/Melotrack-Creator/internal/models"
	"gorm.io/gorm"
)

func StartVideoProcessingWorker(ctx context.Context, db *gorm.DB, itemID uint, urlChanged bool, projectID string) {
	var item models.QuizItem
	if err := db.First(&item, itemID).Error; err != nil {
		return
	}

	db.Model(&item).Update("processing_status", "processing")

	failProcessing := func(err error) {
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		db.Model(&item).Update("processing_status", "error")
		fmt.Printf("Помилка обробки відео для Item %d: %v\n", item.ID, err)
	}

	ytID := ExtractYouTubeID(item.Video.YouTubeURL)
	if ytID == "" {
		failProcessing(errors.New("invalid youtube url"))
		return
	}

	var mediaFile models.Media

	if urlChanged || item.Video.MediaID == nil {
		err := db.Where("you_tube_id = ?", ytID).First(&mediaFile).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			rawPath, _, err := DownloadRawVideo(ctx, item.Video.YouTubeURL)
			if err != nil {
				failProcessing(err)
				return
			}

			mediaFile = models.Media{
				YouTubeID: ytID,
				FilePath:  rawPath,
			}

			if err := db.Create(&mediaFile).Error; err != nil {
				failProcessing(err)
				return
			}
		} else if err != nil {
			failProcessing(err)
			return
		}

		db.Model(&item).Update("media_id", mediaFile.ID)
		item.Video.MediaID = &mediaFile.ID
	} else {
		if err := db.First(&mediaFile, *item.Video.MediaID).Error; err != nil {
			failProcessing(err)
			return
		}
	}

	outputDir := filepath.Join(".", "downloads", "processed", projectID, fmt.Sprintf("%d", item.CategoryID))
	_ = os.MkdirAll(outputDir, os.ModePerm)

	newClipPath := filepath.Join(outputDir, fmt.Sprintf("%d.mp4", item.ID))
	tmpClipPath := newClipPath + ".tmp"

	_ = os.Remove(newClipPath)
	_ = os.Remove(tmpClipPath)

	if err := RunFFmpegCropAndTrim(ctx, mediaFile.FilePath, tmpClipPath, item.Video); err != nil {
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
			"processing_status": "ready",
			"ready_file_path":   webURL,
		})
	} else {
		_ = os.Remove(tmpClipPath)
	}
}

func RunFFmpegCropAndTrim(ctx context.Context, rawPath, outPath string, v models.Video) error {
	args := []string{"-y"}

	// 1. Залишаємо -ss ПЕРЕД -i для швидкого позиціонування (Fast Seeking)
	if v.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%f", v.StartTime))
	}

	// 2. Замість -to краще використовувати точну тривалість (-t).
	// Це гарантує ідеальну довжину файлу при рекодингу.
	if v.EndTime > 0 && v.EndTime > v.StartTime {
		duration := v.EndTime - v.StartTime
		args = append(args, "-t", fmt.Sprintf("%f", duration))
	}

	args = append(args, "-i", rawPath)

	// 3. ЗАВЖДИ реенкодимо відео за допомогою libx264.
	// Це дозволяє FFmpeg створити новий ключовий кадр ТОЧНО на StartTime.
	if v.CropWidth > 0 && v.CropHeight > 0 {
		vfArg := fmt.Sprintf("crop=%d:%d:%d:%d", v.CropWidth, v.CropHeight, v.CropX, v.CropY)
		args = append(args, "-vf", vfArg)
	}

	// Використовуємо пресет 'veryfast' або 'ultrafast' для миттєвої обробки на сервері.
	// crf 23 забезпечує чудову якість при невеликому розмірі файлу.
	args = append(args, "-c:v", "libx264", "-crf", "23", "-preset", "veryfast")

	// 4. Обробка аудіо (залишається як була)
	args = append(args, "-af", fmt.Sprintf("volume=%f", v.Volume))
	args = append(args, "-c:a", "aac", "-b:a", "192k", "-f", "mp4", outPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	return cmd.Run()
}
