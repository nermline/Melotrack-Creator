package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/api"
	"github.com/nermline/Melotrack-Creator/internal/auth"
	"github.com/nermline/Melotrack-Creator/internal/config"
	"github.com/nermline/Melotrack-Creator/internal/game"
	"github.com/nermline/Melotrack-Creator/internal/live"
	"github.com/nermline/Melotrack-Creator/internal/media"
	"github.com/nermline/Melotrack-Creator/internal/store"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
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

	for _, bin := range []string{cfg.FFmpegPath, cfg.FFprobePath, cfg.YtDlpPath} {
		if _, err := exec.LookPath(bin); err != nil {
			slog.Warn("required tool not found; media features will fail", "tool", bin)
		}
	}

	yt := &media.YtDlp{Path: cfg.YtDlpPath, Proxy: cfg.YtDlpProxy, Cookies: cfg.YtDlpCookies, Extra: cfg.YtDlpExtra()}
	ff := &media.FFmpeg{FFmpeg: cfg.FFmpegPath, FFprobe: cfg.FFprobePath}
	mgr := media.NewManager(db, st, yt, ff, media.NewMetaFetcher(cfg.YtDlpProxy), media.Options{
		DownloadWorkers: cfg.DownloadWorkers,
		RenderWorkers:   cfg.RenderWorkers,
		ClipHeight:      cfg.ClipHeight,
	})

	srv := api.NewServer(db, cfg,
		auth.New(cfg.JWTSecret, cfg.Password, cfg.SessionTTL, cfg.SecureCookies),
		mgr, yt, live.NewHub(cfg.AllowedOrigins), game.NewSessions(db))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mgr.Start(ctx)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           srv.Router(),
		ReadHeaderTimeout: 15 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", "http://"+cfg.ListenAddr(), "data", cfg.DataDir)
		errc <- httpSrv.ListenAndServe()
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
		_ = httpSrv.Shutdown(sctx)
	}
	return nil
}
