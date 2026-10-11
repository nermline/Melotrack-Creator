package worker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/media"
)

// Options configure a worker.
type Options struct {
	Name            string
	Token           string
	Dir             string // working directory: cache/, results/, tmp/
	YtDlp           *media.YtDlp
	FFmpeg          *media.FFmpeg
	DownloadWorkers int
	RenderWorkers   int
	CacheBytes      int64 // sources above this total are evicted, oldest first
}

type Server struct {
	opt   Options
	dlSem chan struct{}
	rdSem chan struct{}
	mu    sync.Mutex // guards cache eviction
}

func NewServer(opt Options) (*Server, error) {
	if opt.Token == "" {
		return nil, errors.New("worker: WORKER_TOKEN is required")
	}
	for _, d := range []string{"cache", "results", "tmp"} {
		if err := os.MkdirAll(filepath.Join(opt.Dir, d), 0o755); err != nil {
			return nil, err
		}
	}
	// Leftovers of interrupted jobs.
	entries, _ := os.ReadDir(filepath.Join(opt.Dir, "tmp"))
	for _, e := range entries {
		os.RemoveAll(filepath.Join(opt.Dir, "tmp", e.Name()))
	}
	return &Server{
		opt:   opt,
		dlSem: make(chan struct{}, max(opt.DownloadWorkers, 1)),
		rdSem: make(chan struct{}, max(opt.RenderWorkers, 1)),
	}, nil
}

func (s *Server) cachePath(hash string) string { return filepath.Join(s.opt.Dir, "cache", hash+".mp4") }
func (s *Server) resultPath(token string) string {
	return filepath.Join(s.opt.Dir, "results", token+".mp4")
}

var hashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)
var tokenRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("POST /v1/download", s.download)
	mux.HandleFunc("HEAD /v1/sources/{hash}", s.getSource)
	mux.HandleFunc("GET /v1/sources/{hash}", s.getSource)
	mux.HandleFunc("PUT /v1/sources/{hash}", s.putSource)
	mux.HandleFunc("POST /v1/render", s.render)
	mux.HandleFunc("GET /v1/results/{token}", s.getResult)
	mux.HandleFunc("DELETE /v1/results/{token}", s.deleteResult)

	root := http.NewServeMux()
	// Liveness for Docker's health check; says nothing about the worker.
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok\n") })
	root.Handle("/", s.auth(mux))
	return root
}

func (s *Server) auth(next http.Handler) http.Handler {
	want := []byte("Bearer " + s.opt.Token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CleanResults removes clips the server never collected (it crashed, say).
func (s *Server) CleanResults(olderThan time.Duration) {
	entries, _ := os.ReadDir(filepath.Join(s.opt.Dir, "results"))
	for _, e := range entries {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > olderThan {
			os.Remove(filepath.Join(s.opt.Dir, "results", e.Name()))
		}
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	v, _ := s.opt.YtDlp.Version(ctx)
	writeJSON(w, Health{OK: true, Name: s.opt.Name, YtDlpVersion: v, CachedBytes: s.cacheSize()})
}

// stream sends job events as NDJSON with periodic heartbeats, so the server
// can tell a long silent step (yt-dlp resolving formats) from a dead worker.
type stream struct {
	mu   sync.Mutex
	w    http.ResponseWriter
	f    http.Flusher
	enc  *json.Encoder
	stop chan struct{}
}

func newStream(w http.ResponseWriter) *stream {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	f, _ := w.(http.Flusher)
	st := &stream{w: w, f: f, enc: json.NewEncoder(w), stop: make(chan struct{})}
	st.flush()
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-st.stop:
				return
			case <-t.C:
				st.send(event{Heartbeat: true})
			}
		}
	}()
	return st
}

func (st *stream) send(e event) {
	st.mu.Lock()
	defer st.mu.Unlock()
	_ = st.enc.Encode(e)
	st.flush()
}

func (st *stream) flush() {
	if st.f != nil {
		st.f.Flush()
	}
}

func (st *stream) progress(p float64) { st.send(event{Progress: &p}) }
func (st *stream) close()             { close(st.stop) }

func (st *stream) fail(code, msg, detail string) {
	st.send(event{Error: &errorEvent{Code: code, Message: msg, Detail: detail}})
}

