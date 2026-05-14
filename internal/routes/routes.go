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

	api.Use(authMiddleware.MiddlewareFunc())
	{
		api.GET("/profile", func(c *gin.Context) {
			user, _ := c.Get("id")
			c.JSON(http.StatusOK, gin.H{
				"message": "Authorized",
				"user":    user,
			})
		})

		api.GET("/projects", handlers.GetProjects(db))
		api.GET("/projects/:id", handlers.GetProjectByID(db))
		api.POST("/projects", handlers.CreateProject(db))
		api.PUT("/projects/:id", handlers.UpdateProject(db))
		api.DELETE("/projects/:id", handlers.DeleteProject(db))

		api.GET("/projects/:id/categories", handlers.GetCategories(db))
		api.POST("/projects/:id/categories", handlers.CreateCategory(db))
		api.PUT("/categories/:id", handlers.UpdateCategory(db))
		api.DELETE("/categories/:id", handlers.DeleteCategory(db))

		api.GET("/categories/:id/items", handlers.GetQuizItems(db))
		api.POST("/categories/:id/items", handlers.CreateQuizItem(db))
		api.PUT("/items/:id", handlers.UpdateQuizItem(db))
		api.DELETE("/items/:id", handlers.DeleteQuizItem(db))

		// api.GET("/pagers", handlers.GetPagers(db))
		// api.POST("/pagers", handlers.CreatePager(db))
		// api.PUT("/pagers/:id", handlers.UpdatePager(db))
		// api.DELETE("/pagers/:id", handlers.DeletePager(db))

		// api.GET("/media", handlers.GetMedia(db))
		// api.POST("/media", handlers.CreatePager(db))
		// api.PUT("/media/:id", handlers.UpdatePager(db))
		// api.DELETE("/media/:id", handlers.DeletePager(db))

		api.POST("/logout", authMiddleware.LogoutHandler)

	}
}
