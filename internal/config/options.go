package config

import (
	"encoding/json"
	"flag"
	"strconv"
	"time"
)

type option struct {
	register  func(*flag.FlagSet, *Config)
	applyFile func(*Config, json.RawMessage) error
	applyEnv  func(*Config, string) error
	restore   func(*Config, Config)
	value     func(*Config) any

	flagName string
	envName  string
	jsonName string
	usage    string
}

func stringOption(flagName, envName, jsonName, usage string, field func(*Config) *string) option {
	return option{
		flagName: flagName,
		envName:  envName,
		jsonName: jsonName,
		usage:    usage,
		register: func(fs *flag.FlagSet, cfg *Config) {
			fs.StringVar(field(cfg), flagName, *field(cfg), usage)
		},
		applyFile: func(cfg *Config, raw json.RawMessage) error {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return err
			}
			*field(cfg) = value
			return nil
		},
		applyEnv: func(cfg *Config, value string) error {
			*field(cfg) = value
			return nil
		},
		restore: func(dst *Config, src Config) {
			*field(dst) = *field(&src)
		},
		value: func(cfg *Config) any {
			return *field(cfg)
		},
	}
}

func intOption(flagName, envName, jsonName, usage string, field func(*Config) *int) option {
	return option{
		flagName: flagName,
		envName:  envName,
		jsonName: jsonName,
		usage:    usage,
		register: func(fs *flag.FlagSet, cfg *Config) {
			fs.IntVar(field(cfg), flagName, *field(cfg), usage)
		},
		applyFile: func(cfg *Config, raw json.RawMessage) error {
			var value int
			if err := json.Unmarshal(raw, &value); err != nil {
				return err
			}
			*field(cfg) = value
			return nil
		},
		applyEnv: func(cfg *Config, value string) error {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			*field(cfg) = parsed
			return nil
		},
		restore: func(dst *Config, src Config) {
			*field(dst) = *field(&src)
		},
		value: func(cfg *Config) any {
			return *field(cfg)
		},
	}
}

func durationOption(flagName, envName, jsonName, usage string, field func(*Config) *time.Duration) option {
	return option{
		flagName: flagName,
		envName:  envName,
		jsonName: jsonName,
		usage:    usage,
		register: func(fs *flag.FlagSet, cfg *Config) {
			fs.DurationVar(field(cfg), flagName, *field(cfg), usage)
		},
		applyFile: func(cfg *Config, raw json.RawMessage) error {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return err
			}
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			*field(cfg) = parsed
			return nil
		},
		applyEnv: func(cfg *Config, value string) error {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			*field(cfg) = parsed
			return nil
		},
		restore: func(dst *Config, src Config) {
			*field(dst) = *field(&src)
		},
		value: func(cfg *Config) any {
			return *field(cfg)
		},
	}
}

func boolOption(flagName, envName, jsonName, usage string, field func(*Config) *bool) option {
	return option{
		flagName: flagName,
		envName:  envName,
		jsonName: jsonName,
		usage:    usage,
		register: func(fs *flag.FlagSet, cfg *Config) {
			fs.BoolVar(field(cfg), flagName, *field(cfg), usage)
		},
		applyFile: func(cfg *Config, raw json.RawMessage) error {
			var value bool
			if err := json.Unmarshal(raw, &value); err != nil {
				return err
			}
			*field(cfg) = value
			return nil
		},
		applyEnv: func(cfg *Config, value string) error {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			*field(cfg) = parsed
			return nil
		},
		restore: func(dst *Config, src Config) {
			*field(dst) = *field(&src)
		},
		value: func(cfg *Config) any {
			return *field(cfg)
		},
	}
}

var options = []option{
	stringOption("a", "SERVER_ADDRESS", "server_address", "address to run HTTP server on",
		func(c *Config) *string { return &c.ServerAddress }),
	stringOption("b", "BASE_URL", "base_url", "base URL for shortened links",
		func(c *Config) *string { return &c.BaseURL }),
	intOption("m", "", "collision_retries", "max code generation attempts (0 or -1 for unlimited)",
		func(c *Config) *int { return &c.CollisionRetries }),
	stringOption("f", "FILE_STORAGE_PATH", "file_storage_path", "file path for URL storage",
		func(c *Config) *string { return &c.FileStoragePath }),
	stringOption("d", "DATABASE_DSN", "database_dsn", "database DSN",
		func(c *Config) *string { return &c.DatabaseDSN }),
	stringOption("secret-key", "SECRET_KEY", "secret_key", "secret key for signing auth cookies",
		func(c *Config) *string { return &c.SecretKey }),
	durationOption("token-ttl", "TOKEN_TTL", "token_ttl", "auth token lifetime",
		func(c *Config) *time.Duration { return &c.TokenTTL }),
	stringOption("audit-file", "AUDIT_FILE", "audit_file", "path to file sink for audit events",
		func(c *Config) *string { return &c.AuditFile }),
	stringOption("audit-url", "AUDIT_URL", "audit_url", "URL of remote sink for audit events",
		func(c *Config) *string { return &c.AuditURL }),
	stringOption("pprof-address", "PPROF_ADDRESS", "pprof_address", "address for the pprof debug server (empty = disabled)",
		func(c *Config) *string { return &c.PprofAddress }),
	boolOption("s", "ENABLE_HTTPS", "enable_https", "enable HTTPS with an in-memory self-signed certificate",
		func(c *Config) *bool { return &c.EnableHTTPS }),
	stringOption("t", "TRUSTED_SUBNET", "trusted_subnet", "trusted subnet in CIDR notation for /api/internal/stats (empty = deny all)",
		func(c *Config) *string { return &c.TrustedSubnet }),
}
