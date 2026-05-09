package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	melotrackcreator "github.com/nermline/Melotrack-Creator/pkg"

	_ "modernc.org/sqlite"
)

func testHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "authorized",
		})
	}
}

func main() {
	db, err := melotrackcreator.InitDB()
	if err != nil {
		log.Fatalf("[FATAL] main: %v", err)
	}

	if err = melotrackcreator.SetupPassword(db); err != nil {
		log.Fatalf("[FATAL] main: %v", err)
	}

	secret, err := melotrackcreator.GetJWTSecret()
	if err != nil {
		log.Fatalf("[FATAL] main: %v", err)
	}

	r := gin.Default()

	r.Static("/static", "./static")
	r.GET("/admin", func(c *gin.Context) { c.File("./static/index.html") })

	r.GET("/login/status", melotrackcreator.LoginStatusHandler(db))
	r.POST("/login", melotrackcreator.LoginHandler(db, secret))
	r.POST("/refresh", melotrackcreator.RefreshHandler(db, secret))

	authGroup := r.Group("/")
	authGroup.Use(melotrackcreator.AuthMiddleware(db, secret))
	{
		authGroup.GET("/admin/sessions", melotrackcreator.ListSessionsHandler(db))
		authGroup.POST("/admin/sessions/delete/:id", melotrackcreator.DeleteSessionHandler(db))

		authGroup.POST("/logout", melotrackcreator.LogoutHandler(db))
		authGroup.GET("/test", testHandler())
	}

	r.Run()
}
