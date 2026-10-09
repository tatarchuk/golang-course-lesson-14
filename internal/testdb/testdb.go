// Package testdb gives database-backed tests one shared PostgreSQL connection.
//
// go test ./... runs packages in parallel, and two packages truncating the same
// tables at the same time would corrupt each other's fixtures. Run therefore holds
// a session-level advisory lock for the whole package run, so packages that use
// the database execute one after another. Tests inside a package run sequentially
// anyway.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"gorm.io/gorm"

	"homework/internal/repository"
)

// lockID is an arbitrary application-level key for pg_advisory_lock.
const lockID = 804_801

var (
	db  *gorm.DB
	dsn string
)

// Run wraps testing.M for a package that needs the database. When TEST_DATABASE_URL
// is unset it just runs the tests, which then skip via DB; otherwise it opens the
// connection, takes the advisory lock, applies the migrations once and releases
// everything when the package is done.
func Run(m *testing.M) int {
	dsn = os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		return m.Run()
	}

	var err error
	db, err = repository.Open(repository.Config{DSN: dsn, LogLevel: "silent"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb: open: %v\n", err)
		return 1
	}
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb: sql db: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// A dedicated connection holds the lock: advisory locks are per session.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb: conn: %v\n", err)
		return 1
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: lock: %v\n", err)
		return 1
	}
	if err := repository.Migrate(dsn, MigrationsDir()); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: migrate: %v\n", err)
		return 1
	}

	code := m.Run()

	unlock(conn)
	_ = sqlDB.Close()
	return code
}

func unlock(conn *sql.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	_ = conn.Close()
}

// DB returns the shared connection or skips the test when no database is configured.
func DB(t testing.TB) *gorm.DB {
	t.Helper()
	if db == nil {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return db
}

// DSN returns the test database URL (empty when not configured).
func DSN() string { return dsn }

// Reset empties every table and restarts the id sequences, so a test starts from a
// clean state with predictable ids.
func Reset(t testing.TB) {
	t.Helper()
	d := DB(t)
	err := d.Exec("TRUNCATE copies, album_artists, albums, artists, genres, users RESTART IDENTITY CASCADE").Error
	if err != nil {
		t.Fatalf("testdb: reset: %v", err)
	}
}

// MigrationsDir is the repository's migrations directory, resolved relative to this
// file so tests work from any package directory.
func MigrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}
