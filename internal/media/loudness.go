package media

import (
	"bufio"
	"context"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// silenceLUFS is the floor used for silent seconds.
const silenceLUFS = -70.0

// LoudnessMap measures the loudness of every second of a file (LUFS, from
// EBU R128 momentary loudness). The editor uses it to preview a fragment as
// loud as the normalised clip will be.
func (f *FFmpeg) LoudnessMap(ctx context.Context, path string) ([]float64, error) {
	cmd := exec.CommandContext(ctx, f.FFmpeg, "-hide_banner", "-nostdin", "-i", path,
		"-vn", "-af", "ebur128=framelog=info", "-f", "null", "-")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	m := parseEBUR128(stderr)
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return m, nil
}

// parseEBUR128 turns ffmpeg's per-100ms "t: … M: …" lines into one value per
// second (energy average of the momentary loudness).
func parseEBUR128(r io.Reader) []float64 {
	var sums []float64
	var counts []int
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		ti := strings.Index(line, " t: ")
		mi := strings.Index(line, " M:")
		if ti < 0 || mi < 0 {
			continue
		}
		t, err1 := strconv.ParseFloat(firstField(line[ti+4:]), 64)
		lufs, err2 := strconv.ParseFloat(firstField(line[mi+3:]), 64)
		if err1 != nil || err2 != nil || t < 0 {
			continue
		}
		sec := int(t)
		for len(sums) <= sec {
			sums = append(sums, 0)
			counts = append(counts, 0)
		}
		sums[sec] += math.Pow(10, max(lufs, silenceLUFS)/10)
		counts[sec]++
	}
	out := make([]float64, len(sums))
	for i := range sums {
		v := silenceLUFS
		if counts[i] > 0 && sums[i] > 0 {
			v = max(10*math.Log10(sums[i]/float64(counts[i])), silenceLUFS)
		}
		out[i] = math.Round(v*10) / 10
	}
	return out
}

func firstField(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}
