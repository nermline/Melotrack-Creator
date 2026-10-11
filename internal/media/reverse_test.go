package media

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// A tone and a white picture at the start of the fragment must end up at the
// end of a reversed clip. Six seconds span two reversed pieces.
func TestReversedClipPlaysBackwards(t *testing.T) {
	ffPath, err1 := exec.LookPath("ffmpeg")
	fpPath, err2 := exec.LookPath("ffprobe")
	if err1 != nil || err2 != nil {
		t.Skip("ffmpeg not installed")
	}
	ff := &FFmpeg{FFmpeg: ffPath, FFprobe: fpPath}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")
	// 2 s of white picture with a tone, then 4 s of black and silence.
	mk := exec.Command(ffPath, "-v", "error",
		"-f", "lavfi", "-i", "color=white:s=320x180:r=25:d=2",
		"-f", "lavfi", "-i", "color=black:s=320x180:r=25:d=4",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2,apad=whole_dur=6",
		"-filter_complex", "[0:v][1:v]concat=n=2:v=1:a=0[v]",
		"-map", "[v]", "-map", "2:a",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-t", "6", src)
	if b, err := mk.CombinedOutput(); err != nil {
		t.Skipf("cannot make test video: %v %s", err, b)
	}

	brightness := func(path string, at float64) float64 {
		out, err := exec.Command(ffPath, "-v", "error", "-ss", strconv.FormatFloat(at, 'f', 2, 64), "-i", path,
			"-frames:v", "1", "-vf", "signalstats,metadata=print:key=lavfi.signalstats.YAVG:file=-", "-f", "null", "-").Output()
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(`YAVG=([0-9.]+)`).FindSubmatch(out)
		if m == nil {
			t.Fatalf("no brightness in %q", out)
		}
		v, _ := strconv.ParseFloat(string(m[1]), 64)
		return v
	}
	render := func(name string, reverse bool) (string, []float64) {
		out := filepath.Join(dir, name)
		spec := ClipSpec{Source: src, Out: out, Start: 0, End: 6, Frame: "fit", SrcW: 320, SrcH: 180,
			HasVideo: true, HasAudio: true, Height: 180, Reverse: reverse}
		if err := ff.RenderClip(context.Background(), spec, nil); err != nil {
			t.Fatal(err)
		}
		info, err := ff.Probe(context.Background(), out)
		if err != nil || info.Duration < 5.8 || info.Duration > 6.3 {
			t.Fatalf("%s lasts %.2fs (%v), want 6s", name, info.Duration, err)
		}
		m, err := ff.LoudnessMap(context.Background(), out)
		if err != nil || len(m) < 6 {
			t.Fatalf("loudness map %v, %v", m, err)
		}
		return out, m
	}

	fwd, loud := render("forward.mp4", false)
	if loud[0] < -40 || loud[4] > -50 {
		t.Fatalf("forward clip: tone should open it, got %v", loud)
	}
	if brightness(fwd, 0.5) < 128 || brightness(fwd, 5.5) > 128 {
		t.Fatal("forward clip: white picture should open it")
	}

	rev, loud := render("reversed.mp4", true)
	if loud[0] > -50 || loud[4] < -40 {
		t.Fatalf("reversed clip: tone should close it, got %v", loud)
	}
	// Mirror of the source: black until 4 s, white after (checked across the
	// boundary of the two reversed pieces at 3 s).
	for at, white := range map[float64]bool{0.5: false, 3.5: false, 4.5: true, 5.5: true} {
		if got := brightness(rev, at) > 128; got != white {
			t.Fatalf("reversed clip at %.1fs: white = %v, want %v", at, got, white)
		}
	}
}
