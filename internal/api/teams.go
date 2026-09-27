package api

import (
	"errors"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxTeams = 200

func (s *Server) teamsOf(pid uint) []teamDTO {
	var teams []store.Team
	s.db.Where("project_id = ?", pid).Order("created_at, id").Find(&teams)
	out := make([]teamDTO, 0, len(teams))
	for i := range teams {
		out = append(out, toTeamDTO(&teams[i]))
	}
	return out
}

// teamsChanged pushes the team list to editors and the welcome screen.
func (s *Server) teamsChanged(pid uint) {
	s.hub.Publish(pid, "teams", s.teamsOf(pid))
	s.games.Refresh(pid)
}

func (s *Server) listTeams(c *gin.Context) {
	pid, ok := s.projectID(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, s.teamsOf(pid))
}

func (s *Server) addTeam(pid uint, rawName string) (*store.Team, int, string) {
	name, ok := cleanTitle(rawName, 40)
	if !ok {
		return nil, http.StatusBadRequest, "Назва команди: від 1 до 40 символів"
	}
	var n int64
	s.db.Model(&store.Team{}).Where("project_id = ?", pid).Count(&n)
	if n >= maxTeams {
		return nil, http.StatusConflict, "Досягнуто ліміту команд"
	}
	t := store.Team{ProjectID: pid, Name: name, NameKey: store.TeamKey(name)}
	if err := s.db.Create(&t).Error; err != nil {
		if store.IsUniqueViolation(err) {
			return nil, http.StatusConflict, "Команда з такою назвою вже є"
		}
		return nil, http.StatusInternalServerError, "Внутрішня помилка сервера"
	}
	return &t, http.StatusCreated, ""
}

func (s *Server) createTeam(c *gin.Context) {
	pid, ok := s.projectID(c)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &in) {
		return
	}
	t, status, msg := s.addTeam(pid, in.Name)
	if t == nil {
		fail(c, status, "invalid", msg)
		return
	}
	s.teamsChanged(pid)
	c.JSON(status, toTeamDTO(t))
}

func (s *Server) loadTeam(c *gin.Context) (*store.Team, bool) {
	tid, ok := idParam(c, "tid")
	if !ok {
		return nil, false
	}
	var t store.Team
	if err := s.db.First(&t, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			notFound(c, "Команду")
		} else {
			failInternal(c, err)
		}
		return nil, false
	}
	return &t, true
}

func (s *Server) updateTeam(c *gin.Context) {
	t, ok := s.loadTeam(c)
	if !ok {
		return
	}
	var in struct {
		Name  *string  `json:"name"`
		Bonus *float64 `json:"bonus"`
	}
	if !bindJSON(c, &in) {
		return
	}
	upd := map[string]any{}
	if in.Name != nil {
		name, ok := cleanTitle(*in.Name, 40)
		if !ok {
			badRequest(c, "Назва команди: від 1 до 40 символів")
			return
		}
		upd["name"] = name
		upd["name_key"] = store.TeamKey(name)
	}
	if in.Bonus != nil {
		if math.Abs(*in.Bonus) > 1000 {
			badRequest(c, "Забагато бонусних балів")
			return
		}
		upd["bonus"] = math.Round(*in.Bonus*2) / 2
	}
	if len(upd) > 0 {
		if err := s.db.Model(t).Updates(upd).Error; err != nil {
			if store.IsUniqueViolation(err) {
				fail(c, http.StatusConflict, "duplicate", "Команда з такою назвою вже є")
				return
			}
			failInternal(c, err)
			return
		}
	}
	s.db.First(t, t.ID)
	s.teamsChanged(t.ProjectID)
	c.JSON(http.StatusOK, toTeamDTO(t))
}

