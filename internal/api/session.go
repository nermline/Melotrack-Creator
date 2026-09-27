package api

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/game"
	"github.com/nermline/Melotrack-Creator/internal/live"
)

func requestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.Request.URL.Path
		status := c.Writer.Status()
		if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/files/") && status < 400 {
			return
		}
		attrs := []any{"method", c.Request.Method, "path", path, "status", status,
			"took", time.Since(start).Round(time.Millisecond)}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "err", c.Errors.String())
		}
		if status >= 500 {
			slog.Error("request", attrs...)
		} else if strings.HasPrefix(path, "/api/") {
			slog.Debug("request", attrs...)
		}
	}
}

func (s *Server) login(c *gin.Context) {
	var in struct {
		Password string `json:"password"`
	}
	if !bindJSON(c, &in) {
		return
	}
	ok, wait := s.auth.Login(c, in.Password)
	if wait > 0 {
		fail(c, http.StatusTooManyRequests, "rate_limited",
			fmt.Sprintf("Забагато спроб. Спробуйте за %d хв.", int(math.Ceil(wait.Minutes()))))
		return
	}
	if !ok {
		fail(c, http.StatusUnauthorized, "bad_password", "Невірний пароль")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) logout(c *gin.Context) {
	s.auth.Logout(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) system(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	downloads, renders := s.media.QueueSizes()
	version, age := s.yt.Version(ctx)
	c.JSON(http.StatusOK, gin.H{
		"ytdlp_version":  version,
		"ytdlp_age_days": age,
		"proxy":          s.cfg.YtDlpProxy != "",
		"cookies":        s.cfg.YtDlpCookies != "",
		"clip_height":    s.cfg.ClipHeight,
		"queue":          gin.H{"downloads": downloads, "renders": renders},
	})
}

// ---- live connection and the show ----

func (s *Server) serveLive(c *gin.Context) {
	pid, ok := s.projectID(c)
	if !ok {
		return
	}
	role := live.Role(c.Query("role"))
	switch role {
	case live.RoleScreen, live.RoleRemote, live.RoleEditor:
	default:
		badRequest(c, "role має бути screen, remote або editor")
		return
	}
	s.hub.Serve(c.Writer, c.Request, pid, role)
}

type showView struct {
	game.View
	Devices map[live.Role]int `json:"devices,omitempty"`
}

func (s *Server) showView(projectID uint, host bool) (any, bool) {
	sess, err := s.games.Get(projectID)
	if err != nil {
		return nil, false
	}
	v := showView{View: sess.View(host)}
	if host {
		v.Devices = s.hub.Connected(projectID)
	}
	return v, true
}

func (s *Server) showCommand(projectID uint, action string, value float64) {
	sess, err := s.games.Get(projectID)
	if err != nil {
		return
	}
	sess.Command(action, value)
}
