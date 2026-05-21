package routes

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	jwt "github.com/appleboy/gin-jwt/v3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/handlers"
	"github.com/nermline/Melotrack-Creator/ws"
	"gorm.io/gorm"
)

func Setup(r *gin.Engine, db *gorm.DB, authMiddleware *jwt.GinJWTMiddleware) {
	r.Use(cors.New(cors.Config{
		// Додай сюди порти, на яких крутитиметься твій локальний фронтенд
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	hub := ws.NewHub()

	r.POST("/login", authMiddleware.LoginHandler)
	r.GET("/refresh", authMiddleware.RefreshHandler)

	api := r.Group("/api")
	protectedMedia := r.Group("/")

	api.Use(authMiddleware.MiddlewareFunc())
	{

		api.GET("/projects", handlers.GetProjects(db))
		api.GET("/projects/:pid", handlers.GetProjectByID(db))
		api.POST("/projects", handlers.CreateProject(db))
		api.PUT("/projects/:pid", handlers.UpdateProject(db))
		api.DELETE("/projects/:pid", handlers.DeleteProject(db))

		api.GET("/projects/:pid/ws", ws.ServeGameWS(db, hub))

		api.GET("/projects/:pid/categories", handlers.GetCategories(db))
		api.POST("/projects/:pid/categories", handlers.CreateCategory(db))
		api.PUT("/projects/:pid/categories/:cid", handlers.UpdateCategory(db))
		api.DELETE("/projects/:pid/categories/:cid", handlers.DeleteCategory(db))

		api.GET("/categories/:cid/ws", ws.ServeEditorWS(hub))

		api.GET("/projects/:pid/categories/:cid/items", handlers.GetQuizItems(db))
		api.POST("/projects/:pid/categories/:cid/items", handlers.CreateQuizItem(db, hub))
		api.POST("/projects/:pid/categories/:cid/items/:iid/render", handlers.RenderQuizItem(db, hub))
		api.POST("/projects/:pid/categories/:cid/items/:iid/image", handlers.UploadAnswerImage(db, hub))
		api.DELETE("/projects/:pid/categories/:cid/items/:iid/image", handlers.DeleteQuizItemImage(db, hub))
		api.PUT("/projects/:pid/categories/:cid/items/:iid", handlers.UpdateQuizItem(db, hub))
		api.DELETE("/projects/:pid/categories/:cid/items/:iid", handlers.DeleteQuizItem(db, hub))

		api.POST("/logout", authMiddleware.LogoutHandler)
	}

	protectedMedia.Use(authMiddleware.MiddlewareFunc())
	{
		protectedMedia.StaticFS("/media", http.Dir("./downloads/processed"))
		protectedMedia.StaticFS("/raw", http.Dir("./downloads/raw"))
		protectedMedia.StaticFS("/answers", http.Dir("./downloads/answers"))
	}

	serveSPA(r, "./frontend/dist")
}

// serveSPA роздає зібраний React-фронтенд із distDir (якщо він існує) з тієї ж
// адреси, що й API (:8080). Завдяки єдиному origin зникають CORS-проблеми й
// застосунок одразу доступний з інших пристроїв локальної мережі.
// Усі невідомі (не-API) шляхи віддають index.html для клієнтської маршрутизації.
func serveSPA(r *gin.Engine, distDir string) {
	index := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		return // фронтенд ще не зібраний — у dev використовується Vite
	}

	// Префікси, які обслуговує бекенд і які не можна підміняти на index.html.
	apiPrefixes := []string{"/api", "/login", "/refresh", "/media", "/raw", "/answers"}

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		for _, pref := range apiPrefixes {
			if p == pref || strings.HasPrefix(p, pref+"/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
		}

		// Якщо це реальний файл усередині dist (asset, favicon тощо) — віддаємо його.
		clean := filepath.Clean(strings.TrimPrefix(p, "/"))
		if clean != "." && !strings.HasPrefix(clean, "..") {
			f := filepath.Join(distDir, clean)
			if fi, err := os.Stat(f); err == nil && !fi.IsDir() {
				c.File(f)
				return
			}
		}

		// Інакше — SPA-маршрут, віддаємо index.html.
		c.File(index)
	})
}
