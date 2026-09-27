// Package game is the show's state machine. The server owns the clock: screens
// and remotes only render the state they receive, so every device agrees on what
// is playing and when it ends.
package game

type Phase string

const (
	PhaseWelcome   Phase = "welcome"   // project title, registered teams, join QR
	PhaseCategory  Phase = "category"  // category title card (timed)
	PhaseCountdown Phase = "countdown" // 3-2-1 before the first song (timed)
	PhasePlaying   Phase = "playing"   // a clip plays (timed by clip length)
	PhaseThinking  Phase = "thinking"  // pause to write the answer down (timed)
	PhaseCollect   Phase = "collect"   // "hand in your sheets, answers coming up"
	PhaseAnswers   Phase = "answers"   // correct answers of the category
	PhaseStandings Phase = "standings" // optional intermediate table between categories
	PhaseScoring   Phase = "scoring"   // "counting the points…" before the results
	PhaseResults   Phase = "results"   // winners revealed step by step
	PhaseFinished  Phase = "finished"  // thank-you screen
)

const (
	categoryMs     int64 = 3500
	countdownMs    int64 = 3000
	defaultClipMs  int64 = 15000
	defaultThinkMs int64 = 10000
)

type Item struct {
	ID         uint   `json:"id"`
	Answer     string `json:"answer"`
	ImageURL   string `json:"image_url"`
	ClipURL    string `json:"clip_url"`
	ShowVideo  bool   `json:"show_video"`
	DurationMs int64  `json:"duration_ms"`
	ClipReady  bool   `json:"clip_ready"`
}

type Category struct {
	ID    uint   `json:"id"`
	Title string `json:"title"`
	Items []Item `json:"items"`
}

type Standing struct {
	TeamID uint    `json:"team_id"`
	Name   string  `json:"name"`
	Points float64 `json:"points"`
	Rank   int     `json:"rank"`
}

// Source supplies the engine with fresh data from the database.
type Source interface {
	Program() []Category
	Leaderboard() []Standing
	ThinkMs() int64
}

type Timer struct {
	EndsAt    int64 `json:"ends_at"`      // unix ms; 0 when stopped or paused
	Duration  int64 `json:"duration_ms"`  // full length of the phase; 0 = untimed
	Remaining int64 `json:"remaining_ms"` // valid while paused
	Paused    bool  `json:"paused"`
}

type Engine struct {
	Phase  Phase
	Cat    int
	Item   int
	Reveal int // results: number of reveal steps taken
	Timer  Timer
	Cats   []Category
	Board  []Standing

	src Source
	now func() int64
}

func NewEngine(src Source, now func() int64) *Engine {
	e := &Engine{Phase: PhaseWelcome, src: src, now: now}
	e.Cats = src.Program()
	return e
}

// Apply runs a remote command. It returns false for unknown or inapplicable commands.
func (e *Engine) Apply(action string, value float64) bool {
	switch action {
	case "start":
		e.Cats = e.src.Program()
		e.enterCategory(0)
	case "next":
		e.Next()
	case "back":
		e.Back()
	case "pause":
		return e.pause()
	case "resume":
		return e.resume()
	case "toggle_pause":
		if e.Timer.Paused {
			return e.resume()
		}
		return e.pause()
	case "seek":
		return e.seek(int64(value * 1000))
	case "replay":
		return e.replay()
	case "goto":
		i := int(value)
		if i < 0 || i >= len(e.Cats) {
			return false
		}
		e.enterCategory(i)
	case "standings":
		e.enterStandings()
	case "results":
		e.enterScoring()
	case "reset":
		e.Cats = e.src.Program()
		e.enterWelcome()
	default:
		return false
	}
	return true
}

// Expire is called when the phase timer runs out.
func (e *Engine) Expire() {
	if e.Timer.Duration > 0 && !e.Timer.Paused {
		e.Next()
	}
}

