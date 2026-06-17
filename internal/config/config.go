package config

import (
	"flag"
	"os"
)

type Config struct {
	ServerAddress    string
	BaseURL          string
	CollisionRetries int
	FileStoragePath  string
}

func Parse() Config {
	cfg := Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address to run HTTP server on")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base URL for shortened links")
	flag.IntVar(&cfg.CollisionRetries, "m", 5, "max code generation attempts (0 or -1 for unlimited)")
	flag.StringVar(&cfg.FileStoragePath, "f", "data.json", "file path for URL storage")
	flag.Parse()

	if envServerAddress := os.Getenv("SERVER_ADDRESS"); envServerAddress != "" {
		cfg.ServerAddress = envServerAddress
	}

	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}

	if envFileStoragePath := os.Getenv("FILE_STORAGE_PATH"); envFileStoragePath != "" {
		cfg.FileStoragePath = envFileStoragePath
	}

	return cfg
}
