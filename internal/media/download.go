package media

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/lrstanley/go-ytdlp"
)

const cookiesFile = "cookies.txt"

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

	cleanURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", mediaID)

	outPath := fmt.Sprintf("./downloads/raw/%s.mp4", mediaID)

	tmpBase := fmt.Sprintf("./downloads/raw/%s.tmp", mediaID)
	tmpActualPath := tmpBase + ".mp4"

	_ = os.Remove(tmpActualPath)

	formatFilter := "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best[height<=1080]"

	dl := ytdlp.New().
		Format(formatFilter).
		MergeOutputFormat("mp4").
		Verbose().
		StderrFunc(func(line string) {
			log.Printf("[yt-dlp %s] %s", mediaID, line)
		}).
		Output(tmpBase)

	if _, err := os.Stat(cookiesFile); err == nil {
		log.Printf("[yt-dlp %s] використовую cookies: %s", mediaID, cookiesFile)
		dl = dl.Cookies(cookiesFile)
	} else {
		log.Printf("[yt-dlp %s] cookies.txt не знайдено — завантаження без кукісів", mediaID)
	}

	log.Printf("[yt-dlp %s] старт завантаження: %s", mediaID, cleanURL)

	res, err := dl.Run(ctx, cleanURL)
	if err != nil {
		_ = os.Remove(tmpActualPath)
		if res != nil {
			log.Printf("[yt-dlp %s] ПОМИЛКА: %v\n--- stderr ---\n%s", mediaID, err, res.Stderr)
		} else {
			log.Printf("[yt-dlp %s] ПОМИЛКА: %v", mediaID, err)
		}
		return "", "", fmt.Errorf("помилка завантаження yt-dlp: %w", err)
	}

	if err := os.Rename(tmpActualPath, outPath); err != nil {
		_ = os.Remove(tmpActualPath)
		log.Printf("[yt-dlp %s] помилка збереження: %v", mediaID, err)
		return "", "", fmt.Errorf("помилка збереження відео: %w", err)
	}

	log.Printf("[yt-dlp %s] готово → %s", mediaID, outPath)
	return outPath, mediaID, nil
}
