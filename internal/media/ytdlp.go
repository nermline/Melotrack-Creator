package media

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// YtDlp runs the yt-dlp binary. It is invoked directly (not through a Go wrapper
// that pins an old version) so it can be updated independently: YouTube breaks
// old yt-dlp releases regularly.
type YtDlp struct {
	Path    string
	Proxy   string
	Cookies string
	Extra   []string
}

// DownloadError carries a machine-readable code and a message for organisers.
type DownloadError struct {
	Code    string
	Message string
	Detail  string
}

func (e *DownloadError) Error() string { return e.Message + ": " + e.Detail }

type DownloadResult struct {
	File  string
	Title string
}

func (y *YtDlp) args(id, outDir string) []string {
	args := []string{
		"--no-playlist",
		"--newline",
		"--no-mtime",
		// Prefer H.264/AAC up to 1080p: plays in every browser and needs no re-encode for preview.
		"-f", "bv*[height<=1080]+ba/b[height<=1080]/bv*+ba/b",
		"-S", "vcodec:h264,acodec:aac",
		"--merge-output-format", "mp4",
		"--remux-video", "mp4",
		"-o", filepath.Join(outDir, "%(id)s.%(ext)s"),
		"--progress",
		"--progress-template", "download:MTPROG %(progress.downloaded_bytes)s %(progress.total_bytes)s %(progress.total_bytes_estimate)s",
		"--print", "after_move:MTFILE %(filepath)s",
		"--print", "after_move:MTTITLE %(title)s",
	}
	if y.Proxy != "" {
		args = append(args, "--proxy", y.Proxy)
	}
	if y.Cookies != "" {
		if _, err := os.Stat(y.Cookies); err == nil {
			args = append(args, "--cookies", y.Cookies)
		} else {
			slog.Warn("YTDLP_COOKIES file not found, downloading without cookies", "path", y.Cookies)
		}
	}
	args = append(args, y.Extra...)
	return append(args, "--", WatchURL(id))
}

// Download fetches the video into outDir. onProgress receives 0..1 per downloaded stream.
func (y *YtDlp) Download(ctx context.Context, id, outDir string, onProgress func(float64)) (DownloadResult, error) {
	cmd := exec.CommandContext(ctx, y.Path, y.args(id, outDir)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return DownloadResult{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return DownloadResult{}, err
	}
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
			return DownloadResult{}, &DownloadError{Code: "ytdlp_missing", Message: "yt-dlp не встановлено на сервері", Detail: err.Error()}
		}
		return DownloadResult{}, err
	}

	var res DownloadResult
	var tail tailBuffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			tail.add(line)
			slog.Debug("yt-dlp", "id", id, "line", line)
		}
	}()
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MTPROG "):
			if f, ok := parseProgress(strings.TrimPrefix(line, "MTPROG ")); ok && onProgress != nil {
				onProgress(f)
			}
		case strings.HasPrefix(line, "MTFILE "):
			res.File = strings.TrimPrefix(line, "MTFILE ")
		case strings.HasPrefix(line, "MTTITLE "):
			res.Title = strings.TrimPrefix(line, "MTTITLE ")
		default:
			tail.add(line)
		}
	}
	_, _ = io.Copy(io.Discard, stdout)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return DownloadResult{}, ctx.Err()
		}
		return DownloadResult{}, classify(tail.String(), err)
	}
	if res.File == "" || !fileExists(res.File) {
		return DownloadResult{}, &DownloadError{Code: "failed", Message: "yt-dlp завершився без файлу", Detail: tail.String()}
	}
	return res, nil
}

func parseProgress(s string) (float64, bool) {
	f := strings.Fields(s)
	if len(f) != 3 {
		return 0, false
	}
	done, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, false
	}
	total, err := strconv.ParseFloat(f[1], 64)
	if err != nil || total <= 0 {
		total, err = strconv.ParseFloat(f[2], 64)
		if err != nil || total <= 0 {
			return 0, false
		}
	}
	return min(done/total, 1), true
}

