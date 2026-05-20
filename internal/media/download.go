package media

import (
	"context"
	"fmt"
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

	ytdlp.MustInstallAll(context.TODO())

	outPath := fmt.Sprintf("./downloads/raw/%s.mp4", mediaID)

	formatFilter := "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best[height<=1080]"

	dl := ytdlp.New().
		Format(formatFilter).
		MergeOutputFormat("mp4").
		Output(outPath)

	res, err := dl.Run(context.Background(), youtubeURL)
	if err != nil {
		return "", "", fmt.Errorf("помилка завантаження yt-dlp: %w", err)
	}

	fmt.Println(res.OutputLogs)

	return outPath, mediaID, nil
}
