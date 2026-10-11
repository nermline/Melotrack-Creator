package worker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/media"
)

const testToken = "test-token-0123456789"

func needFFmpeg(t *testing.T) *media.FFmpeg {
	t.Helper()
	ff, err1 := exec.LookPath("ffmpeg")
	fp, err2 := exec.LookPath("ffprobe")
	if err1 != nil || err2 != nil {
		t.Skip("ffmpeg not installed")
	}
	return &media.FFmpeg{FFmpeg: ff, FFprobe: fp}
}

// testVideo makes a short clip with a picture and a tone.
func testVideo(t *testing.T, ff *media.FFmpeg) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "src.mp4")
	cmd := exec.Command(ff.FFmpeg, "-v", "error",
		"-f", "lavfi", "-i", "testsrc=duration=4:size=320x180:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=4",
		"-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot make a test video: %v %s", err, b)
	}
	return out
}

// fakeYtDlp writes a script that "downloads" src the way yt-dlp reports it.
func fakeYtDlp(t *testing.T, src string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "yt-dlp")
	body := `#!/bin/sh
if [ "$1" = "--version" ]; then echo 2026.01.01; exit 0; fi
out=""
while [ $# -gt 0 ]; do
  if [ "$1" = "-o" ]; then out="$2"; fi
  shift
done
dir=$(dirname "$out")
cp "` + src + `" "$dir/abcdefghijk.mp4"
echo "MTPROG 50 100 NA"
echo "MTPROG 100 100 NA"
echo "MTFILE $dir/abcdefghijk.mp4"
echo "MTTITLE Test song"
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func newWorker(t *testing.T, yt *media.YtDlp, ff *media.FFmpeg) (*Server, *httptest.Server) {
	t.Helper()
	s, err := NewServer(Options{Name: "test", Token: testToken, Dir: t.TempDir(), YtDlp: yt, FFmpeg: ff, CacheBytes: 1 << 30})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

func TestDownloadAndRenderThroughWorker(t *testing.T) {
	ff := needFFmpeg(t)
	src := testVideo(t, ff)
	_, ts := newWorker(t, &media.YtDlp{Path: fakeYtDlp(t, src)}, ff)
	c := NewClient(ts.URL, testToken)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	h, err := c.Health(ctx)
	if err != nil || !h.OK || h.YtDlpVersion != "2026.01.01" {
		t.Fatalf("health = %+v, %v", h, err)
	}

	var last float64
	res, err := c.Download(ctx, "abcdefghijk", t.TempDir(), func(p float64) { last = p })
	if err != nil {
		t.Fatal(err)
	}
	want, _ := media.HashFile(src)
	if res.Hash != want || res.Title != "Test song" || last != 1 {
		t.Fatalf("download = %+v (progress %v), want hash %s", res, last, want)
	}
	if len(res.Loudness) < 3 {
		t.Fatalf("loudness map has %d seconds, want ~4", len(res.Loudness))
	}
	if got, _ := media.HashFile(res.File); got != want {
		t.Fatal("downloaded file differs from the source")
	}

	out := filepath.Join(t.TempDir(), "clip.mp4")
	spec := media.ClipSpec{Out: out, Start: 1, End: 3, Frame: "fit", SrcW: 320, SrcH: 180,
		HasVideo: true, HasAudio: true, Height: 180, Reverse: true}
	if err := c.Render(ctx, media.SourceRef{Path: res.File, Hash: res.Hash}, spec, nil); err != nil {
		t.Fatal(err)
	}
	info, err := ff.Probe(ctx, out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Duration < 1.8 || info.Duration > 2.3 {
		t.Fatalf("clip lasts %.2fs, want 2s", info.Duration)
	}
}

// A source the worker has never seen is uploaded before rendering.
func TestRenderUploadsMissingSource(t *testing.T) {
	ff := needFFmpeg(t)
	src := testVideo(t, ff)
	s, ts := newWorker(t, &media.YtDlp{Path: "/nonexistent"}, ff)
	c := NewClient(ts.URL, testToken)
	hash, _ := media.HashFile(src)

	out := filepath.Join(t.TempDir(), "clip.mp4")
	spec := media.ClipSpec{Out: out, Start: 0, End: 2, Frame: "fit", SrcW: 320, SrcH: 180,
		HasVideo: true, HasAudio: true, Height: 180}
	if err := c.Render(context.Background(), media.SourceRef{Path: src, Hash: hash}, spec, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.cachePath(hash)); err != nil {
		t.Fatal("uploaded source is not cached")
	}
	if entries, _ := os.ReadDir(filepath.Join(s.opt.Dir, "results")); len(entries) != 0 {
		t.Fatalf("collected results are left on the worker: %d", len(entries))
	}

	// A wrong hash is refused.
	err := c.ensureSource(context.Background(), media.SourceRef{Path: src, Hash: strings.Repeat("0", 64)})
	if err == nil {
		t.Fatal("upload with a wrong hash accepted")
	}
}

func TestWorkerUnavailableIsRetryable(t *testing.T) {
	ts := httptest.NewServer(nil)
	url := ts.URL
	ts.Close() // nothing listens there any more

	c := NewClient(url, testToken)
	_, err := c.Download(context.Background(), "abcdefghijk", t.TempDir(), nil)
	if !errors.Is(err, media.ErrWorkerUnavailable) {
		t.Fatalf("err = %v, want ErrWorkerUnavailable", err)
	}
}

func TestWrongTokenIsNotRetried(t *testing.T) {
	_, ts := newWorker(t, &media.YtDlp{Path: "/nonexistent"}, &media.FFmpeg{})
	if resp, err := http.Get(ts.URL + "/healthz"); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz needs no token: %v", err)
	}
	c := NewClient(ts.URL, "another-token-0123456789")
	_, err := c.Download(context.Background(), "abcdefghijk", t.TempDir(), nil)
	if err == nil || errors.Is(err, media.ErrWorkerUnavailable) || !strings.Contains(err.Error(), "WORKER_TOKEN") {
		t.Fatalf("err = %v, want a token error", err)
	}
}

// yt-dlp failures reach the server with their explanation.
func TestDownloadErrorIsPassedOn(t *testing.T) {
	script := filepath.Join(t.TempDir(), "yt-dlp")
	os.WriteFile(script, []byte("#!/bin/sh\necho 'ERROR: [youtube] abcdefghijk: Private video' >&2\nexit 1\n"), 0o755)
	_, ts := newWorker(t, &media.YtDlp{Path: script}, &media.FFmpeg{})
	c := NewClient(ts.URL, testToken)
	_, err := c.Download(context.Background(), "abcdefghijk", t.TempDir(), nil)
	var de *media.DownloadError
	if !errors.As(err, &de) || de.Code != "unavailable" {
		t.Fatalf("err = %v, want an unavailable DownloadError", err)
	}
}
