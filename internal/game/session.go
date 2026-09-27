package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nermline/Melotrack-Creator/internal/media"
	"github.com/nermline/Melotrack-Creator/internal/store"
	"gorm.io/gorm"
)

// Sessions keeps one live show per project.
type Sessions struct {
	db       *gorm.DB
	mu       sync.Mutex
	m        map[uint]*Session
	onChange func(projectID uint)
}

func NewSessions(db *gorm.DB) *Sessions {
	return &Sessions{db: db, m: map[uint]*Session{}, onChange: func(uint) {}}
}

// OnChange registers the callback that pushes new state to connected devices.
func (ss *Sessions) OnChange(fn func(projectID uint)) { ss.onChange = fn }

var ErrNoProject = errors.New("project not found")

func (ss *Sessions) Get(projectID uint) (*Session, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if s, ok := ss.m[projectID]; ok {
		return s, nil
	}
	s, err := newSession(ss.db, projectID, func() { ss.onChange(projectID) })
	if err != nil {
		return nil, err
	}
	ss.m[projectID] = s
	return s, nil
}

// Existing returns the session if one is loaded.
func (ss *Sessions) Existing(projectID uint) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.m[projectID]
}

// Refresh reloads project details (title, theme, teams) into a running session.
func (ss *Sessions) Refresh(projectID uint) {
	if s := ss.Existing(projectID); s != nil {
		s.refreshInfo()
		ss.onChange(projectID)
	}
}

func (ss *Sessions) Drop(projectID uint) {
	ss.mu.Lock()
	s := ss.m[projectID]
	delete(ss.m, projectID)
	ss.mu.Unlock()
	if s != nil {
		s.close()
	}
}

type info struct {
	Title    string
	Theme    string
	JoinCode string
	RegOpen  bool
	ThinkMs  int64
	Teams    []string
}

type Session struct {
	projectID uint
	db        *gorm.DB
	changed   func()

	mu    sync.Mutex
	eng   *Engine
	info  info
	seq   int64
	timer *time.Timer
	gen   int

	saveMu    sync.Mutex
	saveData  string
	saveKick  chan struct{}
	closeOnce sync.Once
	done      chan struct{}
}

// knownPhases guards restoring a position saved by an older version
// (which, for example, had an intermediate-standings screen).
var knownPhases = map[Phase]bool{
	PhaseWelcome: true, PhaseCategory: true, PhaseCountdown: true, PhasePlaying: true,
	PhaseThinking: true, PhaseCollect: true, PhaseAnswers: true, PhaseScoring: true,
	PhaseResults: true, PhaseFinished: true,
}

type savedState struct {
	Phase  Phase      `json:"phase"`
	Cat    int        `json:"cat"`
	Item   int        `json:"item"`
	Reveal int        `json:"reveal"`
	Board  []Standing `json:"board,omitempty"`
}

func newSession(db *gorm.DB, projectID uint, changed func()) (*Session, error) {
	var p store.Project
	if err := db.First(&p, projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoProject
		}
		return nil, err
	}
	s := &Session{projectID: projectID, db: db, changed: changed,
		saveKick: make(chan struct{}, 1), done: make(chan struct{})}
	s.refreshInfo()
	s.eng = NewEngine(s, func() int64 { return time.Now().UnixMilli() })
	s.restore(p.GameState)
	go s.saver()
	return s, nil
}

// restore puts the show back where it was before a restart. A timed phase comes
// back paused so nothing plays until the operator resumes.
func (s *Session) restore(raw string) {
	if raw == "" {
		return
	}
	var st savedState
	if json.Unmarshal([]byte(raw), &st) != nil || !knownPhases[st.Phase] {
		return
	}
	e := s.eng
	if st.Cat >= len(e.Cats) && st.Phase != PhaseScoring && st.Phase != PhaseResults && st.Phase != PhaseFinished {
		return
	}
	e.Phase, e.Cat, e.Item, e.Reveal, e.Board = st.Phase, st.Cat, st.Item, st.Reveal, st.Board
	var d int64
	switch st.Phase {
	case PhaseCategory:
		d = categoryMs
	case PhaseCountdown:
		d = countdownMs
	case PhasePlaying:
		d = defaultClipMs
		if it := e.CurrentItem(); it != nil && it.DurationMs > 0 {
			d = it.DurationMs
		}
	case PhaseThinking:
		d = e.thinkMs()
	}
	if d > 0 {
		e.Timer = Timer{Duration: d, Remaining: d, Paused: true}
	}
}

