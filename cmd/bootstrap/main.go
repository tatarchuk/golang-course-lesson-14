// Command bootstrap creates or resets an administrator account: it hashes the password
// with bcrypt and upserts the row with role admin. It is the only way to give the first
// account a password, because migrations never store hashes.
//
//	DATABASE_URL=... ADMIN_PASSWORD=... go run ./cmd/bootstrap -email admin@albums.local
//	docker compose exec -e ADMIN_PASSWORD=... api /app/bootstrap -email admin@albums.local
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"homework/internal/auth"
	"homework/internal/model"
	"homework/internal/repository"
	"homework/migrations"
)

func main() {
	email := flag.String("email", "admin@albums.local", "e-mail of the administrator account")
	password := flag.String("password", "", "password; prefer ADMIN_PASSWORD, which stays out of the shell history and the process list")
	flag.Parse()
	if err := run(*email, *password); err != nil {
		log.Fatal(err)
	}
}

// run validates the input, applies pending migrations (a no-op when the API already did)
// and upserts the account. SQL is logged with placeholders (repository.Open), so the hash
// never reaches the output at any log level.
func run(email, password string) error {
	if password == "" {
		password = os.Getenv("ADMIN_PASSWORD")
	}
	in := model.RegisterInput{Email: email, Password: password}
	if err := in.Validate(); err != nil {
		return fmt.Errorf("invalid input (set ADMIN_PASSWORD or pass -password): %w", err)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required, e.g. postgres://albums:albums@localhost:5432/albums?sslmode=disable")
	}
	migrationsDir := os.Getenv("MIGRATIONS_PATH") // empty: the set embedded in the binary
	db, err := repository.Open(repository.Config{DSN: dsn, LogLevel: "warn"})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if migrationsDir != "" {
		err = repository.Migrate(dsn, migrationsDir)
	} else {
		err = repository.MigrateFS(dsn, migrations.FS)
	}
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	user, err := repository.NewGorm(db).Users.UpsertAdmin(context.Background(), model.NormalizeEmail(email), hash)
	if err != nil {
		return err
	}
	fmt.Printf("administrator %s is ready (id %d)\n", user.Email, user.ID)
	return nil
}
