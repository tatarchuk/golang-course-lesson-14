package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// MigrateFS applies every pending migration found in fsys to the database at dsn,
// using golang-migrate as a library. It is idempotent: an up-to-date database is left
// as it is. The application runs it at startup with the migrations embedded in the
// binary (package migrations), so a fresh database receives the schema and the demo
// data before the first request. The migrator opens its own connection and closes it
// on return; it never touches GORM's pool.
func MigrateFS(dsn string, fsys fs.FS) error {
	src, err := iofs.New(fsys, ".")
	if err != nil {
		return fmt.Errorf("migrations source: %w", err)
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

// Migrate applies the SQL files of the directory dir on disk: the development override
// behind MIGRATIONS_PATH. The default source is the embedded set, migrations.FS.
func Migrate(dsn, dir string) error {
	return MigrateFS(dsn, os.DirFS(dir))
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
