package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
		"ENABLE_HTTPS", "TRUSTED_SUBNET", "CONFIG",
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
	if cfg.EnableHTTPS {
		t.Error("EnableHTTPS = true, want false by default")
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

func TestParseEnableHTTPSFlag(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	os.Args = []string{"testsrv", "-s"}

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if !cfg.EnableHTTPS {
		t.Error("EnableHTTPS = false, want true with -s flag")
	}
	if cfg.BaseURL != "https://localhost:8080" {
		t.Errorf("BaseURL = %q, want https://localhost:8080 when HTTPS is enabled", cfg.BaseURL)
	}
}

func TestParseEnableHTTPSEnv(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"one", "1", true},
		{"false", "false", false},
		{"zero", "0", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetFlags(t)

			t.Setenv("ENABLE_HTTPS", tc.value)

			cfg, err := Parse()
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if cfg.EnableHTTPS != tc.want {
				t.Errorf("EnableHTTPS = %v, want %v for ENABLE_HTTPS=%q", cfg.EnableHTTPS, tc.want, tc.value)
			}
		})
	}
}

func TestParseEnvOverridesEnableHTTPSFlag(t *testing.T) {
	resetFlags(t)

	os.Args = []string{"testsrv", "-s"}
	t.Setenv("ENABLE_HTTPS", "false")

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.EnableHTTPS {
		t.Error("EnableHTTPS = true, env must override flag")
	}
}

func TestParseInvalidEnableHTTPS(t *testing.T) {
	resetFlags(t)

	t.Setenv("ENABLE_HTTPS", "garbage")

	if _, err := Parse(); err == nil {
		t.Fatal("Parse() error = nil, want error for invalid ENABLE_HTTPS")
	}
}

func TestParseHTTPSBaseURLSchemes(t *testing.T) {
	cases := []struct {
		name    string
		https   bool
		baseURL string
		want    string
	}{
		{"default without https", false, "", "http://localhost:8080"},
		{"explicit http with https", true, "http://short.example", "https://short.example"},
		{"explicit https with https", true, "https://short.example", "https://short.example"},
		{"explicit http without https", false, "http://short.example", "http://short.example"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetFlags(t)
			clearEnv(t)

			args := []string{"testsrv"}
			if tc.https {
				args = append(args, "-s")
			}
			if tc.baseURL != "" {
				args = append(args, "-b", tc.baseURL)
			}
			os.Args = args

			cfg, err := Parse()
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if cfg.BaseURL != tc.want {
				t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, tc.want)
			}
		})
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

const testConfigJSON = `{
    "server_address": "localhost:5555",
    "base_url": "http://short.example",
    "collision_retries": 3,
    "file_storage_path": "/tmp/file-store.json",
    "database_dsn": "postgres://file:dsn",
    "secret_key": "filesecret",
    "token_ttl": "1h",
    "audit_file": "/tmp/file-audit.log",
    "audit_url": "http://file-audit.example",
    "pprof_address": "localhost:7070",
    "enable_https": false
}`

func TestParseConfigFileAllOptions(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	path := writeConfigFile(t, testConfigJSON)
	os.Args = []string{"testsrv", "-c", path}

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:5555" {
		t.Errorf("ServerAddress = %q", cfg.ServerAddress)
	}
	if cfg.BaseURL != "http://short.example" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.CollisionRetries != 3 {
		t.Errorf("CollisionRetries = %d", cfg.CollisionRetries)
	}
	if cfg.FileStoragePath != "/tmp/file-store.json" {
		t.Errorf("FileStoragePath = %q", cfg.FileStoragePath)
	}
	if cfg.DatabaseDSN != "postgres://file:dsn" {
		t.Errorf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	if cfg.SecretKey != "filesecret" {
		t.Errorf("SecretKey = %q", cfg.SecretKey)
	}
	if cfg.TokenTTL != time.Hour {
		t.Errorf("TokenTTL = %v", cfg.TokenTTL)
	}
	if cfg.AuditFile != "/tmp/file-audit.log" {
		t.Errorf("AuditFile = %q", cfg.AuditFile)
	}
	if cfg.AuditURL != "http://file-audit.example" {
		t.Errorf("AuditURL = %q", cfg.AuditURL)
	}
	if cfg.PprofAddress != "localhost:7070" {
		t.Errorf("PprofAddress = %q", cfg.PprofAddress)
	}
	if cfg.EnableHTTPS {
		t.Error("EnableHTTPS = true, want false")
	}
}

