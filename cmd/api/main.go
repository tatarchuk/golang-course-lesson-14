package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"homework/internal/app"
	"homework/internal/repository"
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
	if secret == "" {
		return errors.New("JWT_SECRET is required: a long random string that signs login tokens")
	}
	migrations := envOr("MIGRATIONS_PATH", "migrations")
	port := envOr("PORT", "8080")

	db, err := repository.Open(repository.Config{DSN: dsn, LogLevel: os.Getenv("DB_LOG_LEVEL")})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := repository.Migrate(dsn, migrations); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           app.NewRouter(repository.NewGorm(db), app.Config{JWTSecret: secret}),
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
