// Package api is the HTTP layer: JSON endpoints, file serving, the WebSocket
// entry point and the built frontend.
package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/auth"
	"github.com/nermline/Melotrack-Creator/internal/config"
	"github.com/nermline/Melotrack-Creator/internal/game"
	"github.com/nermline/Melotrack-Creator/internal/live"
	"github.com/nermline/Melotrack-Creator/internal/media"
	"gorm.io/gorm"
)

type Server struct {
	db    *gorm.DB
	cfg   *config.Config
	auth  *auth.Auth
	media *media.Manager
	yt    *media.YtDlp
	hub   *live.Hub
	games *game.Sessions
	joins *joinLimiter
}

func NewServer(db *gorm.DB, cfg *config.Config, a *auth.Auth, m *media.Manager, yt *media.YtDlp, hub *live.Hub, games *game.Sessions) *Server {
	s := &Server{db: db, cfg: cfg, auth: a, media: m, yt: yt, hub: hub, games: games, joins: newJoinLimiter()}
	m.SetNotifier(s)
	games.OnChange(hub.PublishShow)
	hub.SetHandlers(s.showView, s.showCommand)
	return s
}

func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), requestLog())
	r.MaxMultipartMemory = 32 << 20
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})

	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	api.POST("/login", s.login)
	api.POST("/logout", s.logout)
	api.GET("/public/join/:code", s.joinInfo)
	api.POST("/public/join/:code", s.joinRegister)

	priv := api.Group("", s.auth.Middleware())
	priv.GET("/session", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	priv.GET("/system", s.system)

	priv.GET("/projects", s.listProjects)
	priv.POST("/projects", s.createProject)
	priv.GET("/projects/:pid", s.getProject)
	priv.PATCH("/projects/:pid", s.updateProject)
	priv.DELETE("/projects/:pid", s.deleteProject)
	priv.POST("/projects/:pid/duplicate", s.duplicateProject)
	priv.GET("/projects/:pid/live", s.serveLive)

	priv.POST("/projects/:pid/categories", s.createCategory)
	priv.PUT("/projects/:pid/categories/order", s.orderCategories)
	priv.PATCH("/categories/:cid", s.updateCategory)
	priv.DELETE("/categories/:cid", s.deleteCategory)

	priv.POST("/categories/:cid/items", s.createItems)
	priv.POST("/categories/:cid/items/upload", s.createUploadItem)
	priv.PUT("/categories/:cid/items/order", s.orderItems)
	priv.PATCH("/items/:iid", s.updateItem)
	priv.DELETE("/items/:iid", s.deleteItem)
	priv.POST("/items/:iid/image", s.uploadImage)
	priv.DELETE("/items/:iid/image", s.deleteImage)
	priv.POST("/items/:iid/source", s.uploadSource)
	priv.POST("/items/:iid/retry", s.retryDownload)
	priv.POST("/items/:iid/render", s.forceRender)

	priv.GET("/projects/:pid/teams", s.listTeams)
	priv.POST("/projects/:pid/teams", s.createTeam)
	priv.PATCH("/teams/:tid", s.updateTeam)
	priv.DELETE("/teams/:tid", s.deleteTeam)
	priv.GET("/projects/:pid/scores", s.listScores)
	priv.PUT("/scores", s.putScore)

	files := r.Group("/files", s.auth.Middleware())
	st := s.media.Storage()
	files.GET("/source/:name", serveDir(st.SourceDir(), "private, max-age=3600"))
	files.GET("/thumbs/:name", serveDir(st.ThumbsDir(), "private, max-age=86400"))
	files.GET("/clips/:name", serveDir(st.ClipsDir(), "private, max-age=31536000, immutable"))
	files.GET("/images/:name", serveDir(st.ImagesDir(), "private, max-age=31536000, immutable"))

	s.serveSPA(r)
	return r
}

// serveDir serves a single generated file name from dir (no listings, no traversal).
func serveDir(dir, cacheControl string) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
			fail(c, http.StatusNotFound, "not_found", "Файл не знайдено")
			return
		}
		path := filepath.Join(dir, name)
		if fi, err := os.Stat(path); err != nil || fi.IsDir() {
			fail(c, http.StatusNotFound, "not_found", "Файл не знайдено")
			return
		}
		c.Header("Cache-Control", cacheControl)
		c.File(path)
	}
}

func (s *Server) serveSPA(r *gin.Engine) {
	dir := s.cfg.StaticDir
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err != nil {
		r.NoRoute(func(c *gin.Context) { fail(c, http.StatusNotFound, "not_found", "Не знайдено") })
		return
	}
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/files/") {
			fail(c, http.StatusNotFound, "not_found", "Не знайдено")
			return
		}
		clean := filepath.Clean("/" + p)
		if clean != "/" {
			f := filepath.Join(dir, filepath.FromSlash(clean))
			if fi, err := os.Stat(f); err == nil && !fi.IsDir() {
				if strings.HasPrefix(clean, "/assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				c.File(f)
				return
			}
		}
		c.Header("Cache-Control", "no-cache")
		c.File(index)
	})
}

// ---- helpers ----

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func fail(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": apiError{Code: code, Message: msg}})
}

func failInternal(c *gin.Context, err error) {
	_ = c.Error(err)
	fail(c, http.StatusInternalServerError, "internal", "Внутрішня помилка сервера")
}

func notFound(c *gin.Context, what string) {
	fail(c, http.StatusNotFound, "not_found", what+" не знайдено")
}

func badRequest(c *gin.Context, msg string) {
	fail(c, http.StatusBadRequest, "invalid", msg)
}

// idParam parses a positive integer path parameter or answers 404.
func idParam(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || v == 0 {
		fail(c, http.StatusNotFound, "not_found", "Не знайдено")
		return 0, false
	}
	return uint(v), true
}

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		badRequest(c, "Некоректні дані запиту")
		return false
	}
	return true
}
