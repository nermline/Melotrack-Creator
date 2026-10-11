package store

import (
	"strings"
	"time"
)

// Presentation themes understood by the frontend.
var Themes = []string{"neon", "vinyl", "stage"}

type Project struct {
	ID           uint `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Title        string
	Theme        string
	ThinkSeconds int
	// GameState is the persisted show position (JSON), so a restart does not lose it.
	GameState string

	Categories []Category `gorm:"foreignKey:ProjectID"`
	Teams      []Team     `gorm:"foreignKey:ProjectID"`
}

func (Project) TableName() string { return "projects" }

type Category struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ProjectID uint
	Title     string
	Position  int
	// Responsible is the organiser who prepares the category.
	Responsible string
	// Criteria tell the people scoring how many points to give for what.
	Criteria string

	Items []Item `gorm:"foreignKey:CategoryID"`
}

func (Category) TableName() string { return "categories" }

// Media statuses.
const (
	MediaPending     = "pending"     // queued for download
	MediaDownloading = "downloading" // yt-dlp running (or an upload being processed)
	MediaReady       = "ready"
	MediaError       = "error"
)

// Media is one source video, shared by every item that uses the same YouTube video.
type Media struct {
	ID         uint `gorm:"primaryKey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	YouTubeID  string `gorm:"column:youtube_id"`
	Title      string
	Channel    string
	Status     string
	Error      string
	ErrorCode  string
	SourceFile string // file name inside the source dir
	SourceKind string // "youtube" or "upload"
	ThumbFile  string // file name inside the thumbs dir
	Width      int
	Height     int
	Duration   float64
	// Version bumps whenever the source file is replaced, which invalidates rendered clips.
	Version int
	// Loudness is a JSON array with the loudness (LUFS) of every second of the
	// source; the editor uses it to play the fragment as loud as the clip will be.
	Loudness string
	// SourceHash is the sha256 of the source file; remote workers cache by it.
	SourceHash string
}

func (Media) TableName() string { return "media" }

// Clip statuses. They describe the clip for the item's *current* settings;
// ClipFile may still point at an older, playable render meanwhile.
const (
	ClipPending   = "pending"
	ClipRendering = "rendering"
	ClipReady     = "ready"
	ClipError     = "error"
)

// Frame modes.
const (
	FrameFit  = "fit"  // whole picture, letterboxed to 16:9
	FrameCrop = "crop" // user-selected 16:9 region
)

type Item struct {
	ID         uint `gorm:"primaryKey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	CategoryID uint
	Position   int
	MediaID    uint
	Answer     string
	AltAnswer  string // another correct answer, shown to scorers only
	ImageFile  string
	ShowVideo  bool
	StartSec   float64
	EndSec     float64
	GainDB     float64 `gorm:"column:gain_db"`
	Frame      string
	CropX      int
	CropY      int
	CropW      int
	CropH      int
	// Reverse plays the fragment backwards, sound and picture.
	Reverse bool
	// Configured is set once the organiser saved the fragment in the editor;
	// only then is the clip cut (new songs are not cut with default settings).
	Configured bool

	ClipStatus string
	ClipKey    string
	ClipFile   string
	ClipError  string

	Media Media `gorm:"foreignKey:MediaID"`
}

func (Item) TableName() string { return "items" }

func (it *Item) ClipSeconds() float64 { return it.EndSec - it.StartSec }

type Team struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ProjectID uint
	Name      string
	NameKey   string // lower-cased name; SQLite's NOCASE ignores Cyrillic
}

// TeamKey normalises a team name for the uniqueness check.
func TeamKey(name string) string { return strings.ToLower(strings.Join(strings.Fields(name), " ")) }

func (Team) TableName() string { return "teams" }

type Score struct {
	TeamID    uint `gorm:"primaryKey;autoIncrement:false"`
	ItemID    uint `gorm:"primaryKey;autoIncrement:false"`
	Points    float64
	UpdatedAt time.Time
}

func (Score) TableName() string { return "scores" }
