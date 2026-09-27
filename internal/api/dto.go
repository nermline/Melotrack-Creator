package api

import (
	"strconv"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/media"
	"github.com/nermline/Melotrack-Creator/internal/store"
)

type mediaDTO struct {
	ID        uint    `json:"id"`
	Kind      string  `json:"kind"` // youtube | upload
	YouTubeID string  `json:"youtube_id,omitempty"`
	URL       string  `json:"url,omitempty"`
	Title     string  `json:"title"`
	Channel   string  `json:"channel"`
	Status    string  `json:"status"`
	Error     string  `json:"error,omitempty"`
	ErrorCode string  `json:"error_code,omitempty"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Duration  float64 `json:"duration"`
	SourceURL string  `json:"source_url,omitempty"`
	ThumbURL  string  `json:"thumb_url,omitempty"`
	Replaced  bool    `json:"replaced"`            // a YouTube video whose file was uploaded by hand
	Suggested string  `json:"suggested,omitempty"` // "Artist — Song" guessed from the title
}

func toMediaDTO(m *store.Media) mediaDTO {
	d := mediaDTO{
		ID: m.ID, Title: m.Title, Channel: m.Channel, Status: m.Status,
		Error: m.Error, ErrorCode: m.ErrorCode,
		Width: m.Width, Height: m.Height, Duration: m.Duration,
		SourceURL: media.SourceURL(m), ThumbURL: media.ThumbURL(m.ThumbFile),
	}
	if media.IsUploadID(m.YouTubeID) {
		d.Kind = "upload"
	} else {
		d.Kind = "youtube"
		d.YouTubeID = m.YouTubeID
		d.URL = media.WatchURL(m.YouTubeID)
		d.Replaced = m.SourceKind == "upload"
		if m.Title != "" {
			d.Suggested = media.SuggestAnswer(m.Title, m.Channel)
		}
	}
	return d
}

type rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type clipDTO struct {
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
	Error  string `json:"error,omitempty"`
}

type itemDTO struct {
	ID         uint    `json:"id"`
	CategoryID uint    `json:"category_id"`
	Position   int     `json:"position"`
	MediaID    uint    `json:"media_id"`
	Answer     string  `json:"answer"`
	ShowVideo  bool    `json:"show_video"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	GainDB     float64 `json:"gain_db"`
	Frame      string  `json:"frame"`
	Crop       *rect   `json:"crop"`
	ImageURL   string  `json:"image_url,omitempty"`    // uploaded picture
	AnswerURL  string  `json:"answer_image,omitempty"` // what the audience sees (picture or thumbnail)
	Clip       clipDTO `json:"clip"`
}

func toItemDTO(it *store.Item) itemDTO {
	d := itemDTO{
		ID: it.ID, CategoryID: it.CategoryID, Position: it.Position, MediaID: it.MediaID,
		Answer: it.Answer, ShowVideo: it.ShowVideo,
		Start: it.StartSec, End: it.EndSec, GainDB: it.GainDB, Frame: it.Frame,
		ImageURL:  media.ImageURL(it.ImageFile),
		AnswerURL: media.AnswerImageURL(it, &it.Media),
		Clip:      clipDTO{Status: it.ClipStatus, URL: media.ClipURL(it.ClipFile), Error: it.ClipError},
	}
	if it.Frame == store.FrameCrop && it.CropW > 0 {
		d.Crop = &rect{X: it.CropX, Y: it.CropY, W: it.CropW, H: it.CropH}
	}
	return d
}

type categoryDTO struct {
	ID       uint      `json:"id"`
	Title    string    `json:"title"`
	Position int       `json:"position"`
	Items    []itemDTO `json:"items"`
}

type teamDTO struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Bonus     float64   `json:"bonus"`
	CreatedAt time.Time `json:"created_at"`
}

func toTeamDTO(t *store.Team) teamDTO {
	return teamDTO{ID: t.ID, Name: t.Name, Bonus: t.Bonus, CreatedAt: t.CreatedAt}
}

type projectDTO struct {
	ID               uint                `json:"id"`
	Title            string              `json:"title"`
	CreatedAt        time.Time           `json:"created_at"`
	Theme            string              `json:"theme"`
	ThinkSeconds     int                 `json:"think_seconds"`
	RegistrationOpen bool                `json:"registration_open"`
	JoinCode         string              `json:"join_code"`
	Categories       []categoryDTO       `json:"categories"`
	Media            map[string]mediaDTO `json:"media"`
	Teams            []teamDTO           `json:"teams"`
}

func toProjectDTO(p *store.Project) projectDTO {
	d := projectDTO{
		ID: p.ID, Title: p.Title, CreatedAt: p.CreatedAt, Theme: p.Theme,
		ThinkSeconds: p.ThinkSeconds, RegistrationOpen: p.RegistrationOpen, JoinCode: p.JoinCode,
		Categories: make([]categoryDTO, 0, len(p.Categories)),
		Media:      map[string]mediaDTO{},
		Teams:      make([]teamDTO, 0, len(p.Teams)),
	}
	for _, c := range p.Categories {
		cd := categoryDTO{ID: c.ID, Title: c.Title, Position: c.Position, Items: make([]itemDTO, 0, len(c.Items))}
		for i := range c.Items {
			it := &c.Items[i]
			cd.Items = append(cd.Items, toItemDTO(it))
			d.Media[strconv.FormatUint(uint64(it.MediaID), 10)] = toMediaDTO(&it.Media)
		}
		d.Categories = append(d.Categories, cd)
	}
	for i := range p.Teams {
		d.Teams = append(d.Teams, toTeamDTO(&p.Teams[i]))
	}
	return d
}
