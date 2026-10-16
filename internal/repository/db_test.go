package repository_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"homework/internal/repository"
	"homework/internal/testdb"
)

// TestOpenLogsSQLWithoutValues pins the fix for docs/auth/audit-raw.md finding 1: at Info level
// the GORM logger prints every statement, so it must print placeholders instead of bound
// values, or a bcrypt hash and an e-mail would land in the server log on every registration.
func TestOpenLogsSQLWithoutValues(t *testing.T) {
	testdb.Reset(t)
	var buf bytes.Buffer
	db, err := repository.Open(repository.Config{DSN: testdb.DSN(), LogLevel: "info", LogOutput: &buf})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	const hash = "$2a$10$notarealhashbutitlookslikeoneforthelogcheck00000000000"
	if _, err := repository.NewGorm(db).Users.Create(context.Background(), "logged@albums.local", hash); err != nil {
		t.Fatal(err)
	}
	log := buf.String()
	if !strings.Contains(log, `INSERT INTO "users"`) {
		t.Fatalf("expected the INSERT statement in the log, got:\n%s", log)
	}
	for _, secret := range []string{"$2a$10$", "logged@albums.local"} {
		if strings.Contains(log, secret) {
			t.Errorf("the log must not contain %q:\n%s", secret, log)
		}
	}
}
