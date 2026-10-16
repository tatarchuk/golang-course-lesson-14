package repository_test

import (
	"io/fs"
	"testing"

	"homework/internal/repository"
	"homework/internal/testdb"
	"homework/migrations"
)

func TestMigrateIsIdempotentAndCreatesEveryTable(t *testing.T) {
	db := testdb.DB(t)
	// Already applied by testdb.Run; a second run through the on-disk override (the
	// directory behind MIGRATIONS_PATH) must be a no-op, not an error.
	if err := repository.Migrate(testdb.DSN(), "../../migrations"); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var n int64
	err := db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('genres','artists','albums','album_artists','copies','users')").Scan(&n).Error
	if err != nil || n != 6 {
		t.Fatalf("expected 6 tables, got %d (%v)", n, err)
	}
}

// TestMigrateFSAppliesTheEmbeddedSet pins Lesson 10's single-binary requirement: the
// migrations travel inside the binary, so MigrateFS with migrations.FS must see every
// up/down pair and be a no-op on a database that testdb.Run already migrated.
func TestMigrateFSAppliesTheEmbeddedSet(t *testing.T) {
	db := testdb.DB(t)
	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil || len(files) != 6 {
		t.Fatalf("expected 3 up/down pairs in the embedded set, got %v (%v)", files, err)
	}
	if err := repository.MigrateFS(testdb.DSN(), migrations.FS); err != nil {
		t.Fatalf("embedded migrate on an up-to-date database: %v", err)
	}
	var version int64
	if err := db.Raw("SELECT version FROM schema_migrations").Scan(&version).Error; err != nil || version != 3 {
		t.Fatalf("expected schema version 3, got %d (%v)", version, err)
	}
}