// classify maps yt-dlp's stderr to an explanation an organiser can act on.
func classify(stderr string, err error) *DownloadError {
	s := strings.ToLower(stderr)
	last := lastErrorLine(stderr)
	if last == "" {
		last = err.Error()
	}
	switch {
	case strings.Contains(s, "not a bot"):
		return &DownloadError{Code: "bot_check", Detail: last,
			Message: "YouTube блокує завантаження з цього сервера (перевірка «я не бот»). Налаштуйте cookies або проксі, або завантажте файл вручну"}
	case strings.Contains(s, "confirm your age") || strings.Contains(s, "age-restricted") || strings.Contains(s, "inappropriate for some users"):
		return &DownloadError{Code: "age_restricted", Detail: last,
			Message: "Відео з віковим обмеженням: потрібні cookies акаунта YouTube або ручне завантаження файлу"}
	case strings.Contains(s, "private video") || strings.Contains(s, "video unavailable") ||
		strings.Contains(s, "has been removed") || strings.Contains(s, "not available in your country") ||
		strings.Contains(s, "members-only"):
		return &DownloadError{Code: "unavailable", Detail: last,
			Message: "Відео недоступне (приватне, видалене або заблоковане в регіоні сервера)"}
	case strings.Contains(s, "http error 429") || strings.Contains(s, "too many requests"):
		return &DownloadError{Code: "rate_limited", Detail: last,
			Message: "YouTube тимчасово обмежив запити з сервера. Спробуйте пізніше або через проксі"}
	case strings.Contains(s, "javascript runtime") || strings.Contains(s, "challenge solv") || strings.Contains(s, "yt-dlp-ejs") || strings.Contains(s, "remote-components"):
		return &DownloadError{Code: "js_runtime", Detail: last,
			Message: "yt-dlp потребує JavaScript-рушій (deno) для YouTube. Встановіть deno або скористайтесь Docker-образом"}
	case strings.Contains(s, "http error 403"):
		return &DownloadError{Code: "forbidden", Detail: last,
			Message: "YouTube відхилив запит (403). Найчастіше допомагає оновлення yt-dlp; на VPS — cookies або проксі"}
	case strings.Contains(s, "requested format is not available"):
		return &DownloadError{Code: "format", Detail: last,
			Message: "YouTube не віддав жодного відео-формату. Часто допомагає оновлення yt-dlp"}
	}
	return &DownloadError{Code: "failed", Message: "Не вдалося завантажити відео", Detail: last}
}

func lastErrorLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "ERROR:") {
			return strings.TrimSpace(strings.TrimPrefix(lines[i], "ERROR:"))
		}
	}
	if len(lines) > 0 {
		return lines[len(lines)-1]
	}
	return ""
}

// tailBuffer keeps the last lines of process output for error reporting.
type tailBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (t *tailBuffer) add(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lines = append(t.lines, line)
	if len(t.lines) > 40 {
		t.lines = t.lines[len(t.lines)-40:]
	}
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.Join(t.lines, "\n")
}

var versionCache struct {
	sync.Mutex
	value string
	at    time.Time
}

// Version returns `yt-dlp --version` and how many days old that release is
// (-1 when unknown). YouTube breaks old releases within weeks. The answer is
// cached for a few minutes because the show tab polls it.
func (y *YtDlp) Version(ctx context.Context) (string, int) {
	versionCache.Lock()
	defer versionCache.Unlock()
	if versionCache.value == "" || time.Since(versionCache.at) > 5*time.Minute {
		out, err := exec.CommandContext(ctx, y.Path, "--version").Output()
		if err != nil {
			return fmt.Sprintf("недоступний (%v)", err), -1
		}
		versionCache.value, versionCache.at = strings.TrimSpace(string(out)), time.Now()
	}
	v := versionCache.value
	if len(v) >= 10 {
		if t, err := time.Parse("2006.01.02", v[:10]); err == nil {
			return v, int(time.Since(t).Hours() / 24)
		}
	}
	return v, -1
}
