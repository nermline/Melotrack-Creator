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

// DownloadRawVideo downloads a YouTube video and saves it to disk.
// The download respects ctx cancellation — if ctx is cancelled, yt-dlp is interrupted.
//
// yt-dlp always appends the container extension to the Output() path, so we pass
// a base name without ".mp4". The actual file yt-dlp writes is tmpBase+".mp4",
// which we then rename to the final outPath.
func DownloadRawVideo(ctx context.Context, youtubeURL string) (string, string, error) {
	if err := os.MkdirAll("./downloads/raw", os.ModePerm); err != nil {
		return "", "", fmt.Errorf("не вдалося створити директорію: %w", err)
	}

	mediaID := ExtractYouTubeID(youtubeURL)

	outPath := fmt.Sprintf("./downloads/raw/%s.mp4", mediaID)

	// yt-dlp appends the output format extension to whatever path we give it.
	// By passing a path without ".mp4", the file it actually creates will be
	// tmpBase + ".mp4", i.e. exactly tmpActualPath below.
	tmpBase := fmt.Sprintf("./downloads/raw/%s.tmp", mediaID)
	tmpActualPath := tmpBase + ".mp4"

	_ = os.Remove(tmpActualPath)

	formatFilter := "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best[height<=1080]"

	dl := ytdlp.New().
		Format(formatFilter).
		MergeOutputFormat("mp4").
		Output(tmpBase)

	res, err := dl.Run(ctx, youtubeURL)
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
