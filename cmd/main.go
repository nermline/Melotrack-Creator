package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/auth"
	"github.com/nermline/Melotrack-Creator/internal/config"
	"github.com/nermline/Melotrack-Creator/internal/database"
	"github.com/nermline/Melotrack-Creator/internal/routes"
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

	routes.Setup(r, db, authMiddleware)

	if err := r.Run(":80"); err != nil {
		log.Fatalf("main(): %v", err)
	}

}
