package game

import (
	"testing"
)

type fakeSource struct {
	cats  []Category
	board []Standing
	think int64
}

func (f *fakeSource) Program() []Category     { return f.cats }
func (f *fakeSource) Leaderboard() []Standing { return f.board }
func (f *fakeSource) ThinkMs() int64          { return f.think }

type clock struct{ t int64 }

func (c *clock) now() int64 { return c.t }

func newTestEngine(items ...int) (*Engine, *fakeSource, *clock) {
	src := &fakeSource{think: 8000}
	for i, n := range items {
		c := Category{ID: uint(i + 1), Title: "cat"}
		for j := 0; j < n; j++ {
			c.Items = append(c.Items, Item{ID: uint(100*(i+1) + j), DurationMs: 12000, Answer: "a"})
		}
		src.cats = append(src.cats, c)
	}
	src.board = []Standing{{TeamID: 1, Name: "A", Points: 5, Rank: 1}, {TeamID: 2, Name: "B", Points: 3, Rank: 2}}
	clk := &clock{t: 1_000_000}
	return NewEngine(src, clk.now), src, clk
}

func expectPhase(t *testing.T, e *Engine, want Phase) {
	t.Helper()
	if e.Phase != want {
		t.Fatalf("phase = %s, want %s (cat=%d item=%d)", e.Phase, want, e.Cat, e.Item)
	}
}

func TestFullShow(t *testing.T) {
	e, _, _ := newTestEngine(2, 1)
	e.Apply("start", 0)
	expectPhase(t, e, PhaseCategory)
	e.Expire()
	expectPhase(t, e, PhaseCountdown)
	e.Expire()
	expectPhase(t, e, PhasePlaying)
	if e.Timer.Duration != 12000 {
		t.Fatalf("clip timer = %d, want 12000", e.Timer.Duration)
	}
	e.Expire()
	expectPhase(t, e, PhaseThinking)
	if e.Timer.Duration != 8000 {
		t.Fatalf("think timer = %d, want 8000", e.Timer.Duration)
	}
	e.Expire()
	expectPhase(t, e, PhasePlaying)
	if e.Item != 1 {
		t.Fatalf("item = %d, want 1", e.Item)
	}
	e.Expire() // thinking
	e.Expire() // last item done
	expectPhase(t, e, PhaseCollect)
	e.Expire() // untimed: nothing happens
	expectPhase(t, e, PhaseCollect)
	e.Next()
	expectPhase(t, e, PhaseAnswers)
	e.Next()
	expectPhase(t, e, PhaseCategory)
	if e.Cat != 1 {
		t.Fatalf("cat = %d, want 1", e.Cat)
	}
	e.Next() // countdown
	e.Next() // playing
	e.Next() // thinking
	e.Next() // collect
	e.Next() // answers
	e.Next()
	expectPhase(t, e, PhaseScoring)
	e.Next()
	expectPhase(t, e, PhaseResults)
	if e.Revealed() != 0 {
		t.Fatalf("revealed = %d, want 0", e.Revealed())
	}
	e.Next()
	e.Next()
	if e.Revealed() != 2 {
		t.Fatalf("revealed = %d, want 2", e.Revealed())
	}
	e.Next()
	expectPhase(t, e, PhaseFinished)
}

func TestEmptyCategorySkipsToCollect(t *testing.T) {
	e, _, _ := newTestEngine(0)
	e.Apply("start", 0)
	e.Next()
	expectPhase(t, e, PhaseCollect)
}

func TestNoTeamsFinishesAfterScoring(t *testing.T) {
	e, src, _ := newTestEngine(1)
	src.board = nil
	e.Apply("results", 0)
	expectPhase(t, e, PhaseScoring)
	e.Next()
	expectPhase(t, e, PhaseFinished)
}

