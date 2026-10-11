package worker

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/media"
)

// silenceLimit is how long a job stream may stay quiet (no progress, no
// heartbeat) before the worker is considered gone.
const silenceLimit = 60 * time.Second

// Client sends jobs to a remote worker. It implements media.Downloader and
// media.Renderer.
type Client struct {
	base  string
	token string
	http  *http.Client
}

func NewClient(baseURL, token string) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	tr.ResponseHeaderTimeout = 30 * time.Second
	return &Client{base: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{Transport: tr}}
}

func (c *Client) URL() string { return c.base }

func unavailable(err error) error { return fmt.Errorf("%w: %v", media.ErrWorkerUnavailable, err) }

func (c *Client) request(ctx context.Context, method, path string, body io.Reader, size int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if size >= 0 {
		req.ContentLength = size
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, unavailable(err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, errors.New("воркер відхилив WORKER_TOKEN — перевірте, що він однаковий на обох серверах")
	}
	if resp.StatusCode >= 500 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, unavailable(fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg))))
	}
	return resp, nil
}

func (c *Client) postJSON(ctx context.Context, path string, v any) (*http.Response, error) {
	b, _ := json.Marshal(v)
	return c.request(ctx, http.MethodPost, path, bytes.NewReader(b), int64(len(b)))
}

// Health checks that the worker answers.
func (c *Client) Health(ctx context.Context) (Health, error) {
	var h Health
	resp, err := c.request(ctx, http.MethodGet, "/v1/health", nil, -1)
	if err != nil {
		return h, err
	}
	defer resp.Body.Close()
	return h, json.NewDecoder(resp.Body).Decode(&h)
}

// follow reads a job stream until "done" or "error". A silent or broken
// stream means the worker went away.
func (c *Client) follow(ctx context.Context, cancel context.CancelFunc, resp *http.Response, onProgress func(float64)) (*doneEvent, *errorEvent, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, nil, fmt.Errorf("воркер: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	watchdog := time.AfterFunc(silenceLimit, cancel)
	defer watchdog.Stop()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 8<<20)
	for sc.Scan() {
		watchdog.Reset(silenceLimit)
		var e event
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		switch {
		case e.Done != nil:
			return e.Done, nil, nil
		case e.Error != nil:
			return nil, e.Error, nil
		case e.Progress != nil && onProgress != nil:
			onProgress(*e.Progress)
		}
	}
	if ctx.Err() != nil && !errors.Is(context.Cause(ctx), errSilent) {
		return nil, nil, ctx.Err()
	}
	return nil, nil, unavailable(errors.New("звʼязок із воркером обірвався"))
}

var errSilent = errors.New("worker went silent")

// fetch saves a file from the worker to dst and returns its sha256.
func (c *Client) fetch(ctx context.Context, path, dst string) (string, error) {
	resp, err := c.request(ctx, http.MethodGet, path, nil, -1)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", unavailable(fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode))
	}
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst)
		return "", unavailable(err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c *Client) Download(ctx context.Context, youtubeID, dir string, onProgress func(float64)) (media.DownloadResult, error) {
	jctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	resp, err := c.postJSON(jctx, "/v1/download", downloadRequest{YouTubeID: youtubeID})
	if err != nil {
		return media.DownloadResult{}, err
	}
	done, jobErr, err := c.follow(jctx, func() { cancel(errSilent) }, resp, onProgress)
	if err != nil {
		return media.DownloadResult{}, err
	}
	if jobErr != nil {
		return media.DownloadResult{}, &media.DownloadError{Code: jobErr.Code, Message: jobErr.Message, Detail: jobErr.Detail}
	}
	file := filepath.Join(dir, youtubeID+".mp4")
	hash, err := c.fetch(ctx, "/v1/sources/"+done.Source, file)
	if err != nil {
		return media.DownloadResult{}, err
	}
	if hash != done.Source {
		os.Remove(file)
		return media.DownloadResult{}, unavailable(errors.New("файл від воркера пошкоджено при передачі"))
	}
	return media.DownloadResult{File: file, Title: done.Title, Hash: hash, Loudness: done.Loudness}, nil
}

// ensureSource uploads the source unless the worker already has it.
func (c *Client) ensureSource(ctx context.Context, src media.SourceRef) error {
	if src.Hash == "" {
		return errors.New("source has no hash")
	}
	resp, err := c.request(ctx, http.MethodHead, "/v1/sources/"+src.Hash, nil, -1)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	f, err := os.Open(src.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	resp, err = c.request(ctx, http.MethodPut, "/v1/sources/"+src.Hash, f, info.Size())
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return unavailable(fmt.Errorf("upload source: HTTP %d", resp.StatusCode))
	}
	return nil
}

func (c *Client) Render(ctx context.Context, src media.SourceRef, spec media.ClipSpec, onProgress func(float64)) error {
	if err := c.ensureSource(ctx, src); err != nil {
		return err
	}
	jctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	resp, err := c.postJSON(jctx, "/v1/render", renderRequest{Source: src.Hash, Spec: spec})
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusConflict {
		resp.Body.Close() // evicted between the check and the job: retry later
		return unavailable(errors.New("воркер не має вихідного відео"))
	}
	done, jobErr, err := c.follow(jctx, func() { cancel(errSilent) }, resp, onProgress)
	if err != nil {
		return err
	}
	if jobErr != nil {
		return fmt.Errorf("%s: %s", jobErr.Message, jobErr.Detail)
	}
	if _, err := c.fetch(ctx, "/v1/results/"+done.Result, spec.Out); err != nil {
		return err
	}
	if resp, err := c.request(ctx, http.MethodDelete, "/v1/results/"+done.Result, nil, -1); err == nil {
		resp.Body.Close()
	}
	return nil
}