func (s *Server) deleteTeam(c *gin.Context) {
	t, ok := s.loadTeam(c)
	if !ok {
		return
	}
	if err := s.db.Delete(&store.Team{}, t.ID).Error; err != nil {
		failInternal(c, err)
		return
	}
	s.teamsChanged(t.ProjectID)
	s.hub.Publish(t.ProjectID, "scores_reload", nil)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type scoreDTO struct {
	TeamID uint    `json:"team_id"`
	ItemID uint    `json:"item_id"`
	Points float64 `json:"points"`
}

func (s *Server) listScores(c *gin.Context) {
	pid, ok := s.projectID(c)
	if !ok {
		return
	}
	out := []scoreDTO{}
	err := s.db.Raw(`SELECT s.team_id, s.item_id, s.points FROM scores s
		JOIN teams t ON t.id = s.team_id WHERE t.project_id = ?`, pid).Scan(&out).Error
	if err != nil {
		failInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// putScore sets one team's points for one song. Several checkers can score in
// parallel: each cell is independent and every change is broadcast.
func (s *Server) putScore(c *gin.Context) {
	var in scoreDTO
	if !bindJSON(c, &in) {
		return
	}
	if in.Points < 0 || in.Points > 10 {
		badRequest(c, "Бали: від 0 до 10")
		return
	}
	in.Points = math.Round(in.Points*2) / 2
	var t store.Team
	if err := s.db.First(&t, in.TeamID).Error; err != nil {
		notFound(c, "Команду")
		return
	}
	itemProject, ok := s.projectOfItem(in.ItemID)
	if !ok || itemProject != t.ProjectID {
		notFound(c, "Пісню")
		return
	}
	sc := store.Score{TeamID: in.TeamID, ItemID: in.ItemID, Points: in.Points, UpdatedAt: time.Now()}
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "team_id"}, {Name: "item_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"points", "updated_at"}),
	}).Create(&sc).Error
	if err != nil {
		failInternal(c, err)
		return
	}
	s.hub.Publish(t.ProjectID, "score", in)
	c.JSON(http.StatusOK, in)
}

// ---- public team registration (QR code on the welcome screen) ----

func (s *Server) projectByJoinCode(c *gin.Context) (*store.Project, bool) {
	var p store.Project
	if err := s.db.Where("join_code = ?", c.Param("code")).First(&p).Error; err != nil {
		fail(c, http.StatusNotFound, "not_found", "Реєстрацію не знайдено. Перевірте посилання")
		return nil, false
	}
	return &p, true
}

func (s *Server) joinInfo(c *gin.Context) {
	p, ok := s.projectByJoinCode(c)
	if !ok {
		return
	}
	var names []string
	s.db.Model(&store.Team{}).Where("project_id = ?", p.ID).Order("created_at, id").Pluck("name", &names)
	c.JSON(http.StatusOK, gin.H{"title": p.Title, "open": p.RegistrationOpen, "teams": names, "theme": p.Theme})
}

func (s *Server) joinRegister(c *gin.Context) {
	p, ok := s.projectByJoinCode(c)
	if !ok {
		return
	}
	if !p.RegistrationOpen {
		fail(c, http.StatusForbidden, "closed", "Реєстрацію закрито — зверніться до організаторів")
		return
	}
	if !s.joins.allow(c.ClientIP()) {
		fail(c, http.StatusTooManyRequests, "rate_limited", "Забагато реєстрацій з цього пристрою. Зверніться до організаторів")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &in) {
		return
	}
	t, status, msg := s.addTeam(p.ID, in.Name)
	if t == nil {
		fail(c, status, "invalid", msg)
		return
	}
	s.teamsChanged(p.ID)
	c.JSON(http.StatusCreated, gin.H{"name": t.Name})
}

// joinLimiter allows a handful of self-registrations per IP per 10 minutes.
// Venues often share one Wi-Fi IP, so the limit is generous.
type joinLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newJoinLimiter() *joinLimiter { return &joinLimiter{hits: map[string][]time.Time{}} }

func (l *joinLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if now.Sub(t) < 10*time.Minute {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 30 {
		l.hits[ip] = kept
		return false
	}
	l.hits[ip] = append(kept, now)
	return true
}