// Command applies a remote action and pushes the result to every device.
func (s *Session) Command(action string, value float64) {
	s.mu.Lock()
	ok := s.eng.Apply(action, value)
	if ok {
		s.afterChange()
	}
	s.mu.Unlock()
	if ok {
		s.changed()
	}
}

// afterChange re-arms the phase timer and persists the position. Caller holds s.mu.
func (s *Session) afterChange() {
	s.seq++
	s.gen++
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if t := s.eng.Timer; t.Duration > 0 && !t.Paused {
		gen := s.gen
		s.timer = time.AfterFunc(time.Duration(max(s.eng.Remaining(), 0))*time.Millisecond, func() {
			s.mu.Lock()
			if gen != s.gen {
				s.mu.Unlock()
				return
			}
			s.eng.Expire()
			s.afterChange()
			s.mu.Unlock()
			s.changed()
		})
	}
	st := savedState{Phase: s.eng.Phase, Cat: s.eng.Cat, Item: s.eng.Item, Reveal: s.eng.Reveal, Board: s.eng.Board}
	data, _ := json.Marshal(st)
	s.saveMu.Lock()
	s.saveData = string(data)
	s.saveMu.Unlock()
	select {
	case s.saveKick <- struct{}{}:
	default:
	}
}

// saver writes the latest position in the background (coalescing bursts).
func (s *Session) saver() {
	for {
		select {
		case <-s.done:
			return
		case <-s.saveKick:
			s.saveMu.Lock()
			data := s.saveData
			s.saveMu.Unlock()
			if err := s.db.Model(&store.Project{}).Where("id = ?", s.projectID).Update("game_state", data).Error; err != nil {
				slog.Warn("save game state", "project", s.projectID, "err", err)
			}
		}
	}
}

func (s *Session) close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		if s.timer != nil {
			s.timer.Stop()
		}
		s.gen++
		s.mu.Unlock()
		close(s.done)
	})
}

func (s *Session) refreshInfo() {
	var p store.Project
	if s.db.First(&p, s.projectID).Error != nil {
		return
	}
	var teams []string
	s.db.Model(&store.Team{}).Where("project_id = ?", s.projectID).Order("created_at, id").Pluck("name", &teams)
	think := int64(p.ThinkSeconds) * 1000
	s.mu.Lock()
	s.info = info{Title: p.Title, Theme: p.Theme, JoinCode: p.JoinCode, RegOpen: p.RegistrationOpen, ThinkMs: think, Teams: teams}
	s.seq++
	s.mu.Unlock()
}

// Source implementation (called by the engine with s.mu held, or during construction).

func (s *Session) ThinkMs() int64 { return s.info.ThinkMs }

func (s *Session) Program() []Category { return LoadProgram(s.db, s.projectID) }

func (s *Session) Leaderboard() []Standing { return LoadLeaderboard(s.db, s.projectID) }

// LoadProgram reads categories and items in show order.
func LoadProgram(db *gorm.DB, projectID uint) []Category {
	var cats []store.Category
	err := db.Where("project_id = ?", projectID).Order("position, id").
		Preload("Items", func(d *gorm.DB) *gorm.DB { return d.Order("position, id") }).
		Preload("Items.Media").
		Find(&cats).Error
	if err != nil {
		slog.Warn("load program", "project", projectID, "err", err)
	}
	out := make([]Category, 0, len(cats))
	for _, c := range cats {
		gc := Category{ID: c.ID, Title: c.Title, Items: make([]Item, 0, len(c.Items))}
		for _, it := range c.Items {
			gc.Items = append(gc.Items, Item{
				ID:         it.ID,
				Answer:     it.Answer,
				ImageURL:   media.AnswerImageURL(&it, &it.Media),
				ClipURL:    media.ClipURL(it.ClipFile),
				ShowVideo:  it.ShowVideo,
				DurationMs: int64(it.ClipSeconds() * 1000),
				ClipReady:  it.ClipStatus == store.ClipReady,
			})
		}
		out = append(out, gc)
	}
	return out
}

// LoadLeaderboard sums points per team (plus manual bonus) and ranks them.
func LoadLeaderboard(db *gorm.DB, projectID uint) []Standing {
	var rows []Standing
	err := db.Raw(`SELECT t.id AS team_id, t.name AS name, t.bonus + COALESCE(SUM(s.points), 0) AS points
		FROM teams t LEFT JOIN scores s ON s.team_id = t.id
		WHERE t.project_id = ? GROUP BY t.id`, projectID).Scan(&rows).Error
	if err != nil {
		slog.Warn("load leaderboard", "project", projectID, "err", err)
	}
	return RankStandings(rows)
}

