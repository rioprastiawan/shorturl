// Package config loads application configuration from the process environment.
package config

import (
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every setting the server needs. Values come from the
// environment only; there is no config file and there are no baked-in secrets.
type Config struct {
	AppEnv    string
	AppName   string
	AppDomain string
	AppURL    string

	ServerPort      int
	ShutdownTimeout time.Duration

	DatabaseURL   string
	RedisAddr     string
	RedisPassword string

	SessionSecret  []byte
	IPHashSecret   []byte
	SessionTTL     time.Duration
	SessionCookie  string
	CookieSecure   bool
	TrustedProxies []netip.Prefix

	// Rate limits, all per minute.
	RateLimitAuth     int
	RateLimitAPI      int
	RateLimitRedirect int

	// Analytics
	ClickStreamName     string
	ClickConsumerGroup  string
	ClickBatchSize      int
	ClickBlockDuration  time.Duration
	ClickRetentionDays  int
	ClickStreamMaxLen   int64
	CacheTTL            time.Duration
	TraefikDynamicDir   string
	TraefikCertResolver string

	LogLevel slog.Level
}

// IsProduction reports whether the server runs with production defaults
// (JSON logs, secure cookies, no verbose error bodies).
func (c Config) IsProduction() bool { return c.AppEnv == "production" }

// Addr is the TCP address the HTTP server listens on.
func (c Config) Addr() string { return fmt.Sprintf(":%d", c.ServerPort) }

// RequireSecrets verifies the signing keys are present.
//
// Only `serve` needs them — sessions, IP anonymisation, and the password-gate
// cookie. `migrate` and `worker` do not, and demanding them there would push
// operators into copying secrets to containers that never use them.
func (c Config) RequireSecrets() error {
	if !c.IsProduction() {
		return nil
	}
	for _, s := range []struct {
		name  string
		value []byte
	}{
		{"SESSION_SECRET", c.SessionSecret},
		{"IP_HASH_SECRET", c.IPHashSecret},
	} {
		if len(s.value) == 0 {
			return fmt.Errorf("%s is required when APP_ENV=production (generate with: openssl rand -hex 32)", s.name)
		}
	}
	return nil
}

// Load reads configuration from the environment, applying defaults suitable
// for local development. It returns an error when a value is present but
// unusable, so misconfiguration fails at startup rather than at first request.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:              env("APP_ENV", "development"),
		AppName:             env("APP_NAME", "ShortURL"),
		AppDomain:           strings.ToLower(env("APP_DOMAIN", "localhost")),
		AppURL:              strings.TrimRight(env("APP_URL", "http://localhost:8080"), "/"),
		ShutdownTimeout:     15 * time.Second,
		DatabaseURL:         env("DATABASE_URL", ""),
		RedisAddr:           env("REDIS_ADDR", ""),
		RedisPassword:       env("REDIS_PASSWORD", ""),
		SessionCookie:       env("SESSION_COOKIE_NAME", "shorturl_session"),
		ClickStreamName:     env("CLICK_STREAM_NAME", "shorturl:click-events"),
		ClickConsumerGroup:  env("CLICK_CONSUMER_GROUP", "analytics-workers"),
		TraefikDynamicDir:   env("TRAEFIK_DYNAMIC_DIR", ""),
		TraefikCertResolver: env("TRAEFIK_CERT_RESOLVER", "letsencrypt"),
	}

	cfg.AppEnv = strings.ToLower(cfg.AppEnv)
	cfg.CookieSecure = cfg.IsProduction()

	var err error
	if cfg.ServerPort, err = envPort("SERVER_PORT", 8080); err != nil {
		return Config{}, err
	}
	if cfg.LogLevel, err = parseLogLevel(env("LOG_LEVEL", defaultLogLevel(cfg.AppEnv))); err != nil {
		return Config{}, err
	}

	// Secrets are parsed here but not demanded: `migrate` has no use for a
	// session key, and requiring one would mean handing it to a container that
	// only touches the schema. Commands that actually need them call
	// RequireSecrets, so the check happens where the need is.
	//
	// Development falls back to a fixed value so `go run` works at all, which
	// is safe only because production is gated by that same call.
	if cfg.SessionSecret, err = envSecret("SESSION_SECRET", cfg.IsProduction()); err != nil {
		return Config{}, err
	}
	if cfg.IPHashSecret, err = envSecret("IP_HASH_SECRET", cfg.IsProduction()); err != nil {
		return Config{}, err
	}

	for _, spec := range []struct {
		target   *int
		key      string
		fallback int
	}{
		{&cfg.RateLimitAuth, "RATE_LIMIT_AUTH_PER_MINUTE", 10},
		{&cfg.RateLimitAPI, "RATE_LIMIT_API_PER_MINUTE", 120},
		{&cfg.RateLimitRedirect, "RATE_LIMIT_REDIRECT_PER_MINUTE", 600},
		{&cfg.ClickBatchSize, "CLICK_BATCH_SIZE", 256},
		{&cfg.ClickRetentionDays, "CLICK_RETENTION_DAYS", 400},
	} {
		if *spec.target, err = envInt(spec.key, spec.fallback, 1, 1_000_000); err != nil {
			return Config{}, err
		}
	}

	streamMaxLen, err := envInt("CLICK_STREAM_MAX_LEN", 1_000_000, 1000, 100_000_000)
	if err != nil {
		return Config{}, err
	}
	cfg.ClickStreamMaxLen = int64(streamMaxLen)

	sessionDays, err := envInt("SESSION_TTL_DAYS", 30, 1, 365)
	if err != nil {
		return Config{}, err
	}
	cfg.SessionTTL = time.Duration(sessionDays) * 24 * time.Hour

	cacheMinutes, err := envInt("CACHE_TTL_MINUTES", 60, 1, 10_080)
	if err != nil {
		return Config{}, err
	}
	cfg.CacheTTL = time.Duration(cacheMinutes) * time.Minute

	cfg.ClickBlockDuration = 5 * time.Second

	// Default to the RFC1918 and loopback ranges: in Compose, Traefik reaches
	// the server over a private bridge network, and nothing else can.
	if cfg.TrustedProxies, err = envPrefixes("TRUSTED_PROXIES",
		"127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,fc00::/7"); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func defaultLogLevel(appEnv string) string {
	if appEnv == "production" {
		return "info"
	}
	return "debug"
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback, min, max int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a number", key, raw)
	}
	if v < min || v > max {
		return 0, fmt.Errorf("%s: %d is outside the allowed range %d-%d", key, v, min, max)
	}
	return v, nil
}

func envPort(key string, fallback int) (int, error) {
	return envInt(key, fallback, 1, 65535)
}

// envSecret returns the secret as bytes, or nil in production when it is
// absent. A development fallback keeps `go run` usable; RequireSecrets is what
// stops that fallback ever reaching production.
func envSecret(key string, production bool) ([]byte, error) {
	raw := os.Getenv(key)
	if raw == "" {
		if production {
			return nil, nil
		}
		return []byte("insecure-development-" + key), nil
	}
	if production && len(raw) < 32 {
		return nil, fmt.Errorf("%s must be at least 32 characters (generate with: openssl rand -hex 32)", key)
	}
	return []byte(raw), nil
}

func envPrefixes(key, fallback string) ([]netip.Prefix, error) {
	raw := env(key, fallback)
	var out []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a CIDR range", key, part)
		}
		out = append(out, prefix)
	}
	return out, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL: %q is not one of debug, info, warn, error", raw)
	}
}
