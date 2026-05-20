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

func DownloadRawVideo(youtubeURL string) (string, string, error) {
	mediaID := ExtractYouTubeID(youtubeURL)

	outPath := fmt.Sprintf("./downloads/raw/%s.mp4", mediaID)
	tmpPath := outPath + ".tmp"

	_ = os.Remove(tmpPath)

	formatFilter := "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best[height<=1080]"

	dl := ytdlp.New().
		Format(formatFilter).
		MergeOutputFormat("mp4").
		Output(tmpPath)

	res, err := dl.Run(context.Background(), youtubeURL)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("помилка завантаження yt-dlp: %w", err)
	}

	fmt.Println(res.OutputLogs)

	if err := os.Rename(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", "", fmt.Errorf("помилка збереження відео: %w", err)
	}

	return outPath, mediaID, nil
}
