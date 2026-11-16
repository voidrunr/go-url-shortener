package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// fileConfig mirrors the JSON configuration file. Pointer fields let the
// loader distinguish "key absent" from "key set to a zero value", so only
// explicitly present keys are applied.
type fileConfig struct {
	ServerAddress    *string `json:"server_address"`
	BaseURL          *string `json:"base_url"`
	CollisionRetries *int    `json:"collision_retries"`
	FileStoragePath  *string `json:"file_storage_path"`
	DatabaseDSN      *string `json:"database_dsn"`
	SecretKey        *string `json:"secret_key"`
	TokenTTL         *string `json:"token_ttl"`
	AuditFile        *string `json:"audit_file"`
	AuditURL         *string `json:"audit_url"`
	PprofAddress     *string `json:"pprof_address"`
	EnableHTTPS      *bool   `json:"enable_https"`
	TrustedSubnet    *string `json:"trusted_subnet"`
}

// loadFile reads and parses a JSON configuration file.
func loadFile(path string) (fileConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	var fc fileConfig
	if err := json.Unmarshal(raw, &fc); err != nil {
		return fileConfig{}, fmt.Errorf("parse config file %q: %w", path, err)
	}

	return fc, nil
}
