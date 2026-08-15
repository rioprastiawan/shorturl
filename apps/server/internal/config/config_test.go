package config

import (
	"log/slog"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.Addr() != ":8080" {
		t.Errorf("Addr() = %q, want :8080", cfg.Addr())
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.IsProduction() {
		t.Error("IsProduction() = true, want false")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_URL", "https://short.example.com/")
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("SESSION_SECRET", strings.Repeat("a", 64))
	t.Setenv("IP_HASH_SECRET", strings.Repeat("b", 64))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.IsProduction() {
		t.Error("IsProduction() = false, want true")
	}
	if cfg.AppURL != "https://short.example.com" {
		t.Errorf("AppURL = %q, want trailing slash trimmed", cfg.AppURL)
	}
	if cfg.Addr() != ":9000" {
		t.Errorf("Addr() = %q, want :9000", cfg.Addr())
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info in production", cfg.LogLevel)
	}
}

func TestRequireSecrets(t *testing.T) {
	t.Run("development needs none", func(t *testing.T) {
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if err := cfg.RequireSecrets(); err != nil {
			t.Errorf("RequireSecrets() = %v, want nil outside production", err)
		}
	})

	t.Run("production without secrets fails", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		cfg, err := Load()
		if err != nil {
			// Load itself must succeed: `migrate` runs in production and has
			// no use for a session key.
			t.Fatalf("Load() = %v, want nil so secret-free commands can boot", err)
		}
		if err := cfg.RequireSecrets(); err == nil {
			t.Error("RequireSecrets() = nil in production with no secrets set")
		}
	})

	t.Run("production with secrets passes", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("SESSION_SECRET", strings.Repeat("a", 64))
		t.Setenv("IP_HASH_SECRET", strings.Repeat("b", 64))
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if err := cfg.RequireSecrets(); err != nil {
			t.Errorf("RequireSecrets() = %v, want nil", err)
		}
	})

	t.Run("production rejects a short secret", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("SESSION_SECRET", "tooshort")
		if _, err := Load(); err == nil {
			t.Error("Load() accepted a secret below the length floor")
		}
	})
}

func TestLoadRejectsBadValues(t *testing.T) {
	tests := map[string]struct{ key, value string }{
		"non-numeric port":  {"SERVER_PORT", "http"},
		"out of range port": {"SERVER_PORT", "70000"},
		"unknown log level": {"LOG_LEVEL", "chatty"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() with %s=%q: want error, got nil", tc.key, tc.value)
			}
		})
	}
}
