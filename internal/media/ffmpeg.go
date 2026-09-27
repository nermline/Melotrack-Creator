package media

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type FFmpeg struct {
	FFmpeg  string
	FFprobe string
}

type ProbeInfo struct {
	Width, Height      int
	Duration           float64
	HasVideo, HasAudio bool
	VideoCodec         string
	AudioCodec         string
	Format             string
}

func (f *FFmpeg) Probe(ctx context.Context, path string) (ProbeInfo, error) {
	out, err := exec.CommandContext(ctx, f.FFprobe, "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		return ProbeInfo{}, fmt.Errorf("ffprobe: %w", err)
	}
	var raw struct {
		Streams []struct {
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
		Format struct {
			Duration   string `json:"duration"`
			FormatName string `json:"format_name"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return ProbeInfo{}, fmt.Errorf("ffprobe: %w", err)
	}
	var p ProbeInfo
	p.Format = raw.Format.FormatName
	p.Duration, _ = strconv.ParseFloat(raw.Format.Duration, 64)
	for _, s := range raw.Streams {
		switch {
		case s.CodecType == "video" && s.Disposition.AttachedPic == 0 && !p.HasVideo:
			p.HasVideo, p.Width, p.Height, p.VideoCodec = true, s.Width, s.Height, s.CodecName
		case s.CodecType == "audio" && !p.HasAudio:
			p.HasAudio, p.AudioCodec = true, s.CodecName
		}
	}
	if !p.HasVideo && !p.HasAudio {
		return p, fmt.Errorf("файл не містить ні відео, ні аудіо")
	}
	return p, nil
}

// ClipSpec describes one clip render.
type ClipSpec struct {
	Source   string
	Out      string
	Start    float64
	End      float64
	GainDB   float64
	Frame    string // "fit" or "crop"
	CropX    int
	CropY    int
	CropW    int
	CropH    int
	SrcW     int
	SrcH     int
	HasVideo bool
	HasAudio bool
	Height   int // output height; width is 16:9
}

// renderVersion is part of every clip key: bump it when the ffmpeg recipe changes
// and every clip is re-rendered automatically on the next start.
const renderVersion = "r1"

// ClipKey identifies the rendered output of a given source + settings.
func ClipKey(mediaID uint, mediaVersion int, s ClipSpec) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%d.%d|%.3f|%.3f|%.2f|%s|%d,%d,%d,%d|%d",
		renderVersion, mediaID, mediaVersion, s.Start, s.End, s.GainDB, s.Frame,
		s.CropX, s.CropY, s.CropW, s.CropH, s.Height)
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func even(v int) int { return v &^ 1 }

// NormalizeCrop clamps the crop rectangle into the frame with even values
// (libx264 rejects odd dimensions for yuv420p).
func NormalizeCrop(x, y, w, h, srcW, srcH int) (int, int, int, int, bool) {
	if srcW <= 0 || srcH <= 0 || w <= 0 || h <= 0 {
		return 0, 0, 0, 0, false
	}
	x, y = max(x, 0), max(y, 0)
	w, h = min(w, srcW), min(h, srcH)
	if x+w > srcW {
		x = srcW - w
	}
	if y+h > srcH {
		y = srcH - h
	}
	x, y, w, h = even(x), even(y), even(w), even(h)
	if w < 16 || h < 16 {
		return 0, 0, 0, 0, false
	}
	return x, y, w, h, true
}

func fmtSec(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

// ClipArgs builds the ffmpeg command line for a clip: trim, frame to 16:9,
// loudness-normalise (so every song plays equally loud), apply the editor's
// gain and fade the edges so clips never start or stop with a click.
func ClipArgs(s ClipSpec) []string {
	dur := max(s.End-s.Start, 0.5)
	outH := s.Height
	outW := even(outH * 16 / 9)

	args := []string{"-hide_banner", "-nostdin", "-y", "-loglevel", "error",
		"-ss", fmtSec(s.Start), "-i", s.Source}
	videoIn, audioIn := "0:v:0", "0:a:0"
	extra := 1
	if !s.HasVideo {
		args = append(args, "-f", "lavfi", "-i", fmt.Sprintf("color=c=black:s=%dx%d:r=25", outW, outH))
		videoIn = fmt.Sprintf("%d:v:0", extra)
		extra++
	}
	if !s.HasAudio {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo")
		audioIn = fmt.Sprintf("%d:a:0", extra)
	}

	var vf []string
	if s.HasVideo && s.Frame == "crop" {
		if x, y, w, h, ok := NormalizeCrop(s.CropX, s.CropY, s.CropW, s.CropH, s.SrcW, s.SrcH); ok {
			vf = append(vf, fmt.Sprintf("crop=%d:%d:%d:%d", w, h, x, y))
		}
	}
	vf = append(vf,
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", outW, outH),
		fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black", outW, outH),
		"setsar=1", "format=yuv420p")

	fadeIn := min(0.4, dur/4)
	fadeOut := min(1.5, dur/3)
	af := []string{
		"loudnorm=I=-16:TP=-1.5:LRA=11",
		"aresample=48000",
		fmt.Sprintf("volume=%sdB", strconv.FormatFloat(s.GainDB, 'f', 2, 64)),
		fmt.Sprintf("afade=t=in:st=0:d=%s", fmtSec(fadeIn)),
		fmt.Sprintf("afade=t=out:st=%s:d=%s", fmtSec(dur-fadeOut), fmtSec(fadeOut)),
	}

	args = append(args,
		"-map", videoIn, "-map", audioIn,
		"-t", fmtSec(dur),
		"-vf", strings.Join(vf, ","),
		"-af", strings.Join(af, ","),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "21", "-profile:v", "high",
		"-c:a", "aac", "-b:a", "192k", "-ac", "2",
		"-movflags", "+faststart",
		"-progress", "pipe:1", "-nostats",
		"-f", "mp4", s.Out,
	)
	return args
}

// Run executes ffmpeg, reporting progress (0..1) against the expected output duration.
func (f *FFmpeg) Run(ctx context.Context, args []string, expected float64, onProgress func(float64)) error {
	cmd := exec.CommandContext(ctx, f.FFmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var tail tailBuffer
	cmd.Stderr = writerFunc(func(p []byte) (int, error) {
		for _, l := range strings.Split(strings.TrimSpace(string(p)), "\n") {
			tail.add(l)
		}
		return len(p), nil
	})
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok || (k != "out_time_us" && k != "out_time_ms") || onProgress == nil || expected <= 0 {
			continue
		}
		if us, err := strconv.ParseFloat(v, 64); err == nil {
			onProgress(min(max(us/1e6/expected, 0), 1))
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg: %w: %s", err, lastLines(tail.String(), 3))
	}
	return nil
}

// NormalizeArgs converts an uploaded file into a browser-friendly MP4. When the
// codecs already fit, the streams are copied instead of re-encoded.
func NormalizeArgs(in, out string, p ProbeInfo) []string {
	args := []string{"-hide_banner", "-nostdin", "-y", "-loglevel", "error", "-i", in}
	copyVideo := !p.HasVideo || p.VideoCodec == "h264"
	copyAudio := !p.HasAudio || p.AudioCodec == "aac"
	if p.HasVideo {
		args = append(args, "-map", "0:v:0")
		if copyVideo {
			args = append(args, "-c:v", "copy")
		} else {
			args = append(args, "-vf", "scale=-2:'min(1080,ih)',format=yuv420p", "-c:v", "libx264", "-preset", "veryfast", "-crf", "20")
		}
	}
	if p.HasAudio {
		args = append(args, "-map", "0:a:0")
		if copyAudio {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac", "-b:a", "192k")
		}
	}
	return append(args, "-movflags", "+faststart", "-progress", "pipe:1", "-nostats", "-f", "mp4", out)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }
