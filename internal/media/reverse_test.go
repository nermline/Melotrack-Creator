package media

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

// A tone at the start of the fragment must end up at the end of a reversed clip.
func TestReversedClipPlaysBackwards(t *testing.T) {
	ffPath, err1 := exec.LookPath("ffmpeg")
	fpPath, err2 := exec.LookPath("ffprobe")
	if err1 != nil || err2 != nil {
		t.Skip("ffmpeg not installed")
	}
	ff := &FFmpeg{FFmpeg: ffPath, FFprobe: fpPath}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	// 2 s of tone, then 4 s of silence.
	mk := exec.Command(ffPath, "-v", "error",
		"-f", "lavfi", "-i", "testsrc=duration=6:size=320x180:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2,apad=whole_dur=6",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-t", "6", src)
	if b, err := mk.CombinedOutput(); err != nil {
		t.Skipf("cannot make test video: %v %s", err, b)
	}

	loud := func(reverse bool) []float64 {
		out := filepath.Join(dir, "clip.mp4")
		spec := ClipSpec{Source: src, Out: out, Start: 0, End: 6, Frame: "fit", SrcW: 320, SrcH: 180,
			HasVideo: true, HasAudio: true, Height: 180, Reverse: reverse}
		if err := ff.Run(context.Background(), ClipArgs(spec), 6, nil); err != nil {
			t.Fatal(err)
		}
		m, err := ff.LoudnessMap(context.Background(), out)
		if err != nil || len(m) < 6 {
			t.Fatalf("loudness map %v, %v", m, err)
		}
		return m
	}

	fwd := loud(false)
	if fwd[0] < -40 || fwd[4] > -50 {
		t.Fatalf("forward clip: tone should open it, got %v", fwd)
	}
	rev := loud(true)
	if rev[0] > -50 || rev[4] < -40 {
		t.Fatalf("reversed clip: tone should close it, got %v", rev)
	}
}
