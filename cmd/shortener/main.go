package main

import (
	"log"
	"github.com/voidrunr/go-url-shortener/internal/config"
)

func main() {
	cfg := config.GetDefault()

	log.Printf("Serving on %s port %d", cfg.Addr, cfg.Port)
}
