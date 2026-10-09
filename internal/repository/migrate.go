package repository

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrate applies every pending migration from dir to the database at dsn, using
// golang-migrate as a library. It is idempotent: an up-to-date database is left as it
// is. The application runs it at startup, so a fresh database receives the schema
// and the demo data before the first request. The migrator opens its own
// connection and closes it on return; it never touches GORM's pool.
func Migrate(dsn, dir string) error {
	src, err := iofs.New(os.DirFS(dir), ".")
	if err != nil {
		return fmt.Errorf("migrations in %q: %w", dir, err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, pgxURL(dsn))
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("migrate version: %w", err)
	}
	log.Printf("migrations: schema version %d (dirty=%t)", version, dirty)
	return nil
}

// pgxURL rewrites a postgres:// URL to the pgx5:// scheme golang-migrate's pgx/v5
// driver registers; the rest of the URL is identical.
func pgxURL(dsn string) string {
	for _, scheme := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(dsn, scheme) {
			return "pgx5://" + strings.TrimPrefix(dsn, scheme)
		}
	}
	return dsn
}
