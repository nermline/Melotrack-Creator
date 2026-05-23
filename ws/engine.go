package ws

import (
	"time"

	"github.com/nermline/Melotrack-Creator/internal/models"
	"gorm.io/gorm"
)

const (
	titleDurationMs     int64 = 2800
	countdownDurationMs int64 = 3000
	thinkingDurationMs  int64 = 10000
	defaultClipMs       int64 = 5000
)

type gameItem struct {
	ID         uint
	ShowVideo  bool
	HasClip    bool
	DurationMs int64
}

type gameCategory struct {
	ID    uint
	Title string
	Items []gameItem
}

func loadSnapshot(db *gorm.DB, projectID string) []gameCategory {
	var cats []models.Category
	db.
		Preload("Items", func(d *gorm.DB) *gorm.DB { return d.Order("position ASC") }).
		Preload("Items.Media").
		Where("project_id = ?", projectID).
		Order("position ASC").
		Find(&cats)

	out := make([]gameCategory, 0, len(cats))
	for _, c := range cats {
		gc := gameCategory{ID: c.ID, Title: c.Title, Items: make([]gameItem, 0, len(c.Items))}
		for _, it := range c.Items {
			gc.Items = append(gc.Items, gameItem{
				ID:         it.ID,
				ShowVideo:  it.ShowVideo,
				HasClip:    it.Video.RenderStatus == "ready",
				DurationMs: clipDurationMs(it.Video),
			})
		}
		out = append(out, gc)
	}
	return out
}

func clipDurationMs(v models.Video) int64 {
	if v.EndTime > 0 && v.EndTime > v.StartTime {
		return int64((v.EndTime - v.StartTime) * 1000)
	}
	if v.Media.Duration > 0 {
		d := v.Media.Duration - v.StartTime
		if d > 0 {
			return int64(d * 1000)
		}
	}
	return 0
}

func (s *GameSession) curCat() *gameCategory {
	if s.State.CategoryIndex < 0 || s.State.CategoryIndex >= len(s.Categories) {
		return nil
	}
	return &s.Categories[s.State.CategoryIndex]
}

func (s *GameSession) reloadLocked() {
	s.Categories = loadSnapshot(s.db, s.ProjectID)
	s.State.TotalCategories = len(s.Categories)
}

func (s *GameSession) resetLocked() {
	s.clearTimerLocked()
	s.State = GameState{
		ProjectID:       s.ProjectID,
		Phase:           PhaseWelcome,
		TotalCategories: len(s.Categories),
	}
}

func (s *GameSession) startLocked() {
	if len(s.Categories) == 0 {
		s.enterFinishedLocked()
		return
	}
	s.State.CategoryIndex = 0
	s.enterCategoryTitleLocked()
}

func (s *GameSession) enterCategoryTitleLocked() {
	cat := s.curCat()
	if cat == nil {
		s.enterFinishedLocked()
		return
	}
	s.State.Phase = PhaseCategoryTitle
	s.State.CategoryID = cat.ID
	s.State.ItemIndex = 0
	s.State.ItemID = 0
	s.State.ShowVideo = false
	s.State.HasClip = false
	s.State.TotalItems = len(cat.Items)
	s.startTimerLocked(titleDurationMs, titleDurationMs)
}

func (s *GameSession) enterCountdownLocked(itemIndex int) {
	s.State.ItemIndex = itemIndex
	s.resolveItemLocked()
	s.State.Phase = PhaseCountdown
	s.startTimerLocked(countdownDurationMs, countdownDurationMs)
}

func (s *GameSession) enterPlayingLocked() {
	s.resolveItemLocked()
	dur := defaultClipMs
	if cat := s.curCat(); cat != nil && s.State.ItemIndex < len(cat.Items) {
		if d := cat.Items[s.State.ItemIndex].DurationMs; d > 0 {
			dur = d
		}
	}
	s.State.Phase = PhasePlaying
	s.startTimerLocked(dur, dur)
}

func (s *GameSession) enterThinkingLocked() {
	s.State.Phase = PhaseThinking
	s.startTimerLocked(thinkingDurationMs, thinkingDurationMs)
}

func (s *GameSession) enterAwaitAnswersLocked() {
	s.State.Phase = PhaseAwaitAnswers
	s.clearTimerLocked()
}

func (s *GameSession) enterAnswersLocked() {
	s.State.Phase = PhaseAnswers
	s.clearTimerLocked()
}

func (s *GameSession) enterFinishedLocked() {
	s.State.Phase = PhaseFinished
	s.clearTimerLocked()
}

