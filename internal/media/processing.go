package media

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/nermline/Melotrack-Creator/internal/models"
)

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