func TestParseConfigFileFlagOverridesFile(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	path := writeConfigFile(t, testConfigJSON)
	os.Args = []string{"testsrv", "-c", path, "-a", "localhost:6666"}

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:6666" {
		t.Errorf("ServerAddress = %q, want flag value to beat config file", cfg.ServerAddress)
	}
	if cfg.BaseURL != "http://short.example" {
		t.Errorf("BaseURL = %q, want config file value", cfg.BaseURL)
	}
}

func TestParseConfigFileEnvOverridesFile(t *testing.T) {
	resetFlags(t)

	path := writeConfigFile(t, testConfigJSON)
	os.Args = []string{"testsrv", "-c", path}
	t.Setenv("SERVER_ADDRESS", "localhost:7777")
	t.Setenv("BASE_URL", "http://env.example")

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:7777" {
		t.Errorf("ServerAddress = %q, want env to beat config file", cfg.ServerAddress)
	}
	if cfg.BaseURL != "http://env.example" {
		t.Errorf("BaseURL = %q, want env to beat config file", cfg.BaseURL)
	}
}

func TestParseConfigFileFlagOverridesDefaultButFileBeatsDefaults(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	path := writeConfigFile(t, testConfigJSON)
	os.Args = []string{"testsrv", "-config", path}

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.CollisionRetries != 3 {
		t.Errorf("CollisionRetries = %d, want config file value to beat default", cfg.CollisionRetries)
	}
}

func TestParseConfigEnvPathOverridesFlag(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	pathFlag := writeConfigFile(t, `{"server_address": "localhost:1111"}`)
	pathEnv := writeConfigFile(t, `{"server_address": "localhost:2222"}`)
	os.Args = []string{"testsrv", "-c", pathFlag}
	t.Setenv("CONFIG", pathEnv)

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:2222" {
		t.Errorf("ServerAddress = %q, want CONFIG env path to win", cfg.ServerAddress)
	}
}

func TestParseConfigFileHTTPS(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	path := writeConfigFile(t, `{"base_url": "http://short.example", "enable_https": true}`)
	os.Args = []string{"testsrv", "-c", path}

	cfg, err := Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if !cfg.EnableHTTPS {
		t.Error("EnableHTTPS = false, want true from config file")
	}
	if cfg.BaseURL != "https://short.example" {
		t.Errorf("BaseURL = %q, want scheme replaced with https", cfg.BaseURL)
	}
}

func TestParseConfigFileMissing(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	os.Args = []string{"testsrv", "-c", "/nonexistent/config.json"}

	if _, err := Parse(); err == nil {
		t.Fatal("Parse() error = nil, want error for missing config file")
	}
}

func TestParseConfigFileInvalidJSON(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	path := writeConfigFile(t, `{"server_address": `)
	os.Args = []string{"testsrv", "-c", path}

	if _, err := Parse(); err == nil {
		t.Fatal("Parse() error = nil, want error for invalid JSON")
	}
}

func TestParseConfigFileInvalidTokenTTL(t *testing.T) {
	resetFlags(t)
	clearEnv(t)

	path := writeConfigFile(t, `{"token_ttl": "not-a-duration"}`)
	os.Args = []string{"testsrv", "-c", path}

	if _, err := Parse(); err == nil {
		t.Fatal("Parse() error = nil, want error for invalid token_ttl")
	}
}

func TestParseTrustedSubnet(t *testing.T) {
	t.Run("default empty", func(t *testing.T) {
		resetFlags(t)
		clearEnv(t)

		cfg, err := Parse()
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		if cfg.TrustedSubnet != "" {
			t.Errorf("TrustedSubnet = %q, want empty by default", cfg.TrustedSubnet)
		}
	})

	t.Run("flag -t", func(t *testing.T) {
		resetFlags(t)
		clearEnv(t)

		os.Args = []string{"testsrv", "-t", "192.168.1.0/24"}

		cfg, err := Parse()
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		if cfg.TrustedSubnet != "192.168.1.0/24" {
			t.Errorf("TrustedSubnet = %q", cfg.TrustedSubnet)
		}
	})

	t.Run("env overrides flag", func(t *testing.T) {
		resetFlags(t)

		os.Args = []string{"testsrv", "-t", "192.168.1.0/24"}
		t.Setenv("TRUSTED_SUBNET", "10.0.0.0/8")

		cfg, err := Parse()
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		if cfg.TrustedSubnet != "10.0.0.0/8" {
			t.Errorf("TrustedSubnet = %q, want env to win", cfg.TrustedSubnet)
		}
	})

	t.Run("config file", func(t *testing.T) {
		resetFlags(t)
		clearEnv(t)

		path := writeConfigFile(t, `{"trusted_subnet": "172.16.0.0/12"}`)
		os.Args = []string{"testsrv", "-c", path}

		cfg, err := Parse()
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		if cfg.TrustedSubnet != "172.16.0.0/12" {
			t.Errorf("TrustedSubnet = %q, want config file value", cfg.TrustedSubnet)
		}
	})
}