func acquire(ctx context.Context, sem chan struct{}) bool {
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	var req downloadRequest
	if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req) != nil || media.ParseYouTubeID(req.YouTubeID) != req.YouTubeID {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	st := newStream(w)
	defer st.close()
	if !acquire(ctx, s.dlSem) {
		return
	}
	defer func() { <-s.dlSem }()

	dir, err := os.MkdirTemp(filepath.Join(s.opt.Dir, "tmp"), "dl-")
	if err != nil {
		st.fail("failed", "Воркер не зміг створити тимчасову теку", err.Error())
		return
	}
	defer os.RemoveAll(dir)

	slog.Info("download", "youtube", req.YouTubeID)
	res, err := s.opt.YtDlp.Download(ctx, req.YouTubeID, dir, throttle(st.progress))
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		var de *media.DownloadError
		if errors.As(err, &de) {
			slog.Warn("download failed", "youtube", req.YouTubeID, "code", de.Code, "detail", de.Detail)
			st.fail(de.Code, de.Message, de.Detail)
		} else {
			st.fail("failed", "Не вдалося завантажити відео", err.Error())
		}
		return
	}
	hash, err := media.HashFile(res.File)
	if err != nil {
		st.fail("failed", "Воркер не зміг прочитати файл", err.Error())
		return
	}
	loudness, err := s.opt.FFmpeg.LoudnessMap(ctx, res.File)
	if err != nil {
		slog.Warn("loudness analysis failed", "youtube", req.YouTubeID, "err", err)
	}
	if err := s.adopt(res.File, hash); err != nil {
		st.fail("failed", "Воркер не зміг зберегти файл", err.Error())
		return
	}
	slog.Info("downloaded", "youtube", req.YouTubeID, "hash", hash[:12])
	st.send(event{Done: &doneEvent{Source: hash, Title: res.Title, Loudness: loudness}})
}

// adopt moves a file into the cache under its hash and evicts old sources.
func (s *Server) adopt(file, hash string) error {
	dst := s.cachePath(hash)
	if _, err := os.Stat(dst); err == nil {
		os.Remove(file)
		now := time.Now()
		os.Chtimes(dst, now, now)
	} else if err := os.Rename(file, dst); err != nil {
		return err
	}
	s.evict(hash)
	return nil
}

func (s *Server) cacheSize() int64 {
	var total int64
	entries, _ := os.ReadDir(filepath.Join(s.opt.Dir, "cache"))
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			total += info.Size()
		}
	}
	return total
}

// evict deletes the least recently used sources above the cache limit.
func (s *Server) evict(keep string) {
	if s.opt.CacheBytes <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	type file struct {
		path string
		size int64
		mod  time.Time
	}
	var files []file
	var total int64
	entries, _ := os.ReadDir(filepath.Join(s.opt.Dir, "cache"))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		total += info.Size()
		if strings.TrimSuffix(e.Name(), ".mp4") != keep {
			files = append(files, file{filepath.Join(s.opt.Dir, "cache", e.Name()), info.Size(), info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, f := range files {
		if total <= s.opt.CacheBytes {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}

func (s *Server) getSource(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if !hashRe.MatchString(hash) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, s.cachePath(hash))
}

// putSource stores a source the server sends for rendering, checking its hash.
func (s *Server) putSource(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if !hashRe.MatchString(hash) {
		http.Error(w, "bad hash", http.StatusBadRequest)
		return
	}
	tmp, err := os.CreateTemp(filepath.Join(s.opt.Dir, "tmp"), "src-")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(tmp, h), r.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != hash {
		http.Error(w, "hash mismatch", http.StatusBadRequest)
		return
	}
	if err := s.adopt(tmp.Name(), hash); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request) {
	var req renderRequest
	if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req) != nil || !hashRe.MatchString(req.Source) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	src := s.cachePath(req.Source)
	if _, err := os.Stat(src); err != nil {
		http.Error(w, "source not cached", http.StatusConflict)
		return
	}
	ctx := r.Context()
	st := newStream(w)
	defer st.close()
	if !acquire(ctx, s.rdSem) {
		return
	}
	defer func() { <-s.rdSem }()
	now := time.Now()
	os.Chtimes(src, now, now) // recently used: keep it in the cache

	token := randomToken()
	tmp := filepath.Join(s.opt.Dir, "tmp", token+".mp4")
	spec := req.Spec
	spec.Source, spec.Out = src, tmp
	started := time.Now()
	if err := s.opt.FFmpeg.Run(ctx, media.ClipArgs(spec), spec.End-spec.Start, throttle(st.progress)); err != nil {
		os.Remove(tmp)
		if ctx.Err() == nil {
			st.fail("render_failed", "Не вдалося нарізати фрагмент", err.Error())
		}
		return
	}
	if err := os.Rename(tmp, s.resultPath(token)); err != nil {
		st.fail("render_failed", "Воркер не зміг зберегти кліп", err.Error())
		return
	}
	slog.Info("rendered", "source", req.Source[:12], "took", time.Since(started).Round(time.Millisecond))
	st.send(event{Done: &doneEvent{Result: token}})
}

func (s *Server) getResult(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !tokenRe.MatchString(token) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, s.resultPath(token))
}

func (s *Server) deleteResult(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if tokenRe.MatchString(token) {
		os.Remove(s.resultPath(token))
	}
	w.WriteHeader(http.StatusNoContent)
}

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// throttle limits progress events to a few per second.
func throttle(fn func(float64)) func(float64) {
	var mu sync.Mutex
	var last time.Time
	return func(f float64) {
		mu.Lock()
		defer mu.Unlock()
		if time.Since(last) < 500*time.Millisecond && f < 1 {
			return
		}
		last = time.Now()
		fn(f)
	}
}
