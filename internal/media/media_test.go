package media

import (
	"slices"
	"strings"
	"testing"
)

func TestParseYouTubeID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":               "dQw4w9WgXcQ",
		"https://youtube.com/watch?v=dQw4w9WgXcQ&list=PLx&index=3":  "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?feature=share&v=dQw4w9WgXcQ": "dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ":                 "dQw4w9WgXcQ",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ&si=abc":      "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?t=42":                         "dQw4w9WgXcQ",
		"youtu.be/dQw4w9WgXcQ":                                      "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":                "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ?start=10":        "dQw4w9WgXcQ",
		"https://www.youtube.com/live/dQw4w9WgXcQ?feature=share":    "dQw4w9WgXcQ",
		"https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ":        "dQw4w9WgXcQ",
		"  dQw4w9WgXcQ  ":                             "dQw4w9WgXcQ",
		"https://vimeo.com/123456":                    "",
		"https://youtu.be/abc":                        "",
		"https://www.youtube.com/playlist?list=PL123": "",
		"https://evil.example/watch?v=dQw4w9WgXcQ":    "",
		"":               "",
		"just some text": "",
	}
	for in, want := range cases {
		if got := ParseYouTubeID(in); got != want {
			t.Errorf("ParseYouTubeID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSuggestAnswer(t *testing.T) {
	cases := []struct{ title, channel, want string }{
		{"Queen - Bohemian Rhapsody (Official Video Remastered)", "Queen Official", "Queen — Bohemian Rhapsody"},
		{"Океан Ельзи – Обійми [Official Music Video]", "Okean Elzy", "Океан Ельзи — Обійми"},
		{"KAZKA — ПЛАКАЛА (ПРЕМ'ЄРА КЛІПУ)", "KAZKA", "KAZKA — ПЛАКАЛА"},
		{"Bohemian Rhapsody (Remastered 2011)", "Queen - Topic", "Queen — Bohemian Rhapsody"},
		{"Stardew Valley OST - Overture", "ConcernedApe", "Stardew Valley OST — Overture"},
		{"Daft Punk - Get Lucky | Official Audio", "Daft Punk", "Daft Punk — Get Lucky"},
		{"Minecraft Volume Alpha", "C418", "Minecraft Volume Alpha"},
		{"(Official Video)", "X", "(Official Video)"},
	}
	for _, c := range cases {
		if got := SuggestAnswer(c.title, c.channel); got != c.want {
			t.Errorf("SuggestAnswer(%q, %q) = %q, want %q", c.title, c.channel, got, c.want)
		}
	}
}

func TestNormalizeCrop(t *testing.T) {
	x, y, w, h, ok := NormalizeCrop(1901, 3, 999, 563, 1920, 1080)
	if !ok || w%2 != 0 || h%2 != 0 || x%2 != 0 || y%2 != 0 || x+w > 1920 || y+h > 1080 {
		t.Fatalf("got %d,%d %dx%d ok=%v", x, y, w, h, ok)
	}
	if _, _, _, _, ok := NormalizeCrop(0, 0, 4, 4, 1920, 1080); ok {
		t.Fatal("tiny crop accepted")
	}
	if _, _, _, _, ok := NormalizeCrop(0, 0, 100, 100, 0, 0); ok {
		t.Fatal("crop without source size accepted")
	}
}

func arg(args []string, flag string) string {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return ""
	}
	return args[i+1]
}

func TestClipArgs(t *testing.T) {
	spec := ClipSpec{Source: "in.mp4", Out: "out.mp4", Start: 30, End: 45, GainDB: -3,
		Frame: "crop", CropX: 101, CropY: 0, CropW: 1281, CropH: 721, SrcW: 1920, SrcH: 1080,
		HasVideo: true, HasAudio: true, Height: 720}
	a := ClipArgs(spec)
	if arg(a, "-ss") != "30.000" || arg(a, "-t") != "15.000" {
		t.Fatalf("trim wrong: %v", a)
	}
	vf := arg(a, "-vf")
	if !strings.HasPrefix(vf, "crop=1280:720:100:0,") || !strings.Contains(vf, "scale=1280:720") {
		t.Fatalf("vf = %s", vf)
	}
	af := arg(a, "-af")
	for _, want := range []string{"loudnorm", "volume=-3.00dB", "afade=t=out:st=13.500:d=1.500"} {
		if !strings.Contains(af, want) {
			t.Fatalf("af %q lacks %q", af, want)
		}
	}
	if arg(a, "-movflags") != "+faststart" {
		t.Fatal("clips must be faststart for instant playback")
	}

	fit := spec
	fit.Frame = "fit"
	if strings.Contains(arg(ClipArgs(fit), "-vf"), "crop=") {
		t.Fatal("fit mode must not crop")
	}

	audioOnly := spec
	audioOnly.HasVideo = false
	ao := ClipArgs(audioOnly)
	if !strings.Contains(strings.Join(ao, " "), "color=c=black") || arg(ao, "-map") != "1:v:0" {
		t.Fatalf("audio-only source needs a generated picture: %v", ao)
	}
}

func TestClipKeyChangesWithSettings(t *testing.T) {
	base := ClipSpec{Start: 1, End: 16, Frame: "fit", Height: 720}
	k := ClipKey(1, 1, base)
	moved := base
	moved.Start = 2
	if ClipKey(1, 1, moved) == k {
		t.Fatal("key must change with start")
	}
	if ClipKey(1, 2, base) == k {
		t.Fatal("key must change with a new source version")
	}
	other := base
	other.Source, other.Out = "elsewhere.mp4", "x.mp4"
	if ClipKey(1, 1, other) != k {
		t.Fatal("paths must not affect the key")
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"ERROR: [youtube] abc: Sign in to confirm you’re not a bot. Use --cookies": "bot_check",
		"ERROR: [youtube] abc: Private video. Sign in if you've been granted":      "unavailable",
		"ERROR: [youtube] abc: Sign in to confirm your age":                        "age_restricted",
		"ERROR: unable to download video data: HTTP Error 429: Too Many Requests":  "rate_limited",
		"ERROR: unable to download video data: HTTP Error 403: Forbidden":          "forbidden",
		"ERROR: something odd": "failed",
	}
	for stderr, want := range cases {
		if got := classify(stderr, nil).Code; got != want {
			t.Errorf("classify(%q) = %s, want %s", stderr, got, want)
		}
	}
}

func TestParseProgress(t *testing.T) {
	if f, ok := parseProgress("50 100 NA"); !ok || f != 0.5 {
		t.Fatalf("got %v %v", f, ok)
	}
	if f, ok := parseProgress("25 NA 100"); !ok || f != 0.25 {
		t.Fatalf("estimate fallback: got %v %v", f, ok)
	}
	if _, ok := parseProgress("NA NA NA"); ok {
		t.Fatal("unknown totals must be ignored")
	}
}

// legacyKey is ClipKey(1, 1, {Start 1, End 16, fit, 720p}) as computed by the
// release before reverse existed.
const legacyKey = "40c830648213"

func TestClipKeyIsStableForExistingClips(t *testing.T) {
	base := ClipSpec{Start: 1, End: 16, Frame: "fit", Height: 720}
	// Computed with the previous release: a non-reversed clip keeps its key,
	// so upgrading does not re-cut every song.
	if got := ClipKey(1, 1, base); got != legacyKey {
		t.Fatalf("key changed: %s, want %s", got, legacyKey)
	}
	rev := base
	rev.Reverse = true
	if ClipKey(1, 1, rev) == ClipKey(1, 1, base) {
		t.Fatal("reversing must change the key")
	}
}

func TestClipArgsReverse(t *testing.T) {
	spec := ClipSpec{Source: "in.mp4", Out: "out.mp4", Start: 10, End: 25, Frame: "fit",
		HasVideo: true, HasAudio: true, Height: 720, Reverse: true}
	a := ClipArgs(spec)
	if !strings.HasPrefix(arg(a, "-af"), "areverse,loudnorm") {
		t.Fatalf("af = %s", arg(a, "-af"))
	}
	// The input itself is limited to the fragment, so areverse never buffers the rest of the song.
	i := slices.Index(a, "-i")
	if !slices.Contains(a[:i], "-t") {
		t.Fatalf("-t must come before -i: %v", a)
	}
	spec.Reverse = false
	if strings.Contains(arg(ClipArgs(spec), "-af"), "areverse") {
		t.Fatal("areverse only when asked")
	}
}

func TestParseEBUR128(t *testing.T) {
	in := `  Duration: 00:00:03.00
[Parsed_ebur128_0 @ 0x1] t: 0.0999773  TARGET:-23 LUFS    M:-120.7 S:-120.7     I: -70.0 LUFS       LRA:   0.0 LU
[Parsed_ebur128_0 @ 0x1] t: 0.499977   TARGET:-23 LUFS    M:-120.7 S:-120.7     I: -70.0 LUFS       LRA:   0.0 LU
[Parsed_ebur128_0 @ 0x1] t: 1.199977   TARGET:-23 LUFS    M: -20.0 S: -20.0     I: -20.0 LUFS       LRA:   0.0 LU
[Parsed_ebur128_0 @ 0x1] t: 1.499977   TARGET:-23 LUFS    M: -20.0 S: -20.0     I: -20.0 LUFS       LRA:   0.0 LU
[Parsed_ebur128_0 @ 0x1] t: 2.099977   TARGET:-23 LUFS    M: -10.0 S: -12.0     I: -14.0 LUFS       LRA:   1.0 LU
  Summary:
    I:         -14.0 LUFS`
	got := parseEBUR128(strings.NewReader(in))
	want := []float64{-70, -20, -10}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
