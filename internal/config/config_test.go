package config

import (
	"flag"
	"os"
	"testing"
	"time"
)

func resetFlags(t *testing.T) {
	t.Helper()
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(origArgs[0], flag.ExitOnError)
	os.Args = []string{origArgs[0]}
	t.Cleanup(func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	})
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"SERVER_ADDRESS", "BASE_URL", "FILE_STORAGE_PATH", "DATABASE_DSN",
		"SECRET_KEY", "TOKEN_TTL", "AUDIT_FILE", "AUDIT_URL", "PPROF_ADDRESS",
	} {
		t.Setenv(name, "")
	}
}

func TestParseDefaults(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:8080" {
		t.Errorf("ServerAddress = %q, want localhost:8080", cfg.ServerAddress)
	}
	if cfg.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want http://localhost:8080", cfg.BaseURL)
	}
	if cfg.CollisionRetries != 5 {
		t.Errorf("CollisionRetries = %d, want 5", cfg.CollisionRetries)
	}
	if cfg.TokenTTL != 24*time.Hour {
		t.Errorf("TokenTTL = %v, want 24h", cfg.TokenTTL)
	}
	if cfg.SecretKey != "ChangeThisStringImportant" {
		t.Errorf("SecretKey = %q, want default", cfg.SecretKey)
	}
}

func TestParseFlags(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	os.Args = []string{
		"testsrv",
		"-a", "localhost:9999",
		"-b", "http://localhost:9999",
		"-m", "2",
		"-f", "/tmp/store.json",
		"-d", "postgres://u:p@h:5432/db?sslmode=disable",
		"-secret-key", "topsecret",
		"-token-ttl", "30m",
		"-audit-file", "/tmp/audit.log",
		"-pprof-address", "localhost:6060",
	}

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:9999" {
		t.Errorf("ServerAddress = %q", cfg.ServerAddress)
	}
	if cfg.BaseURL != "http://localhost:9999" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.CollisionRetries != 2 {
		t.Errorf("CollisionRetries = %d", cfg.CollisionRetries)
	}
	if cfg.FileStoragePath != "/tmp/store.json" {
		t.Errorf("FileStoragePath = %q", cfg.FileStoragePath)
	}
	if cfg.DatabaseDSN != "postgres://u:p@h:5432/db?sslmode=disable" {
		t.Errorf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	if cfg.SecretKey != "topsecret" {
		t.Errorf("SecretKey = %q", cfg.SecretKey)
	}
	if cfg.TokenTTL != 30*time.Minute {
		t.Errorf("TokenTTL = %v", cfg.TokenTTL)
	}
	if cfg.AuditFile != "/tmp/audit.log" {
		t.Errorf("AuditFile = %q", cfg.AuditFile)
	}
	if cfg.PprofAddress != "localhost:6060" {
		t.Errorf("PprofAddress = %q", cfg.PprofAddress)
	}
}

func TestParseEnvOverridesFlags(t *testing.T) {
	resetFlags(t)

	os.Args = []string{"testsrv", "-a", "localhost:1111"}
	t.Setenv("SERVER_ADDRESS", "localhost:2222")
	t.Setenv("BASE_URL", "https://env.example")

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:2222" {
		t.Errorf("ServerAddress = %q, env must override flag", cfg.ServerAddress)
	}
	if cfg.BaseURL != "https://env.example" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.PprofAddress != "" {
		t.Errorf("PprofAddress = %q, want empty", cfg.PprofAddress)
	}
}

func TestParseEnvAllFields(t *testing.T) {
	resetFlags(t)

	t.Setenv("SERVER_ADDRESS", "localhost:3333")
	t.Setenv("BASE_URL", "http://localhost:3333")
	t.Setenv("FILE_STORAGE_PATH", "/tmp/e.json")
	t.Setenv("DATABASE_DSN", "postgres://x")
	t.Setenv("SECRET_KEY", "envsecret")
	t.Setenv("TOKEN_TTL", "2h")
	t.Setenv("AUDIT_FILE", "/tmp/e-audit.log")
	t.Setenv("AUDIT_URL", "http://audit.example")
	t.Setenv("PPROF_ADDRESS", "localhost:7070")

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:3333" {
		t.Errorf("ServerAddress = %q", cfg.ServerAddress)
	}
	if cfg.FileStoragePath != "/tmp/e.json" {
		t.Errorf("FileStoragePath = %q", cfg.FileStoragePath)
	}
	if cfg.DatabaseDSN != "postgres://x" {
		t.Errorf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	if cfg.SecretKey != "envsecret" {
		t.Errorf("SecretKey = %q", cfg.SecretKey)
	}
	if cfg.TokenTTL != 2*time.Hour {
		t.Errorf("TokenTTL = %v", cfg.TokenTTL)
	}
	if cfg.AuditFile != "/tmp/e-audit.log" {
		t.Errorf("AuditFile = %q", cfg.AuditFile)
	}
	if cfg.AuditURL != "http://audit.example" {
		t.Errorf("AuditURL = %q", cfg.AuditURL)
	}
	if cfg.PprofAddress != "localhost:7070" {
		t.Errorf("PprofAddress = %q", cfg.PprofAddress)
	}
}

func TestParseInvalidTokenTTL(t *testing.T) {
	resetFlags(t)

	t.Setenv("TOKEN_TTL", "not-a-duration")

	if _, err := Parse(); err == nil {
		t.Fatal("Parse() error = nil, want error for invalid TOKEN_TTL")
	}
}
