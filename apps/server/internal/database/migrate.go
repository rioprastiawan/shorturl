package database

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/rioprastiawan/shorturl/apps/server/db"
)

// Migrate applies every pending migration.
//
// golang-migrate takes an advisory lock on the database for the duration, so
// running this concurrently from several containers is safe: the losers block
// and then find nothing to do. The Compose stack still uses a one-shot service
// so that a migration failure stops the deploy instead of crash-looping.
func Migrate(databaseURL string, logger *slog.Logger) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer closeMigrator(m, logger)

	before, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("read schema version: %w", err)
	}
	if dirty {
		return fmt.Errorf("schema is dirty at version %d: a previous migration failed partway and needs manual repair", before)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("schema already up to date", slog.Uint64("version", uint64(before)))
			return nil
		}
		return fmt.Errorf("apply migrations: %w", err)
	}

	after, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	logger.Info("migrations applied",
		slog.Uint64("from", uint64(before)),
		slog.Uint64("to", uint64(after)),
	)
	return nil
}

// MigrateDown rolls back the most recent migration. Development only; there is
// no Compose service for it.
func MigrateDown(databaseURL string, logger *slog.Logger) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer closeMigrator(m, logger)

	if err := m.Steps(-1); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("nothing to roll back")
			return nil
		}
		return fmt.Errorf("roll back migration: %w", err)
	}
	logger.Info("rolled back one migration")
	return nil
}

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	source, err := iofs.New(db.Migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("load embedded migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, normalizeURL(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("open migrator: %w", err)
	}
	return m, nil
}

// normalizeURL rewrites postgres:// to the pgx5:// scheme golang-migrate's pgx
// driver registers itself under, so the same DATABASE_URL works everywhere.
func normalizeURL(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if len(databaseURL) > len(prefix) && databaseURL[:len(prefix)] == prefix {
			return "pgx5://" + databaseURL[len(prefix):]
		}
	}
	return databaseURL
}

func closeMigrator(m *migrate.Migrate, logger *slog.Logger) {
	sourceErr, dbErr := m.Close()
	if sourceErr != nil {
		logger.Warn("closing migration source", slog.String("error", sourceErr.Error()))
	}
	if dbErr != nil {
		logger.Warn("closing migration database", slog.String("error", dbErr.Error()))
	}
}

// ensure the pgx driver is linked in.
var _ = pgxmigrate.Postgres{}
