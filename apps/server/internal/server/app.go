package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rioprastiawan/shorturl/apps/server/internal/analytics"
	"github.com/rioprastiawan/shorturl/apps/server/internal/apikey"
	"github.com/rioprastiawan/shorturl/apps/server/internal/auth"
	"github.com/rioprastiawan/shorturl/apps/server/internal/cache"
	"github.com/rioprastiawan/shorturl/apps/server/internal/config"
	"github.com/rioprastiawan/shorturl/apps/server/internal/database"
	"github.com/rioprastiawan/shorturl/apps/server/internal/domain"
	"github.com/rioprastiawan/shorturl/apps/server/internal/link"
	"github.com/rioprastiawan/shorturl/apps/server/internal/publicapi"
	"github.com/rioprastiawan/shorturl/apps/server/internal/setup"
	"github.com/rioprastiawan/shorturl/apps/server/internal/store"
	"github.com/rioprastiawan/shorturl/apps/server/internal/workspace"
)

// App holds every long-lived dependency and the services built on them.
//
// Construction is centralised here so both `serve` and `worker` share exactly
// one definition of how the system is assembled.
type App struct {
	Cfg    config.Config
	Logger *slog.Logger

	Pool    *pgxpool.Pool
	Redis   *cache.Client
	Queries *store.Queries

	Auth      *auth.Service
	Setup     *setup.Service
	Workspace *workspace.Service
	Domain    *domain.Service
	Link      *link.Service
	Analytics *analytics.Service
	APIKey    *apikey.Service

	Producer *analytics.Producer
}

// NewApp opens the database and Redis connections and wires the services.
// The caller must call Close when finished.
func NewApp(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	if cfg.RedisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is not set")
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	redis, err := cache.Connect(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		pool.Close()
		return nil, err
	}

	q := store.New(pool)
	linkCache := link.NewCache(redis, cfg.CacheTTL, logger)

	app := &App{
		Cfg:     cfg,
		Logger:  logger,
		Pool:    pool,
		Redis:   redis,
		Queries: q,

		Auth:      auth.NewService(q, cfg),
		Setup:     setup.NewService(pool, q, cfg),
		Workspace: workspace.NewService(pool, q),
		Domain:    domain.NewService(q, pool, cfg),
		Link:      link.NewService(pool, q, linkCache, cfg, logger),
		Analytics: analytics.NewService(q, cfg),
		APIKey:    apikey.NewService(q, logger),

		Producer: analytics.NewProducer(redis, cfg, logger),
	}

	return app, nil
}

// PublicAPIHandler builds the machine-to-machine handler.
func (a *App) PublicAPIHandler() *publicapi.Handler {
	return publicapi.NewHandler(a.Link, a.Queries)
}

// SyncTraefik regenerates the dynamic router files for every active domain.
//
// Run at startup: the directory can drift while the process is down — a domain
// verified by another replica, or a file lost when a volume was recreated — and
// a custom domain silently not routing is a failure nobody notices until a link
// breaks.
func (a *App) SyncTraefik(ctx context.Context) {
	if a.Cfg.TraefikDynamicDir == "" {
		return
	}
	if err := a.Domain.SyncTraefik(ctx); err != nil {
		// Never fatal: the database is the source of truth and the next
		// verification or restart retries.
		a.Logger.Error("could not sync Traefik dynamic configuration",
			slog.String("dir", a.Cfg.TraefikDynamicDir),
			slog.String("error", err.Error()),
		)
		return
	}
	a.Logger.Info("synced Traefik dynamic configuration",
		slog.String("dir", a.Cfg.TraefikDynamicDir))
}

// Close releases connections. The producer is drained first so click events
// buffered in memory reach Redis before the connection goes away.
func (a *App) Close(ctx context.Context) {
	if a.Producer != nil {
		if err := a.Producer.Close(ctx); err != nil {
			a.Logger.Warn("draining analytics producer", slog.String("error", err.Error()))
		}
	}
	if a.Redis != nil {
		if err := a.Redis.Close(); err != nil {
			a.Logger.Warn("closing redis", slog.String("error", err.Error()))
		}
	}
	if a.Pool != nil {
		a.Pool.Close()
	}
}