func TestPauseSeekResume(t *testing.T) {
	e, _, clk := newTestEngine(1)
	e.Apply("start", 0)
	e.Next()
	e.Next() // playing, 12s
	clk.t += 2000
	if !e.Apply("pause", 0) {
		t.Fatal("pause refused")
	}
	if e.Remaining() != 10000 {
		t.Fatalf("remaining = %d, want 10000", e.Remaining())
	}
	clk.t += 60000 // time passes while paused
	e.Expire()     // a stale timer must not advance a paused phase
	expectPhase(t, e, PhasePlaying)
	e.Apply("seek", 5) // +5s → 5s left
	if e.Remaining() != 5000 {
		t.Fatalf("remaining after seek = %d, want 5000", e.Remaining())
	}
	e.Apply("seek", -20) // clamps to full duration
	if e.Remaining() != 12000 {
		t.Fatalf("remaining after back-seek = %d, want 12000", e.Remaining())
	}
	e.Apply("resume", 0)
	if e.Timer.EndsAt != clk.t+12000 {
		t.Fatalf("ends_at = %d, want %d", e.Timer.EndsAt, clk.t+12000)
	}
	e.Apply("seek", 30) // past the end → next phase
	expectPhase(t, e, PhaseThinking)
}

func TestBackNavigation(t *testing.T) {
	e, _, _ := newTestEngine(2, 2)
	e.Apply("goto", 1)
	expectPhase(t, e, PhaseCategory)
	e.Back()
	expectPhase(t, e, PhaseAnswers)
	if e.Cat != 0 {
		t.Fatalf("cat = %d, want 0", e.Cat)
	}
	e.Back()
	expectPhase(t, e, PhaseCollect)
	e.Back()
	expectPhase(t, e, PhasePlaying)
	if e.Item != 1 {
		t.Fatalf("item = %d, want 1 (last song)", e.Item)
	}
	e.Back()
	if e.Item != 0 {
		t.Fatalf("item = %d, want 0", e.Item)
	}
	e.Back()
	expectPhase(t, e, PhaseCountdown)
	e.Back()
	expectPhase(t, e, PhaseCategory)
	e.Back()
	expectPhase(t, e, PhaseWelcome)
}

func TestReplayAndNoIntermediateResults(t *testing.T) {
	e, _, clk := newTestEngine(1)
	e.Apply("start", 0)
	e.Next()
	e.Next()
	e.Next() // thinking
	clk.t += 3000
	e.Apply("replay", 0)
	expectPhase(t, e, PhasePlaying)
	if e.Remaining() != 12000 {
		t.Fatalf("replay should restart the clip, remaining = %d", e.Remaining())
	}
	e.Next()
	e.Next() // collect
	e.Next() // answers
	if e.Apply("standings", 0) {
		t.Fatal("results must only be shown at the end")
	}
	if len(e.Board) != 0 {
		t.Fatal("the leaderboard must not be loaded before the results")
	}
	e.Next() // last category → scoring
	expectPhase(t, e, PhaseScoring)
}

func TestGotoOutOfRange(t *testing.T) {
	e, _, _ := newTestEngine(1)
	if e.Apply("goto", 5) {
		t.Fatal("goto beyond the last category must be refused")
	}
	if e.Apply("nope", 0) {
		t.Fatal("unknown command accepted")
	}
}

func TestRevealSteps(t *testing.T) {
	cases := map[int][]int{
		0: nil,
		1: {1},
		3: {1, 2, 3},
		7: {4, 5, 6, 7},
	}
	for n, want := range cases {
		got := revealSteps(n)
		if len(got) != len(want) {
			t.Fatalf("revealSteps(%d) = %v, want %v", n, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("revealSteps(%d) = %v, want %v", n, got, want)
			}
		}
	}
}

func TestRankStandingsTies(t *testing.T) {
	got := RankStandings([]Standing{
		{Name: "C", Points: 4}, {Name: "A", Points: 9}, {Name: "B", Points: 4}, {Name: "D", Points: 1},
	})
	wantNames := []string{"A", "B", "C", "D"}
	wantRanks := []int{1, 2, 2, 4}
	for i := range got {
		if got[i].Name != wantNames[i] || got[i].Rank != wantRanks[i] {
			t.Fatalf("got %+v", got)
		}
	}
}
