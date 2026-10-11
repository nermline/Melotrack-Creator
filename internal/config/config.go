// Package config loads runtime settings from the environment (and an optional .env file).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Host string `envconfig:"HOST" default:"0.0.0.0"`
	Port string `envconfig:"PORT" default:"8080"`

	// DataDir holds the database (unless DATABASE is set) and all media files.
	DataDir string `envconfig:"DATA_DIR" default:"./data"`
	// Database is the SQLite file path. Defaults to DATA_DIR/melotrack.db.
	Database string `envconfig:"DATABASE"`
	// LegacyDownloadsDir is where the pre-v2 version kept its files; they are imported once.
	LegacyDownloadsDir string `envconfig:"LEGACY_DOWNLOADS_DIR" default:"./downloads"`
	// StaticDir is the built frontend. Empty or missing directory disables SPA serving.
	StaticDir string `envconfig:"STATIC_DIR" default:"./frontend/dist"`

	JWTSecret     string        `envconfig:"JWT_SECRET" required:"true"`
	SessionTTL    time.Duration `envconfig:"SESSION_TTL" default:"168h"`
	SecureCookies bool          `envconfig:"SECURE_COOKIES" default:"false"`
	// AllowedOrigins are extra origins allowed to open WebSockets (the dev server, for example).
	AllowedOrigins []string `envconfig:"ALLOWED_ORIGINS" default:"http://localhost:5173"`

	// Password is the single shared organiser password.
	Password string `envconfig:"APP_PASSWORD"`
	// LegacyAdminPassword is used when APP_PASSWORD is not set (pre-v2 .env files).
	LegacyAdminPassword string `envconfig:"ADMIN_PASSWORD"`

	YtDlpPath      string `envconfig:"YTDLP_PATH" default:"yt-dlp"`
	YtDlpProxy     string `envconfig:"YTDLP_PROXY"`
	YtDlpCookies   string `envconfig:"YTDLP_COOKIES"`
	YtDlpExtraArgs string `envconfig:"YTDLP_EXTRA_ARGS"`
	FFmpegPath     string `envconfig:"FFMPEG_PATH" default:"ffmpeg"`
	FFprobePath    string `envconfig:"FFPROBE_PATH" default:"ffprobe"`

	DownloadWorkers int   `envconfig:"DOWNLOAD_WORKERS" default:"2"`
	RenderWorkers   int   `envconfig:"RENDER_WORKERS" default:"1"`
	ClipHeight      int   `envconfig:"CLIP_HEIGHT" default:"720"`
	MaxUploadMB     int64 `envconfig:"MAX_UPLOAD_MB" default:"2048"`

	// Remote workers ("melotrack worker" on another machine, reached over a
	// VPN). Empty means the job runs on this server. Downloading and cutting
	// are configured separately and may point at the same worker.
	DownloadWorkerURL string `envconfig:"DOWNLOAD_WORKER_URL"`
	RenderWorkerURL   string `envconfig:"RENDER_WORKER_URL"`
	WorkerToken       string `envconfig:"WORKER_TOKEN"`
}

// WorkerConfig configures "melotrack worker".
type WorkerConfig struct {
	Listen  string  `envconfig:"WORKER_LISTEN" default:"0.0.0.0:8081"`
	Name    string  `envconfig:"WORKER_NAME"`
	Token   string  `envconfig:"WORKER_TOKEN"`
	DataDir string  `envconfig:"DATA_DIR" default:"./data"`
	CacheGB float64 `envconfig:"WORKER_CACHE_GB" default:"20"`

	YtDlpPath      string `envconfig:"YTDLP_PATH" default:"yt-dlp"`
	YtDlpProxy     string `envconfig:"YTDLP_PROXY"`
	YtDlpCookies   string `envconfig:"YTDLP_COOKIES"`
	YtDlpExtraArgs string `envconfig:"YTDLP_EXTRA_ARGS"`
	FFmpegPath     string `envconfig:"FFMPEG_PATH" default:"ffmpeg"`
	FFprobePath    string `envconfig:"FFPROBE_PATH" default:"ffprobe"`

	DownloadWorkers int `envconfig:"DOWNLOAD_WORKERS" default:"2"`
	RenderWorkers   int `envconfig:"RENDER_WORKERS" default:"2"`
}

func LoadWorker() (*WorkerConfig, error) {
	_ = godotenv.Load()
	cfg := &WorkerConfig{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if len(cfg.Token) < 16 {
		return nil, fmt.Errorf("config: WORKER_TOKEN must be at least 16 characters")
	}
	if cfg.Name == "" {
		cfg.Name, _ = os.Hostname()
	}
	return cfg, nil
}

func (c *Config) ListenAddr() string { return c.Host + ":" + c.Port }

func (c *Config) DatabasePath() string {
	if c.Database != "" {
		return c.Database
	}
	return filepath.Join(c.DataDir, "melotrack.db")
}

// YtDlpExtra splits YTDLP_EXTRA_ARGS on whitespace.
func (c *Config) YtDlpExtra() []string { return strings.Fields(c.YtDlpExtraArgs) }

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if len(cfg.JWTSecret) < 16 {
		return nil, fmt.Errorf("config: JWT_SECRET must be at least 16 characters")
	}
	if cfg.Password == "" {
		cfg.Password = cfg.LegacyAdminPassword
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("config: APP_PASSWORD is required")
	}
	switch cfg.ClipHeight {
	case 480, 720, 1080:
	default:
		return nil, fmt.Errorf("config: CLIP_HEIGHT must be 480, 720 or 1080")
	}
	cfg.DownloadWorkers = max(cfg.DownloadWorkers, 1)
	cfg.RenderWorkers = max(cfg.RenderWorkers, 1)
	if (cfg.DownloadWorkerURL != "" || cfg.RenderWorkerURL != "") && len(cfg.WorkerToken) < 16 {
		return nil, fmt.Errorf("config: WORKER_TOKEN (at least 16 characters) is required with a remote worker")
	}
	return cfg, nil
}
