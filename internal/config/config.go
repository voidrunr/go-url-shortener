package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ServerAddress    string
	BaseURL          string
	FileStoragePath  string
	DatabaseDSN      string
	SecretKey        string
	AuditFile        string
	AuditURL         string
	PprofAddress     string
	TrustedSubnet    string
	CollisionRetries int
	TokenTTL         time.Duration
	EnableHTTPS      bool
}

func defaultConfig() Config {
	return Config{
		ServerAddress:    "localhost:8080",
		BaseURL:          "http://localhost:8080",
		CollisionRetries: 5,
		SecretKey:        "ChangeThisStringImportant",
		TokenTTL:         24 * time.Hour,
	}
}

func Parse() (Config, error) {
	cfg := defaultConfig()

	var configPath string
	for _, opt := range options {
		opt.register(flag.CommandLine, &cfg)
	}
	flag.StringVar(&configPath, "c", "", "path to JSON config file")
	flag.StringVar(&configPath, "config", "", "path to JSON config file")
	flag.Parse()

	snapshot := cfg
	explicit := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	if val := os.Getenv("CONFIG"); val != "" {
		configPath = val
	}

	if configPath != "" {
		fields, err := loadFile(configPath)
		if err != nil {
			return cfg, err
		}
		for _, opt := range options {
			raw, ok := fields[opt.jsonName]
			if !ok || string(raw) == "null" {
				continue
			}
			if err := opt.applyFile(&cfg, raw); err != nil {
				return cfg, fmt.Errorf("parse %s %q from config file %q: %w", opt.jsonName, raw, configPath, err)
			}
		}
	}

	for _, opt := range options {
		if explicit[opt.flagName] {
			opt.restore(&cfg, snapshot)
		}
	}

	for _, opt := range options {
		if opt.envName == "" {
			continue
		}
		if value := os.Getenv(opt.envName); value != "" {
			if err := opt.applyEnv(&cfg, value); err != nil {
				return cfg, fmt.Errorf("parse %s %q: %w", opt.envName, value, err)
			}
		}
	}

	if cfg.EnableHTTPS {
		cfg.BaseURL = strings.Replace(cfg.BaseURL, "http://", "https://", 1)
	}

	return cfg, nil
}