func (e *Engine) Next() {
	switch e.Phase {
	case PhaseWelcome:
		e.Cats = e.src.Program()
		e.enterCategory(0)
	case PhaseCategory:
		if len(e.curItems()) > 0 {
			e.set(PhaseCountdown, countdownMs)
		} else {
			e.set(PhaseCollect, 0)
		}
	case PhaseCountdown:
		e.enterPlaying(0)
	case PhasePlaying:
		e.set(PhaseThinking, e.thinkMs())
	case PhaseThinking:
		if e.Item+1 < len(e.curItems()) {
			e.enterPlaying(e.Item + 1)
		} else {
			e.set(PhaseCollect, 0)
		}
	case PhaseCollect:
		e.set(PhaseAnswers, 0)
	case PhaseAnswers, PhaseStandings:
		if e.Cat+1 < len(e.Cats) {
			e.enterCategory(e.Cat + 1)
		} else {
			e.enterScoring()
		}
	case PhaseScoring:
		e.enterResults()
	case PhaseResults:
		if e.Reveal < len(revealSteps(len(e.Board))) {
			e.Reveal++
		} else {
			e.set(PhaseFinished, 0)
		}
	}
}

func (e *Engine) Back() {
	switch e.Phase {
	case PhaseCategory:
		if e.Cat > 0 {
			e.Cat--
			e.Item = 0
			e.set(PhaseAnswers, 0)
		} else {
			e.enterWelcome()
		}
	case PhaseCountdown:
		e.set(PhaseCategory, categoryMs)
	case PhasePlaying, PhaseThinking:
		if e.Item > 0 {
			e.enterPlaying(e.Item - 1)
		} else {
			e.set(PhaseCountdown, countdownMs)
		}
	case PhaseCollect:
		if n := len(e.curItems()); n > 0 {
			e.enterPlaying(n - 1)
		} else {
			e.set(PhaseCategory, categoryMs)
		}
	case PhaseAnswers:
		e.set(PhaseCollect, 0)
	case PhaseStandings:
		e.set(PhaseAnswers, 0)
	case PhaseScoring:
		if len(e.Cats) > 0 {
			e.Cat = len(e.Cats) - 1
			e.set(PhaseAnswers, 0)
		} else {
			e.enterWelcome()
		}
	case PhaseResults:
		if e.Reveal > 0 {
			e.Reveal--
		} else {
			e.set(PhaseScoring, 0)
		}
	case PhaseFinished:
		if len(e.Board) > 0 {
			e.Phase = PhaseResults
			e.Reveal = len(revealSteps(len(e.Board)))
			e.Timer = Timer{}
		} else {
			e.set(PhaseScoring, 0)
		}
	}
}

func (e *Engine) enterWelcome() {
	e.Cat, e.Item, e.Reveal, e.Board = 0, 0, 0, nil
	e.set(PhaseWelcome, 0)
}

func (e *Engine) enterCategory(i int) {
	if i >= len(e.Cats) {
		e.enterScoring()
		return
	}
	// Pick up last-minute edits (fixed answers, re-rendered clips) between categories.
	if fresh := e.src.Program(); len(fresh) == len(e.Cats) {
		e.Cats = fresh
	}
	e.Cat, e.Item = i, 0
	e.set(PhaseCategory, categoryMs)
}

func (e *Engine) enterPlaying(j int) {
	e.Item = j
	d := defaultClipMs
	if items := e.curItems(); j < len(items) && items[j].DurationMs > 0 {
		d = items[j].DurationMs
	}
	e.set(PhasePlaying, d)
}

func (e *Engine) enterStandings() {
	e.Board = e.src.Leaderboard()
	e.set(PhaseStandings, 0)
}

func (e *Engine) enterScoring() {
	if len(e.Cats) > 0 && e.Cat >= len(e.Cats) {
		e.Cat = len(e.Cats) - 1
	}
	e.set(PhaseScoring, 0)
}

