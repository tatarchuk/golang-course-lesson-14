package main

import (
	"os"
	"testing"

	"homework/internal/auth"
	"homework/internal/model"
	"homework/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m))
}

// TestRunCreatesAndResetsTheAdmin runs the command twice against the test database: the
// first run creates the administrator, the second resets the password of the same row.
func TestRunCreatesAndResetsTheAdmin(t *testing.T) {
	testdb.Reset(t)
	t.Setenv("DATABASE_URL", testdb.DSN())
	t.Setenv("MIGRATIONS_PATH", testdb.MigrationsDir())
	t.Setenv("ADMIN_PASSWORD", "")

	if err := run(" Admin@Albums.LOCAL ", "Albums-Dev-2026"); err != nil {
		t.Fatal(err)
	}
	var user model.User
	if err := testdb.DB(t).First(&user, "email = ?", "admin@albums.local").Error; err != nil {
		t.Fatalf("the admin must be stored under the normalized e-mail: %v", err)
	}
	if user.Role != model.UserRoleAdmin || !auth.CheckPassword(user.PasswordHash, "Albums-Dev-2026") {
		t.Errorf("admin row: role %q, password matches=%v", user.Role, auth.CheckPassword(user.PasswordHash, "Albums-Dev-2026"))
	}

	t.Setenv("ADMIN_PASSWORD", "Another-Pass-2026")
	if err := run("admin@albums.local", ""); err != nil { // the password comes from the environment
		t.Fatal(err)
	}
	var again model.User
	if err := testdb.DB(t).First(&again, "email = ?", "admin@albums.local").Error; err != nil {
		t.Fatal(err)
	}
	if again.ID != user.ID || again.Role != model.UserRoleAdmin || !auth.CheckPassword(again.PasswordHash, "Another-Pass-2026") {
		t.Errorf("the second run must reset the password of the same row: %+v", again)
	}
	var n int64
	if err := testdb.DB(t).Model(&model.User{}).Count(&n).Error; err != nil || n != 1 {
		t.Errorf("expected exactly one account, got %d (%v)", n, err)
	}
}

func TestRunRejectsMissingInput(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "")
	if err := run("admin@albums.local", ""); err == nil {
		t.Error("a missing password must be an error")
	}
	if err := run("admin@albums.local", "short"); err == nil {
		t.Error("a password under 8 bytes must be an error")
	}
	t.Setenv("DATABASE_URL", "")
	if err := run("admin@albums.local", "Albums-Dev-2026"); err == nil {
		t.Error("a missing DATABASE_URL must be an error")
	}
}
