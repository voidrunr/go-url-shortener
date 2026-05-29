package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/voidrunr/go-url-shortener/internal/config"
	"github.com/voidrunr/go-url-shortener/internal/repository"
	"github.com/voidrunr/go-url-shortener/internal/service"
	"github.com/voidrunr/go-url-shortener/internal/handler"
)

func main() {
	cfg := config.GetDefault()
	repo := repository.New()
	baseUrl := fmt.Sprintf("%s:%d", cfg.Addr, cfg.Port)
	svc := service.New(repo, baseUrl)
	hlr := handler.New(svc)

	log.Printf("Serving on %s port %d", cfg.Addr, cfg.Port)

	err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), hlr.Router());
	if  err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
