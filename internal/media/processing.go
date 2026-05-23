package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/nermline/Melotrack-Creator/internal/models"
)

type ffprobeResult struct {
	Streams []struct {
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Duration string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

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

	if duration == 0 {
		if d, err := strconv.ParseFloat(info.Format.Duration, 64); err == nil {
			duration = d
		}
	}

	return
}

func RunFFmpegCropAndTrim(ctx context.Context, rawPath, outPath string, v models.Video) error {
	args := []string{"-y"}

	if v.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%f", v.StartTime))
	}

	if v.EndTime > 0 && v.EndTime > v.StartTime {
		duration := v.EndTime - v.StartTime
		args = append(args, "-t", fmt.Sprintf("%f", duration))
	}

	args = append(args, "-i", rawPath)

	if v.Fit {

		args = append(args, "-vf",
			"scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1")
	} else if v.CropWidth > 0 && v.CropHeight > 0 {
		vfArg := fmt.Sprintf("crop=%d:%d:%d:%d", v.CropWidth, v.CropHeight, v.CropX, v.CropY)
		args = append(args, "-vf", vfArg)
	}

	args = append(args, "-c:v", "libx264", "-crf", "23", "-preset", "veryfast")
	args = append(args, "-af", fmt.Sprintf("volume=%f", v.Volume))
	args = append(args, "-c:a", "aac", "-b:a", "192k", "-f", "mp4", outPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	return cmd.Run()
}
