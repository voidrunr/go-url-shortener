package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	ServerAddress    string
	BaseURL          string
	CollisionRetries int
	FileStoragePath  string
	DatabaseDSN      string
	SecretKey        string
	TokenTTL         time.Duration
}

func Parse() (Config, error) {
	cfg := Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address to run HTTP server on")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base URL for shortened links")
	flag.IntVar(&cfg.CollisionRetries, "m", 5, "max code generation attempts (0 or -1 for unlimited)")
	flag.StringVar(&cfg.FileStoragePath, "f", "", "file path for URL storage")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database DSN")
	flag.StringVar(&cfg.SecretKey, "secret-key", "ChangeThisStringImportant", "secret key for signing auth cookies")
	flag.DurationVar(&cfg.TokenTTL, "token-ttl", 24*time.Hour, "auth token lifetime")
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

	if val := os.Getenv("TOKEN_TTL"); val != "" {
		ttl, err := time.ParseDuration(val)
		if err != nil {
			return cfg, fmt.Errorf("parse TOKEN_TTL %q: %w", val, err)
		}
		cfg.TokenTTL = ttl
	}

	return cfg, nil
}