package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
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
	CollisionRetries int
	TokenTTL         time.Duration
	EnableHTTPS      bool
}

func Parse() (Config, error) {
	cfg := Config{}

	var configPath string
	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address to run HTTP server on")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base URL for shortened links")
	flag.IntVar(&cfg.CollisionRetries, "m", 5, "max code generation attempts (0 or -1 for unlimited)")
	flag.StringVar(&cfg.FileStoragePath, "f", "", "file path for URL storage")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "database DSN")
	flag.StringVar(&cfg.SecretKey, "secret-key", "ChangeThisStringImportant", "secret key for signing auth cookies")
	flag.DurationVar(&cfg.TokenTTL, "token-ttl", 24*time.Hour, "auth token lifetime")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "path to file sink for audit events")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "URL of remote sink for audit events")
	flag.StringVar(&cfg.PprofAddress, "pprof-address", "", "address for the pprof debug server (empty = disabled)")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "enable HTTPS with an in-memory self-signed certificate")
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
		if err := applyFile(&cfg, configPath); err != nil {
			return cfg, err
		}
	}

	// Explicitly passed flags take priority over the config file values.
	reapply := map[string]func(){
		"a":             func() { cfg.ServerAddress = snapshot.ServerAddress },
		"b":             func() { cfg.BaseURL = snapshot.BaseURL },
		"m":             func() { cfg.CollisionRetries = snapshot.CollisionRetries },
		"f":             func() { cfg.FileStoragePath = snapshot.FileStoragePath },
		"d":             func() { cfg.DatabaseDSN = snapshot.DatabaseDSN },
		"secret-key":    func() { cfg.SecretKey = snapshot.SecretKey },
		"token-ttl":     func() { cfg.TokenTTL = snapshot.TokenTTL },
		"audit-file":    func() { cfg.AuditFile = snapshot.AuditFile },
		"audit-url":     func() { cfg.AuditURL = snapshot.AuditURL },
		"pprof-address": func() { cfg.PprofAddress = snapshot.PprofAddress },
		"s":             func() { cfg.EnableHTTPS = snapshot.EnableHTTPS },
	}
	for name, apply := range reapply {
		if explicit[name] {
			apply()
		}
	}

	env := map[string]*string{
		"SERVER_ADDRESS":    &cfg.ServerAddress,
		"BASE_URL":          &cfg.BaseURL,
		"FILE_STORAGE_PATH": &cfg.FileStoragePath,
		"DATABASE_DSN":      &cfg.DatabaseDSN,
		"SECRET_KEY":        &cfg.SecretKey,
		"AUDIT_FILE":        &cfg.AuditFile,
		"AUDIT_URL":         &cfg.AuditURL,
		"PPROF_ADDRESS":     &cfg.PprofAddress,
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

	if val := os.Getenv("ENABLE_HTTPS"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return cfg, fmt.Errorf("parse ENABLE_HTTPS %q: %w", val, err)
		}
		cfg.EnableHTTPS = enabled
	}

	if cfg.EnableHTTPS {
		cfg.BaseURL = strings.Replace(cfg.BaseURL, "http://", "https://", 1)
	}

	return cfg, nil
}

func applyFile(cfg *Config, path string) error {
	fc, err := loadFile(path)
	if err != nil {
		return err
	}

	if fc.ServerAddress != nil {
		cfg.ServerAddress = *fc.ServerAddress
	}
	if fc.BaseURL != nil {
		cfg.BaseURL = *fc.BaseURL
	}
	if fc.CollisionRetries != nil {
		cfg.CollisionRetries = *fc.CollisionRetries
	}
	if fc.FileStoragePath != nil {
		cfg.FileStoragePath = *fc.FileStoragePath
	}
	if fc.DatabaseDSN != nil {
		cfg.DatabaseDSN = *fc.DatabaseDSN
	}
	if fc.SecretKey != nil {
		cfg.SecretKey = *fc.SecretKey
	}
	if fc.TokenTTL != nil {
		ttl, err := time.ParseDuration(*fc.TokenTTL)
		if err != nil {
			return fmt.Errorf("parse token_ttl %q from config file %q: %w", *fc.TokenTTL, path, err)
		}
		cfg.TokenTTL = ttl
	}
	if fc.AuditFile != nil {
		cfg.AuditFile = *fc.AuditFile
	}
	if fc.AuditURL != nil {
		cfg.AuditURL = *fc.AuditURL
	}
	if fc.PprofAddress != nil {
		cfg.PprofAddress = *fc.PprofAddress
	}
	if fc.EnableHTTPS != nil {
		cfg.EnableHTTPS = *fc.EnableHTTPS
	}

	return nil
}
