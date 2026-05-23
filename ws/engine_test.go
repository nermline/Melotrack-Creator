package ws

import (
	"testing"

	"github.com/nermline/Melotrack-Creator/internal/models"
)

func TestClipDurationMs(t *testing.T) {
	cases := []struct {
		name string
		v    models.Video
		want int64
	}{
		{"end greater than start", models.Video{StartTime: 5, EndTime: 15}, 10000},
		{"no end uses media duration", models.Video{StartTime: 2, Media: models.Media{Duration: 12}}, 10000},
		{"no end no media", models.Video{}, 0},
		{"end <= start falls back to media", models.Video{StartTime: 10, EndTime: 5, Media: models.Media{Duration: 30}}, 20000},
		{"start beyond media duration", models.Video{StartTime: 40, Media: models.Media{Duration: 30}}, 0},
		{"exact zero everything", models.Video{StartTime: 0, EndTime: 0}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clipDurationMs(tc.v); got != tc.want {
				t.Errorf("clipDurationMs(%+v) = %d, want %d", tc.v, got, tc.want)
			}
		})
	}
}

func newTestSession() *GameSession {
	s := &GameSession{
		ProjectID: "1",
		Clients:   make(map[*GameClient]bool),
		State:     GameState{ProjectID: "1", Phase: PhaseWelcome},
	}
	s.Categories = []gameCategory{
		{ID: 1, Title: "A", Items: []gameItem{
			{ID: 10, HasClip: true, ShowVideo: true, DurationMs: 3000},
			{ID: 11, HasClip: false, DurationMs: 4000},
		}},
		{ID: 2, Title: "B", Items: []gameItem{
			{ID: 20, DurationMs: 2000},
		}},
	}
	s.State.TotalCategories = len(s.Categories)
	return s
}

func TestAdvanceFullFlow(t *testing.T) {
	s := newTestSession()
	defer s.stopTimerLocked()

	steps := []struct {
		wantPhase   string
		wantCatIdx  int
		wantItemIdx int
		checkItemID bool
		wantItemID  uint
	}{
		{PhaseCategoryTitle, 0, 0, false, 0},
		{PhaseCountdown, 0, 0, false, 0},
		{PhasePlaying, 0, 0, true, 10},
		{PhaseThinking, 0, 0, false, 0},
		{PhasePlaying, 0, 1, true, 11},
		{PhaseThinking, 0, 1, false, 0},
		{PhaseAwaitAnswers, 0, 1, false, 0},
		{PhaseAnswers, 0, 1, false, 0},
		{PhaseCategoryTitle, 1, 0, false, 0},
		{PhaseCountdown, 1, 0, false, 0},
		{PhasePlaying, 1, 0, true, 20},
		{PhaseThinking, 1, 0, false, 0},
		{PhaseAwaitAnswers, 1, 0, false, 0},
		{PhaseAnswers, 1, 0, false, 0},
		{PhaseFinished, 1, 0, false, 0},
		{PhaseFinished, 1, 0, false, 0},
	}

	for i, st := range steps {
		s.advanceLocked()
		if s.State.Phase != st.wantPhase {
			t.Fatalf("step %d: phase = %q, want %q", i, s.State.Phase, st.wantPhase)
		}
		if s.State.CategoryIndex != st.wantCatIdx {
			t.Errorf("step %d: categoryIndex = %d, want %d", i, s.State.CategoryIndex, st.wantCatIdx)
		}
		if s.State.ItemIndex != st.wantItemIdx {
			t.Errorf("step %d: itemIndex = %d, want %d", i, s.State.ItemIndex, st.wantItemIdx)
		}
		if st.checkItemID && s.State.ItemID != st.wantItemID {
			t.Errorf("step %d: itemID = %d, want %d", i, s.State.ItemID, st.wantItemID)
		}
	}
}

func TestAdvancePlayingResolvesFlags(t *testing.T) {
	s := newTestSession()
	defer s.stopTimerLocked()
	s.advanceLocked()
	s.advanceLocked()
	s.advanceLocked()
	if !s.State.ShowVideo || !s.State.HasClip {
		t.Errorf("item0 should have ShowVideo+HasClip, got show=%v clip=%v", s.State.ShowVideo, s.State.HasClip)
	}
	s.advanceLocked()
	s.advanceLocked()
	if s.State.ShowVideo || s.State.HasClip {
		t.Errorf("item1 should have no video/clip, got show=%v clip=%v", s.State.ShowVideo, s.State.HasClip)
	}
}

