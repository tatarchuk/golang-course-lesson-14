package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"homework/internal/app"
	"homework/internal/repository"
	"homework/migrations"
	"homework/web"
)

// @title        Albums API
// @version      3.0
// @description  REST API for a music album catalogue on PostgreSQL: genres, artists, albums and their physical copies. Reads are public; writes need a staff or admin bearer token from POST /auth/login.
// @BasePath     /api/v1
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer " followed by the token from POST /auth/login.
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run reads the configuration from the environment, connects to the database,
// applies pending migrations, serves HTTP and shuts down cleanly on SIGINT/SIGTERM.
func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required, e.g. postgres://albums:albums@localhost:5432/albums?sslmode=disable")
	}
	secret := os.Getenv("JWT_SECRET")
	if err := validateSecret(secret); err != nil {
		return err
	}
	migrationsDir := os.Getenv("MIGRATIONS_PATH") // empty: the embedded set
	port := envOr("PORT", "8080")
	cookieSecure := envBool("COOKIE_SECURE")

	db, err := repository.Open(repository.Config{DSN: dsn, LogLevel: os.Getenv("DB_LOG_LEVEL")})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := applyMigrations(dsn, migrationsDir); err != nil {
		return err
	}

	// The web UI's templates, static files and the migrations travel inside the binary:
	// nothing is read from disk, so the server runs from any directory (Lesson 10).
	cfg := app.Config{JWTSecret: secret, Templates: web.Templates(), Static: web.Static(), CookieSecure: cookieSecure}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           app.NewRouter(repository.NewGorm(db), cfg),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		log.Print("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// minSecretLength is the shortest JWT_SECRET the server accepts: HS256 is only as strong as
// the secret, and a short one is brute-forceable offline from any captured token.
const minSecretLength = 32

// validateSecret rejects an empty, short or template secret before anything is signed with
// it, so the .env.example placeholder can never reach a running server.
func validateSecret(secret string) error {
	switch {
	case secret == "":
		return errors.New("JWT_SECRET is required: a long random string that signs login tokens (openssl rand -base64 48)")
	case strings.HasPrefix(secret, "change-me"):
		return errors.New("JWT_SECRET still holds the .env.example placeholder: generate one with openssl rand -base64 48")
	case len(secret) < minSecretLength:
		return fmt.Errorf("JWT_SECRET must be at least %d bytes (openssl rand -base64 48)", minSecretLength)
	}
	return nil
}

// applyMigrations runs the migrations embedded in the binary or, when dir is not empty
// (MIGRATIONS_PATH), the SQL files of that directory: a development override, the binary
// itself needs no files on disk.
func applyMigrations(dsn, dir string) error {
	if dir != "" {
		log.Printf("migrations: applying the SQL files of %s", dir)
		return repository.Migrate(dsn, dir)
	}
	log.Print("migrations: applying the embedded set")
	return repository.MigrateFS(dsn, migrations.FS)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envBool reads an on/off switch: "true" and "1" (any case) switch it on, anything else
// including an unset variable leaves it off.
func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "true" || v == "1"
}
