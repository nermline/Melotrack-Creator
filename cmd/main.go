package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/auth"
	"github.com/nermline/Melotrack-Creator/internal/config"
	"github.com/nermline/Melotrack-Creator/internal/database"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("main(): %v", err)
	}

	db, err := database.InitDB(cfg.DBLocation)
	if err != nil {
		log.Fatalf("main(): %v", err)
	}

	authMiddleware, err := auth.SetupAuthMiddleware(db, cfg.JWTSecret)
	if err != nil {
		log.Fatalf("main(): %v", err)
	}

	r := gin.Default()

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

		// ТУТ БУДЕ ВАША БІЗНЕС-ЛОГІКА (Проєкти, Відео, FFMPEG)
		// api.POST("/projects", handlers.CreateProject(db))

		api.POST("/logout", authMiddleware.LogoutHandler)
	}

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("main(): %v", err)
	}
}
