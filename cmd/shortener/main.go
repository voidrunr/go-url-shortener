package main

import (
	"log"
	"net/http"

	"github.com/voidrunr/go-url-shortener/internal/config"
	"github.com/voidrunr/go-url-shortener/internal/database"
	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/repository"
	"github.com/voidrunr/go-url-shortener/internal/service"
)

func main() {
	cfg := config.Parse()
	repo, err := repository.New(cfg.FileStoragePath)
	if err != nil {
		log.Fatalf("Failed to create repository: %v", err)
	}
	svc := service.New(repo, cfg.BaseURL, cfg.CollisionRetries)

	opts := []handler.Option{}
	if cfg.DatabaseDSN != "" {
		db, err := database.Open(cfg.DatabaseDSN)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()
		opts = append(opts, handler.WithPinger(db))
	}
	hlr := handler.New(svc, opts...)

	log.Printf("Serving on %s", cfg.ServerAddress)

	err = http.ListenAndServe(cfg.ServerAddress, hlr.Router())
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
