package main

import (
	"log"
	"net/http"

	"github.com/voidrunr/go-url-shortener/internal/config"
	"github.com/voidrunr/go-url-shortener/internal/handler"
	"github.com/voidrunr/go-url-shortener/internal/repository"
	"github.com/voidrunr/go-url-shortener/internal/service"
)

func main() {
	cfg := config.Parse()
	repo := repository.New()
	svc := service.New(repo, cfg.BaseURL)
	hlr := handler.New(svc)

	log.Printf("Serving on %s", cfg.ServerAddress)

	err := http.ListenAndServe(cfg.ServerAddress, hlr.Router())
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
