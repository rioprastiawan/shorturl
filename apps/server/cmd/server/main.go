// Command server is the single ShortURL binary. Subcommands keep the Docker
// image and operational surface small: one image, different commands.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rioprastiawan/shorturl/apps/server/internal/analytics"
	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/database"
	"github.com/rioprastiawan/shorturl/apps/server/internal/server"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `shorturl - self-hosted URL shortener

Usage:
  shorturl serve         Run the HTTP server (API and redirects)
  shorturl worker        Run the analytics worker
  shorturl migrate       Apply pending database migrations
  shorturl migrate-down  Roll back the most recent migration
  shorturl healthcheck   Probe the local /health endpoint (exit 0 when healthy)
  shorturl version       Print the build version
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "serve":
		return serve()
	case "worker":
		return worker()
	case "migrate":
		return migrate()
	case "migrate-down":
		return migrateDown()
	case "healthcheck":
		return healthcheck()
	case "version":
		fmt.Println(version)
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", command)
	}
}

func serve() error {
	cfg, logger, err := boot()
	if err != nil {
		return err
	}
	// Only this command signs cookies and hashes addresses.
	if err := cfg.RequireSecrets(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := server.NewApp(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("start application: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		app.Close(closeCtx)
	}()

	// Repair Traefik's dynamic routers before accepting traffic: the directory
	// can drift while the process is down, and a custom domain that silently
	// stops routing is not noticed until someone reports a broken link.
	app.SyncTraefik(ctx)

	return server.New(cfg, logger, version, app).Run(ctx)
}

func worker() error {
	cfg, logger, err := boot()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := server.NewApp(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("start application: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		app.Close(closeCtx)
	}()

	logger.Info("analytics worker starting",
		slog.String("stream", cfg.ClickStreamName),
		slog.String("group", cfg.ClickConsumerGroup),
	)

	return analytics.NewWorker(app.Pool, app.Queries, app.Redis, cfg, logger).Run(ctx)
}

func migrate() error {
	cfg, logger, err := boot()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}
	return database.Migrate(cfg.DatabaseURL, logger)
}

func migrateDown() error {
	cfg, logger, err := boot()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}
	return database.MigrateDown(cfg.DatabaseURL, logger)
}

// healthcheck probes the server's own /health endpoint. The production image
// is distroless and has no shell, wget, or curl, so Docker's HEALTHCHECK runs
// this subcommand instead.
func healthcheck() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d/health", cfg.ServerPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// boot loads configuration and installs the logger, which every command needs
// before it can do anything useful.
func boot() (config.Config, *slog.Logger, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("load config: %w", err)
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)
	return cfg, logger, nil
}

// newLogger emits JSON in production for log aggregation, and human-readable
// text during development.
func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