// ---- views sent to devices ----

type ClipView struct {
	ID         uint   `json:"id"`
	ClipURL    string `json:"clip_url"`
	ShowVideo  bool   `json:"show_video"`
	DurationMs int64  `json:"duration_ms"`
	ClipReady  bool   `json:"clip_ready"`
}

type AnswerView struct {
	ID       uint   `json:"id"`
	Answer   string `json:"answer"`
	ImageURL string `json:"image_url"`
}

type CategoryView struct {
	ID    uint       `json:"id"`
	Title string     `json:"title"`
	Items []ClipView `json:"items"`
}

type HostView struct {
	Categories []string `json:"categories"`
	Answers    []string `json:"answers"`
	Current    string   `json:"current"`
	Next       string   `json:"next"`
	Warnings   []string `json:"warnings"`
	Teams      int      `json:"teams"`
}

type View struct {
	Seq       int64  `json:"seq"`
	ServerNow int64  `json:"server_now"`
	Phase     Phase  `json:"phase"`
	Title     string `json:"title"`
	Theme     string `json:"theme"`
	CatIndex  int    `json:"cat_index"`
	CatCount  int    `json:"cat_count"`
	ItemIndex int    `json:"item_index"`
	ItemCount int    `json:"item_count"`
	Timer     Timer  `json:"timer"`

	Categories []string      `json:"categories,omitempty"`
	Teams      []string      `json:"teams,omitempty"`
	JoinCode   string        `json:"join_code,omitempty"`
	Category   *CategoryView `json:"category,omitempty"`
	Answers    []AnswerView  `json:"answers,omitempty"`
	Board      []Standing    `json:"board,omitempty"`
	Revealed   int           `json:"revealed"`
	Host       *HostView     `json:"host,omitempty"`
}

// View renders the state for a screen (host=false) or a remote/organiser (host=true).
func (s *Session) View(host bool) View {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.eng
	v := View{
		Seq: s.seq, ServerNow: time.Now().UnixMilli(), Phase: e.Phase,
		Title: s.info.Title, Theme: s.info.Theme,
		CatIndex: e.Cat, CatCount: len(e.Cats), ItemIndex: e.Item,
		Timer: e.Timer, Revealed: e.Revealed(),
	}
	var cat *Category
	if e.Cat >= 0 && e.Cat < len(e.Cats) {
		cat = &e.Cats[e.Cat]
		v.ItemCount = len(cat.Items)
	}

	switch e.Phase {
	case PhaseWelcome:
		v.Teams = s.info.Teams
		for _, c := range e.Cats {
			v.Categories = append(v.Categories, c.Title)
		}
		if s.info.RegOpen {
			v.JoinCode = s.info.JoinCode
		}
	case PhaseResults, PhaseFinished:
		v.Board = e.Board
	}
	if cat != nil && e.Phase != PhaseWelcome && e.Phase != PhaseScoring && e.Phase != PhaseResults && e.Phase != PhaseFinished {
		cv := &CategoryView{ID: cat.ID, Title: cat.Title, Items: make([]ClipView, 0, len(cat.Items))}
		for _, it := range cat.Items {
			cv.Items = append(cv.Items, ClipView{ID: it.ID, ClipURL: it.ClipURL, ShowVideo: it.ShowVideo, DurationMs: it.DurationMs, ClipReady: it.ClipReady})
		}
		v.Category = cv
		if e.Phase == PhaseAnswers {
			for _, it := range cat.Items {
				v.Answers = append(v.Answers, AnswerView{ID: it.ID, Answer: it.Answer, ImageURL: it.ImageURL})
			}
		}
	}

	if host {
		h := &HostView{Teams: len(s.info.Teams)}
		for _, c := range e.Cats {
			h.Categories = append(h.Categories, c.Title)
		}
		if cat != nil {
			for i, it := range cat.Items {
				h.Answers = append(h.Answers, it.Answer)
				if it.ClipURL == "" {
					h.Warnings = append(h.Warnings, fmt.Sprintf("Пісня %d без кліпу — не прозвучить", i+1))
				} else if !it.ClipReady {
					h.Warnings = append(h.Warnings, fmt.Sprintf("Пісня %d: кліп застарів або ще рендериться", i+1))
				}
			}
			if cur := e.CurrentItem(); cur != nil {
				h.Current = cur.Answer
			}
			if e.Phase == PhasePlaying || e.Phase == PhaseThinking {
				if e.Item+1 < len(cat.Items) {
					h.Next = cat.Items[e.Item+1].Answer
				}
			}
		}
		v.Host = h
	}
	return v
}
