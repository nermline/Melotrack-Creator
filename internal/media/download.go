package media

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/lrstanley/go-ytdlp"
)

func ExtractYouTubeID(videoURL string) string {
	re := regexp.MustCompile(`(?:youtube\.com\/(?:[^\/]+\/.+\/|(?:v|e(?:mbed)?)\/|.*[?&]v=)|youtu\.be\/)([^"&?\/\s]{11})`)

	matches := re.FindStringSubmatch(videoURL)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func DownloadRawVideo(ctx context.Context, youtubeURL string) (string, string, error) {
	if err := os.MkdirAll("./downloads/raw", os.ModePerm); err != nil {
		return "", "", fmt.Errorf("не вдалося створити директорію: %w", err)
	}

	mediaID := ExtractYouTubeID(youtubeURL)

	// Очищаємо URL від &list=... та інших параметрів, які змушують yt-dlp
	// завантажувати весь плейлист замість конкретного відео.
	// Без цього rename може впасти → статус Media ніколи не стане "ready".
	cleanURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", mediaID)

	outPath := fmt.Sprintf("./downloads/raw/%s.mp4", mediaID)

	tmpBase := fmt.Sprintf("./downloads/raw/%s.tmp", mediaID)
	tmpActualPath := tmpBase + ".mp4"

	_ = os.Remove(tmpActualPath)

	formatFilter := "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best[height<=1080]"

	dl := ytdlp.New().
		Format(formatFilter).
		MergeOutputFormat("mp4").
		Output(tmpBase)

	res, err := dl.Run(ctx, cleanURL)
	if err != nil {
		_ = os.Remove(tmpActualPath)
		return "", "", fmt.Errorf("помилка завантаження yt-dlp: %w", err)
	}

	fmt.Println(res.OutputLogs)

	if err := os.Rename(tmpActualPath, outPath); err != nil {
		_ = os.Remove(tmpActualPath)
		return "", "", fmt.Errorf("помилка збереження відео: %w", err)
	}

	return outPath, mediaID, nil
}
