package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

// Downloading and cutting can run on this server or be sent to a remote
// worker (see the worker package). The manager only talks to these interfaces.

// Downloader fetches a YouTube video into dir, a fresh directory on this server.
type Downloader interface {
	Download(ctx context.Context, youtubeID, dir string, onProgress func(float64)) (DownloadResult, error)
}

// Renderer cuts the clip described by spec from src into spec.Out.
type Renderer interface {
	Render(ctx context.Context, src SourceRef, spec ClipSpec, onProgress func(float64)) error
}

// SourceRef points at a source video on this server. Hash lets a remote
// worker reuse its own copy instead of receiving the file again.
type SourceRef struct {
	Path string
	Hash string
}

// ErrWorkerUnavailable means the remote worker could not be reached; the job
// is retried later instead of being marked as failed.
var ErrWorkerUnavailable = errors.New("worker unavailable")

// LocalDownloader runs yt-dlp on this server.
type LocalDownloader struct{ YT *YtDlp }

func (d LocalDownloader) Download(ctx context.Context, id, dir string, onProgress func(float64)) (DownloadResult, error) {
	return d.YT.Download(ctx, id, dir, onProgress)
}

// LocalRenderer runs ffmpeg on this server.
type LocalRenderer struct{ FF *FFmpeg }

func (r LocalRenderer) Render(ctx context.Context, src SourceRef, spec ClipSpec, onProgress func(float64)) error {
	spec.Source = src.Path
	return r.FF.RenderClip(ctx, spec, onProgress)
}

// HashFile returns the hex sha256 of a file.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