func TestOptionsMetadata(t *testing.T) {
	flags := make(map[string]bool, len(options))
	envs := make(map[string]bool, len(options))
	jsonNames := make(map[string]bool, len(options))

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, opt := range options {
		if opt.flagName == "" || opt.jsonName == "" || opt.usage == "" {
			t.Errorf("incomplete description for option %q", opt.flagName)
		}
		if opt.register == nil || opt.applyFile == nil || opt.applyEnv == nil || opt.restore == nil || opt.value == nil {
			t.Errorf("missing hook for option %q", opt.flagName)
			continue
		}
		if flags[opt.flagName] {
			t.Errorf("duplicate flag name %q", opt.flagName)
		}
		flags[opt.flagName] = true
		if opt.envName != "" {
			if envs[opt.envName] {
				t.Errorf("duplicate env name %q", opt.envName)
			}
			envs[opt.envName] = true
		}
		if jsonNames[opt.jsonName] {
			t.Errorf("duplicate json name %q", opt.jsonName)
		}
		jsonNames[opt.jsonName] = true

		opt.register(fs, &Config{})
	}

	registered := make(map[string]bool)
	fs.VisitAll(func(f *flag.Flag) { registered[f.Name] = true })
	for name := range flags {
		if !registered[name] {
			t.Errorf("flag %q is described but not registered", name)
		}
	}
	if len(registered) != len(flags) {
		t.Errorf("registered %d flags, want %d", len(registered), len(flags))
	}
}

func optionTestStrings(t *testing.T, opt option, value any) (string, string, string) {
	t.Helper()
	switch value.(type) {
	case string:
		return "file-" + opt.flagName, "flag-" + opt.flagName, "env-" + opt.flagName
	case int:
		return "101", "202", "303"
	case time.Duration:
		return "1h", "2h", "3h"
	case bool:
		return "true", "false", "true"
	default:
		t.Fatalf("unsupported option type %T", value)
		return "", "", ""
	}
}

func optionTestJSON(t *testing.T, value any, raw string) json.RawMessage {
	t.Helper()
	switch value.(type) {
	case string, time.Duration:
		return json.RawMessage(strconv.Quote(raw))
	case int, bool:
		return json.RawMessage(raw)
	default:
		t.Fatalf("unsupported option type %T", value)
		return nil
	}
}

func TestOptionPrecedence(t *testing.T) {
	for _, opt := range options {
		t.Run(opt.flagName, func(t *testing.T) {
			kind := opt.value(&Config{})
			fileRaw, flagRaw, envRaw := optionTestStrings(t, opt, kind)
			fileJSON := optionTestJSON(t, kind, fileRaw)
			path := writeConfigFile(t, fmt.Sprintf("{%q: %s}", opt.jsonName, fileJSON))

			wantFile := Config{}
			if err := opt.applyFile(&wantFile, fileJSON); err != nil {
				t.Fatalf("applyFile: %v", err)
			}
			wantFlag := Config{}
			if err := opt.applyEnv(&wantFlag, flagRaw); err != nil {
				t.Fatalf("applyEnv flag value: %v", err)
			}
			wantEnv := Config{}
			if err := opt.applyEnv(&wantEnv, envRaw); err != nil {
				t.Fatalf("applyEnv env value: %v", err)
			}

			check := func(cfg, want Config) {
				t.Helper()
				if got := opt.value(&cfg); got != opt.value(&want) {
					t.Errorf("value = %v, want %v", got, opt.value(&want))
				}
			}

			resetFlags(t)
			clearEnv(t)
			os.Args = []string{"testsrv"}
			cfg, err := Parse()
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			check(cfg, defaultConfig())

			resetFlags(t)
			clearEnv(t)
			os.Args = []string{"testsrv", "-c", path}
			cfg, err = Parse()
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			check(cfg, wantFile)

			resetFlags(t)
			clearEnv(t)
			os.Args = []string{"testsrv", "-c", path, "-" + opt.flagName + "=" + flagRaw}
			cfg, err = Parse()
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			check(cfg, wantFlag)

			if opt.envName == "" {
				return
			}

			resetFlags(t)
			clearEnv(t)
			os.Args = []string{"testsrv", "-c", path, "-" + opt.flagName + "=" + flagRaw}
			t.Setenv(opt.envName, envRaw)
			cfg, err = Parse()
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			check(cfg, wantEnv)
		})
	}
}