func (s *GameSession) resolveItemLocked() {
	cat := s.curCat()
	if cat == nil {
		return
	}
	s.State.CategoryID = cat.ID
	s.State.TotalItems = len(cat.Items)
	if s.State.ItemIndex < 0 || s.State.ItemIndex >= len(cat.Items) {
		return
	}
	it := cat.Items[s.State.ItemIndex]
	s.State.ItemID = it.ID
	s.State.ShowVideo = it.ShowVideo
	s.State.HasClip = it.HasClip
}

func (s *GameSession) advanceLocked() {
	switch s.State.Phase {
	case PhaseWelcome:
		s.startLocked()
	case PhaseCategoryTitle:
		if cat := s.curCat(); cat != nil && len(cat.Items) > 0 {
			s.enterCountdownLocked(0)
		} else {
			s.enterAwaitAnswersLocked()
		}
	case PhaseCountdown:
		s.enterPlayingLocked()
	case PhasePlaying:
		s.enterThinkingLocked()
	case PhaseThinking:

		cat := s.curCat()
		if cat != nil && s.State.ItemIndex < len(cat.Items)-1 {
			s.State.ItemIndex++
			s.enterPlayingLocked()
		} else {
			s.enterAwaitAnswersLocked()
		}
	case PhaseAwaitAnswers:
		s.enterAnswersLocked()
	case PhaseAnswers:
		if s.State.CategoryIndex < len(s.Categories)-1 {
			s.State.CategoryIndex++
			s.enterCategoryTitleLocked()
		} else {
			s.enterFinishedLocked()
		}
	case PhaseFinished:

	}
}

func (s *GameSession) backLocked() {
	switch s.State.Phase {
	case PhaseCountdown:
		s.enterCategoryTitleLocked()
	case PhasePlaying, PhaseThinking:
		if s.State.ItemIndex > 0 {
			s.State.ItemIndex--
			s.enterPlayingLocked()
		} else {
			s.enterCountdownLocked(0)
		}
	case PhaseAwaitAnswers, PhaseAnswers:

		if cat := s.curCat(); cat != nil && len(cat.Items) > 0 {
			s.State.ItemIndex = len(cat.Items) - 1
			s.enterPlayingLocked()
		} else {
			s.enterCategoryTitleLocked()
		}
	}
}

func (s *GameSession) seekLocked(deltaMs int64) {
	if s.State.PhaseDuration <= 0 {
		return
	}
	var elapsed int64
	if s.State.Paused {
		elapsed = s.State.PhaseDuration - s.State.RemainingMs
	} else {
		elapsed = s.State.PhaseDuration - (s.State.PhaseEndsAt - nowMs())
	}
	newElapsed := elapsed + deltaMs
	if newElapsed < 0 {
		newElapsed = 0
	}
	if newElapsed > s.State.PhaseDuration {
		newElapsed = s.State.PhaseDuration
	}
	remaining := s.State.PhaseDuration - newElapsed

	if s.State.Paused {
		s.State.RemainingMs = remaining
		return
	}
	if remaining <= 0 {
		s.advanceLocked()
		return
	}
	s.startTimerLocked(remaining, s.State.PhaseDuration)
}

func (s *GameSession) pauseLocked() {
	if s.State.Paused || s.State.PhaseEndsAt == 0 {
		return
	}
	remaining := s.State.PhaseEndsAt - nowMs()
	if remaining < 0 {
		remaining = 0
	}
	s.stopTimerLocked()
	s.State.Paused = true
	s.State.RemainingMs = remaining
	s.State.PhaseEndsAt = 0
}

func (s *GameSession) resumeLocked() {
	if !s.State.Paused {
		return
	}
	if s.State.RemainingMs <= 0 {
		s.State.Paused = false
		s.advanceLocked()
		return
	}
	s.startTimerLocked(s.State.RemainingMs, s.State.PhaseDuration)
}

func nowMs() int64 { return time.Now().UnixMilli() }

func (s *GameSession) startTimerLocked(remainingMs, totalMs int64) {
	s.stopTimerLocked()
	s.timerGen++
	gen := s.timerGen

	s.State.PhaseDuration = totalMs
	s.State.RemainingMs = remainingMs
	s.State.Paused = false

	if remainingMs <= 0 {
		s.State.PhaseEndsAt = 0
		s.advanceLocked()
		return
	}

	s.State.PhaseEndsAt = nowMs() + remainingMs
	s.timer = time.AfterFunc(time.Duration(remainingMs)*time.Millisecond, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if gen != s.timerGen {
			return
		}
		s.advanceLocked()
		s.broadcastLocked()
	})
}

func (s *GameSession) stopTimerLocked() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.timerGen++
}

func (s *GameSession) clearTimerLocked() {
	s.stopTimerLocked()
	s.State.PhaseEndsAt = 0
	s.State.PhaseDuration = 0
	s.State.RemainingMs = 0
	s.State.Paused = false
}