func TestBackTransitions(t *testing.T) {
	s := newTestSession()
	defer s.stopTimerLocked()

	for i := 0; i < 5; i++ {
		s.advanceLocked()
	}
	if s.State.Phase != PhasePlaying || s.State.ItemIndex != 1 {
		t.Fatalf("setup: phase=%s idx=%d", s.State.Phase, s.State.ItemIndex)
	}
	s.backLocked()
	if s.State.Phase != PhasePlaying || s.State.ItemIndex != 0 {
		t.Errorf("back from item1: phase=%s idx=%d", s.State.Phase, s.State.ItemIndex)
	}
	s.backLocked()
	if s.State.Phase != PhaseCountdown {
		t.Errorf("back from item0: phase=%s, want countdown", s.State.Phase)
	}
	s.backLocked()
	if s.State.Phase != PhaseCategoryTitle {
		t.Errorf("back from countdown: phase=%s, want category_title", s.State.Phase)
	}
}

func TestSeekClamping(t *testing.T) {
	s := newTestSession()
	defer s.stopTimerLocked()
	s.advanceLocked()
	s.advanceLocked()
	if s.State.PhaseDuration != countdownDurationMs {
		t.Fatalf("expected countdown duration, got %d", s.State.PhaseDuration)
	}

	s.seekLocked(-100000)
	if s.State.Phase != PhaseCountdown {
		t.Errorf("seek backward changed phase to %s", s.State.Phase)
	}

	s.seekLocked(100000)
	if s.State.Phase != PhasePlaying {
		t.Errorf("seek past end should advance to playing, got %s", s.State.Phase)
	}
}

func TestSeekNoTimerPhase(t *testing.T) {
	s := newTestSession()
	defer s.stopTimerLocked()

	s.State.Phase = PhaseAwaitAnswers
	s.clearTimerLocked()
	s.seekLocked(5000)
	if s.State.Phase != PhaseAwaitAnswers {
		t.Errorf("seek on timerless phase changed phase to %s", s.State.Phase)
	}
}

func TestPauseResume(t *testing.T) {
	s := newTestSession()
	defer s.stopTimerLocked()
	s.advanceLocked()
	s.advanceLocked()
	s.pauseLocked()
	if !s.State.Paused {
		t.Error("expected paused")
	}
	if s.State.PhaseEndsAt != 0 {
		t.Error("paused: PhaseEndsAt should be 0")
	}
	if s.State.RemainingMs <= 0 {
		t.Error("paused: RemainingMs should be > 0")
	}
	s.resumeLocked()
	if s.State.Paused {
		t.Error("expected resumed")
	}
	if s.State.Phase != PhaseCountdown {
		t.Errorf("resume should keep phase countdown, got %s", s.State.Phase)
	}
}

func TestPauseNoTimerIsNoop(t *testing.T) {
	s := newTestSession()
	defer s.stopTimerLocked()
	s.State.Phase = PhaseAnswers
	s.clearTimerLocked()
	s.pauseLocked()
	if s.State.Paused {
		t.Error("pause on timerless phase should be no-op")
	}
}

func TestResetLocked(t *testing.T) {
	s := newTestSession()
	defer s.stopTimerLocked()
	s.advanceLocked()
	s.advanceLocked()
	s.resetLocked()
	if s.State.Phase != PhaseWelcome {
		t.Errorf("reset phase = %s, want welcome", s.State.Phase)
	}
	if s.State.TotalCategories != 2 {
		t.Errorf("reset should keep TotalCategories=2, got %d", s.State.TotalCategories)
	}
	if s.State.CategoryIndex != 0 || s.State.ItemIndex != 0 {
		t.Errorf("reset should zero indices")
	}
}

func TestStartEmptyProjectFinishes(t *testing.T) {
	s := &GameSession{
		ProjectID: "1",
		Clients:   make(map[*GameClient]bool),
		State:     GameState{Phase: PhaseWelcome},
	}
	defer s.stopTimerLocked()
	s.startLocked()
	if s.State.Phase != PhaseFinished {
		t.Errorf("starting empty project should finish, got %s", s.State.Phase)
	}
}

func TestCategoryWithNoItemsGoesToAwait(t *testing.T) {
	s := &GameSession{
		ProjectID: "1",
		Clients:   make(map[*GameClient]bool),
		State:     GameState{Phase: PhaseWelcome},
	}
	s.Categories = []gameCategory{{ID: 1, Title: "empty"}}
	s.State.TotalCategories = 1
	defer s.stopTimerLocked()

	s.advanceLocked()
	if s.State.Phase != PhaseCategoryTitle {
		t.Fatalf("phase = %s", s.State.Phase)
	}
	s.advanceLocked()
	if s.State.Phase != PhaseAwaitAnswers {
		t.Errorf("empty category should jump to await_answers, got %s", s.State.Phase)
	}
}
