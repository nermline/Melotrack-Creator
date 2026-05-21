package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/nermline/Melotrack-Creator/internal/models"
)

// ffprobeResult — структура для парсингу виводу ffprobe
type ffprobeResult struct {
	Streams []struct {
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Duration string `json:"duration"` // може бути "N/A" для деяких форматів
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// GetVideoMetadata запускає ffprobe і повертає розміри та тривалість відео.
// При помилці повертає нулі — валідація скіпається якщо width/height == 0.
func GetVideoMetadata(filePath string) (width, height int, duration float64) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "v:0",
		filePath,
	)

	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("ffprobe помилка для %s: %v\n", filePath, err)
		return 0, 0, 0
	}

	var info ffprobeResult
	if err := json.Unmarshal(out, &info); err != nil {
		fmt.Printf("ffprobe: помилка парсингу JSON: %v\n", err)
		return 0, 0, 0
	}

	if len(info.Streams) > 0 {
		width = info.Streams[0].Width
		height = info.Streams[0].Height
		if d, err := strconv.ParseFloat(info.Streams[0].Duration, 64); err == nil && d > 0 {
			duration = d
		}
	}

	// Fallback: duration з секції format (надійніше для деяких контейнерів)
	if duration == 0 {
		if d, err := strconv.ParseFloat(info.Format.Duration, 64); err == nil {
			duration = d
		}
	}

	return
}

func RunFFmpegCropAndTrim(ctx context.Context, rawPath, outPath string, v models.Video) error {
	args := []string{"-y"}

	// 1. -ss ПЕРЕД -i для швидкого позиціонування (Fast Seeking)
	if v.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%f", v.StartTime))
	}

	// 2. Точна тривалість (-t) замість -to — гарантує ідеальну довжину при рекодингу
	if v.EndTime > 0 && v.EndTime > v.StartTime {
		duration := v.EndTime - v.StartTime
		args = append(args, "-t", fmt.Sprintf("%f", duration))
	}

	args = append(args, "-i", rawPath)

	// 3. Завжди реенкодимо libx264 — новий ключовий кадр ТОЧНО на StartTime
	if v.CropWidth > 0 && v.CropHeight > 0 {
		vfArg := fmt.Sprintf("crop=%d:%d:%d:%d", v.CropWidth, v.CropHeight, v.CropX, v.CropY)
		args = append(args, "-vf", vfArg)
	}

	args = append(args, "-c:v", "libx264", "-crf", "23", "-preset", "veryfast")
	args = append(args, "-af", fmt.Sprintf("volume=%f", v.Volume))
	args = append(args, "-c:a", "aac", "-b:a", "192k", "-f", "mp4", outPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	return cmd.Run()
}
