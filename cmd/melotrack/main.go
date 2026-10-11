// Command melotrack runs the Melotrack server, or with "worker" a remote
// worker that downloads and cuts videos for it.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/api"
	"github.com/nermline/Melotrack-Creator/internal/auth"
	"github.com/nermline/Melotrack-Creator/internal/config"
	"github.com/nermline/Melotrack-Creator/internal/game"
	"github.com/nermline/Melotrack-Creator/internal/live"
	"github.com/nermline/Melotrack-Creator/internal/media"
	"github.com/nermline/Melotrack-Creator/internal/store"
	"github.com/nermline/Melotrack-Creator/internal/worker"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	run := runServer
	if len(os.Args) > 1 && os.Args[1] == "worker" {
		run = runWorker
	}
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func checkTools(bins ...string) {
	for _, bin := range bins {
		if _, err := exec.LookPath(bin); err != nil {
			slog.Warn("required tool not found; media features will fail", "tool", bin)
		}
	}
}

func runServer() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	st, err := media.NewStorage(cfg.DataDir)
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.DatabasePath(), store.LegacyImport{
		DownloadsDir: cfg.LegacyDownloadsDir,
		SourceDir:    st.SourceDir(),
		ImagesDir:    st.ImagesDir(),
	})
	if err != nil {
		return err
	}

	checkTools(cfg.FFmpegPath, cfg.FFprobePath)
	yt := &media.YtDlp{Path: cfg.YtDlpPath, Proxy: cfg.YtDlpProxy, Cookies: cfg.YtDlpCookies, Extra: cfg.YtDlpExtra()}
	ff := &media.FFmpeg{FFmpeg: cfg.FFmpegPath, FFprobe: cfg.FFprobePath}

	// Downloading and cutting run here unless a remote worker is configured.
	var dl media.Downloader = media.LocalDownloader{YT: yt}
	var rd media.Renderer = media.LocalRenderer{FF: ff}
	var remotes []api.RemoteWorker
	if cfg.DownloadWorkerURL != "" {
		c := worker.NewClient(cfg.DownloadWorkerURL, cfg.WorkerToken)
		dl = c
		remotes = append(remotes, api.RemoteWorker{Role: "download", Client: c})
		slog.Info("downloads go to a remote worker", "url", cfg.DownloadWorkerURL)
	} else {
		checkTools(cfg.YtDlpPath)
	}
	if cfg.RenderWorkerURL != "" {
		c := worker.NewClient(cfg.RenderWorkerURL, cfg.WorkerToken)
		rd = c
		remotes = append(remotes, api.RemoteWorker{Role: "render", Client: c})
		slog.Info("clips are cut by a remote worker", "url", cfg.RenderWorkerURL)
	}

	mgr := media.NewManager(db, st, ff, media.NewMetaFetcher(cfg.YtDlpProxy), dl, rd, media.Options{
		DownloadWorkers: cfg.DownloadWorkers,
		RenderWorkers:   cfg.RenderWorkers,
		ClipHeight:      cfg.ClipHeight,
	})

	srv := api.NewServer(db, cfg,
		auth.New(cfg.JWTSecret, cfg.Password, cfg.SessionTTL, cfg.SecureCookies),
		mgr, yt, remotes, live.NewHub(cfg.AllowedOrigins), game.NewSessions(db))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mgr.Start(ctx)

	return serve(ctx, &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           srv.Router(),
		ReadHeaderTimeout: 15 * time.Second,
	}, "data", cfg.DataDir)
}

func runWorker() error {
	cfg, err := config.LoadWorker()
	if err != nil {
		return err
	}
	checkTools(cfg.FFmpegPath, cfg.FFprobePath, cfg.YtDlpPath)
	srv, err := worker.NewServer(worker.Options{
		Name:            cfg.Name,
		Token:           cfg.Token,
		Dir:             filepath.Join(cfg.DataDir, "worker"),
		YtDlp:           &media.YtDlp{Path: cfg.YtDlpPath, Proxy: cfg.YtDlpProxy, Cookies: cfg.YtDlpCookies, Extra: splitArgs(cfg.YtDlpExtraArgs)},
		FFmpeg:          &media.FFmpeg{FFmpeg: cfg.FFmpegPath, FFprobe: cfg.FFprobePath},
		DownloadWorkers: cfg.DownloadWorkers,
		RenderWorkers:   cfg.RenderWorkers,
		CacheBytes:      int64(cfg.CacheGB * (1 << 30)),
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		t := time.NewTicker(30 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				srv.CleanResults(2 * time.Hour)
			}
		}
	}()

	return serve(ctx, &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
	}, "worker", cfg.Name)
}

// serve runs an HTTP server until the context ends, then shuts it down.
func serve(ctx context.Context, s *http.Server, what, detail string) error {
	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", s.Addr, what, detail)
		errc <- s.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.Shutdown(sctx)
	}
	return nil
}

func splitArgs(s string) []string {
	return (&config.Config{YtDlpExtraArgs: s}).YtDlpExtra()
}