func (e *Engine) enterResults() {
	e.Board = e.src.Leaderboard()
	if len(e.Board) == 0 {
		e.set(PhaseFinished, 0)
		return
	}
	e.Reveal = 0
	e.set(PhaseResults, 0)
}

func (e *Engine) set(p Phase, durationMs int64) {
	e.Phase = p
	if durationMs > 0 {
		e.Timer = Timer{EndsAt: e.now() + durationMs, Duration: durationMs}
	} else {
		e.Timer = Timer{}
	}
}

func (e *Engine) curItems() []Item {
	if e.Cat < 0 || e.Cat >= len(e.Cats) {
		return nil
	}
	return e.Cats[e.Cat].Items
}

// CurrentItem returns the item being played or thought about, if any.
func (e *Engine) CurrentItem() *Item {
	switch e.Phase {
	case PhaseCountdown, PhasePlaying, PhaseThinking:
		if items := e.curItems(); e.Item >= 0 && e.Item < len(items) {
			return &items[e.Item]
		}
	}
	return nil
}

func (e *Engine) thinkMs() int64 {
	if ms := e.src.ThinkMs(); ms > 0 {
		return ms
	}
	return defaultThinkMs
}

func (e *Engine) Remaining() int64 {
	if e.Timer.Duration == 0 {
		return 0
	}
	if e.Timer.Paused {
		return e.Timer.Remaining
	}
	return max(e.Timer.EndsAt-e.now(), 0)
}

func (e *Engine) pause() bool {
	if e.Timer.Duration == 0 || e.Timer.Paused {
		return false
	}
	e.Timer.Remaining = e.Remaining()
	e.Timer.EndsAt = 0
	e.Timer.Paused = true
	return true
}

func (e *Engine) resume() bool {
	if !e.Timer.Paused {
		return false
	}
	e.Timer.EndsAt = e.now() + e.Timer.Remaining
	e.Timer.Remaining = 0
	e.Timer.Paused = false
	return true
}

func (e *Engine) seek(deltaMs int64) bool {
	if e.Timer.Duration == 0 {
		return false
	}
	rem := min(max(e.Remaining()-deltaMs, 0), e.Timer.Duration)
	if e.Timer.Paused {
		e.Timer.Remaining = rem
		return true
	}
	if rem == 0 {
		e.Next()
		return true
	}
	e.Timer.EndsAt = e.now() + rem
	return true
}

func (e *Engine) replay() bool {
	switch e.Phase {
	case PhasePlaying, PhaseThinking:
		e.enterPlaying(e.Item)
	case PhaseCountdown, PhaseCategory:
		e.set(e.Phase, e.Timer.Duration)
	default:
		return false
	}
	return true
}

// Revealed is how many teams (counted from the last place) are visible on the results screen.
func (e *Engine) Revealed() int {
	steps := revealSteps(len(e.Board))
	switch {
	case e.Phase == PhaseFinished:
		return len(e.Board)
	case e.Phase != PhaseResults || e.Reveal == 0:
		return 0
	case e.Reveal > len(steps):
		return len(e.Board)
	}
	return steps[e.Reveal-1]
}

// revealSteps: everyone below the podium at once, then third, second and first.
func revealSteps(n int) []int {
	if n == 0 {
		return nil
	}
	var steps []int
	if n > 3 {
		steps = append(steps, n-3)
	}
	for i := max(n-2, 1); i <= n; i++ {
		steps = append(steps, i)
	}
	return steps
}

// RankStandings sorts by points and assigns competition ranks (1, 2, 2, 4).
func RankStandings(s []Standing) []Standing {
	out := append([]Standing(nil), s...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && less(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	for i := range out {
		if i > 0 && out[i].Points == out[i-1].Points {
			out[i].Rank = out[i-1].Rank
		} else {
			out[i].Rank = i + 1
		}
	}
	return out
}

func less(a, b Standing) bool {
	if a.Points != b.Points {
		return a.Points > b.Points
	}
	return a.Name < b.Name
}
