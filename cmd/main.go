package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/lrstanley/go-ytdlp"
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

	db, err := database.InitDB(cfg.DBLocation, cfg.AdminPassword, cfg.EditorPassword, cfg.OperatorPassword)
	if err != nil {
		log.Fatalf("main(): %v", err)
	}

	authMiddleware, err := auth.SetupAuthMiddleware(db, cfg.JWTSecret)
	if err != nil {
		log.Fatalf("main(): %v", err)
	}

	ytdlp.MustInstallAll(context.Background())

	r := gin.Default()

	routes.Setup(r, db, authMiddleware)

	addr := cfg.ListenAddr()
	log.Printf("main(): сервер слухає на %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("main(): %v", err)
	}
}
