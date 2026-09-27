package game

import (
	"path/filepath"
	"testing"

	"github.com/nermline/Melotrack-Creator/internal/store"
)

func TestRestoreSavedPosition(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "g.db"), store.LegacyImport{})
	if err != nil {
		t.Fatal(err)
	}
	p := store.Project{Title: "P", Theme: "neon", ThinkSeconds: 10, JoinCode: store.NewJoinCode()}
	db.Omit("Categories", "Teams").Create(&p)
	c := store.Category{ProjectID: p.ID, Title: "C"}
	db.Omit("Items").Create(&c)

	cases := map[string]Phase{
		`{"phase":"answers","cat":0}`:   PhaseAnswers,
		`{"phase":"standings","cat":0}`: PhaseWelcome, // screen removed since; start over
		`{"phase":"playing","cat":7}`:   PhaseWelcome, // category no longer exists
		`not json`:                      PhaseWelcome,
	}
	for raw, want := range cases {
		db.Model(&p).Update("game_state", raw)
		ss := NewSessions(db)
		s, err := ss.Get(p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.View(false).Phase; got != want {
			t.Errorf("restore %s: phase %s, want %s", raw, got, want)
		}
		ss.Drop(p.ID)
	}
}
