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
	DatabaseDSN      string
	SecretKey        string
}

func Parse() Config {
	cfg := Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address to run HTTP server on")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base URL for shortened links")
	flag.IntVar(&cfg.CollisionRetries, "m", 5, "max code generation attempts (0 or -1 for unlimited)")
	flag.StringVar(&cfg.FileStoragePath, "f", "", "file path for URL storage")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database DSN")
	flag.StringVar(&cfg.SecretKey, "secret-key", "", "secret key for signing auth cookies")
	flag.Parse()

	env := map[string]*string{
		"SERVER_ADDRESS":    &cfg.ServerAddress,
		"BASE_URL":          &cfg.BaseURL,
		"FILE_STORAGE_PATH": &cfg.FileStoragePath,
		"DATABASE_DSN":      &cfg.DatabaseDSN,
		"SECRET_KEY":        &cfg.SecretKey,
	}
	for name, target := range env {
		if val := os.Getenv(name); val != "" {
			*target = val
		}
	}

	return cfg
}
