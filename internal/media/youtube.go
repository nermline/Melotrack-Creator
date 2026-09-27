package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ParseYouTubeID accepts any common YouTube link (watch, youtu.be, shorts, embed,
// live, music.youtube.com) or a bare 11-character id. It returns "" otherwise.
func ParseYouTubeID(raw string) string {
	raw = strings.TrimSpace(raw)
	if idRe.MatchString(raw) {
		return raw
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	var id string
	switch host {
	case "youtu.be":
		id = firstSegment(u.Path)
	case "youtube.com", "music.youtube.com", "youtube-nocookie.com":
		if v := u.Query().Get("v"); v != "" {
			id = v
			break
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 2 {
			switch parts[0] {
			case "embed", "shorts", "live", "v", "e":
				id = parts[1]
			}
		}
	}
	if idRe.MatchString(id) {
		return id
	}
	return ""
}

func firstSegment(p string) string {
	p = strings.Trim(p, "/")
	if i := strings.IndexByte(p, '/'); i >= 0 {
		p = p[:i]
	}
	return p
}

func WatchURL(id string) string { return "https://www.youtube.com/watch?v=" + id }

// IsUploadID reports whether a media key belongs to a manually uploaded file.
func IsUploadID(id string) bool { return strings.HasPrefix(id, "file-") }

var (
	// Bracketed fragments that only describe the video, e.g. "(Official Music Video)", "[4K]".
	noiseBrackets = regexp.MustCompile(`(?i)\s*[(\[【][^)\]】]*(official|video|audio|lyric|visuali[sz]er|\bmv\b|m/v|\bhd\b|\bhq\b|\b4k\b|remaster|\bclip\b|кліп|офіційн|прем.?єр|премьер)[^)\]】]*[)\]】]`)
	// Trailing "| Official Video" style suffixes.
	noiseSuffix = regexp.MustCompile(`(?i)\s*[|/]\s*[^|/]*(official|video|lyric|audio|visuali[sz]er|кліп|офіційн)[^|/]*$`)
	dashRe      = regexp.MustCompile(`\s+[-–—]\s+`)
	spacesRe    = regexp.MustCompile(`\s{2,}`)
)

// SuggestAnswer turns a video title into an answer like "Artist — Song".
func SuggestAnswer(title, channel string) string {
	t := noiseBrackets.ReplaceAllString(title, "")
	t = noiseSuffix.ReplaceAllString(t, "")
	t = strings.Trim(spacesRe.ReplaceAllString(t, " "), " -–—|")
	if t == "" {
		t = strings.TrimSpace(title)
	}
	if loc := dashRe.FindStringIndex(t); loc != nil {
		return strings.TrimSpace(t[:loc[0]]) + " — " + strings.TrimSpace(t[loc[1]:])
	}
	// Auto-generated "Artist - Topic" channels only put the song name in the title.
	if artist, ok := strings.CutSuffix(channel, " - Topic"); ok && artist != "" {
		return artist + " — " + t
	}
	return t
}

// Meta is what YouTube's oEmbed endpoint returns. It needs no API key and,
// unlike the video itself, is not blocked for datacenter IPs.
type Meta struct {
	Title   string `json:"title"`
	Channel string `json:"author_name"`
}

type MetaFetcher struct {
	client *http.Client
}

func NewMetaFetcher(proxy string) *MetaFetcher {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		if u, err := url.Parse(proxy); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &MetaFetcher{client: &http.Client{Timeout: 15 * time.Second, Transport: tr}}
}

func (f *MetaFetcher) OEmbed(ctx context.Context, id string) (Meta, error) {
	u := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(WatchURL(id))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := f.client.Do(req)
	if err != nil {
		return Meta{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Meta{}, fmt.Errorf("oembed: HTTP %d", resp.StatusCode)
	}
	var m Meta
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return Meta{}, err
	}
	return m, nil
}

// Thumbnail saves the best available thumbnail to dst.
func (f *MetaFetcher) Thumbnail(ctx context.Context, id, dst string) error {
	var lastErr error
	for _, name := range []string{"maxresdefault.jpg", "sddefault.jpg", "hqdefault.jpg"} {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://i.ytimg.com/vi/"+id+"/"+name, nil)
		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("thumbnail %s: HTTP %d", name, resp.StatusCode)
			continue
		}
		err = writeFile(dst, io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		return err
	}
	return lastErr
}

func writeFile(dst string, r io.Reader) error {
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
