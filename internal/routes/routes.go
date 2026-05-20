package routes

import (
	"net/http"

	jwt "github.com/appleboy/gin-jwt/v3"
	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/handlers"
	"gorm.io/gorm"
)

func Setup(r *gin.Engine, db *gorm.DB, authMiddleware *jwt.GinJWTMiddleware) {
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

		api.GET("/projects/:pid/categories", handlers.GetCategories(db))
		api.POST("/projects/:pid/categories", handlers.CreateCategory(db))
		api.PUT("/projects/:pid/categories/:cid", handlers.UpdateCategory(db))
		api.DELETE("/projects/:pid/categories/:cid", handlers.DeleteCategory(db))

		api.GET("/projects/:pid/categories/:cid/items", handlers.GetQuizItems(db))
		api.POST("/projects/:pid/categories/:cid/items", handlers.CreateQuizItem(db))
		api.PUT("/projects/:pid/categories/:cid/items/:iid", handlers.UpdateQuizItem(db))
		api.DELETE("/projects/:pid/categories/:cid/items/:iid", handlers.DeleteQuizItem(db))

		api.POST("/logout", authMiddleware.LogoutHandler)
	}

	protectedMedia.Use(authMiddleware.MiddlewareFunc())
	{
		protectedMedia.StaticFS("/media", http.Dir("./downloads/processed"))
		protectedMedia.StaticFS("/raw", http.Dir("./downloads/raw"))
	}
}
