package repository_test

import (
	"testing"

	"homework/internal/repository"
	"homework/internal/testdb"
)

func TestMigrateIsIdempotentAndCreatesEveryTable(t *testing.T) {
	db := testdb.DB(t)
	// Already applied by testdb.Run; a second run must be a no-op, not an error.
	if err := repository.Migrate(testdb.DSN(), testdb.MigrationsDir()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var n int64
	err := db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('genres','artists','albums','album_artists','copies','users')").Scan(&n).Error
	if err != nil || n != 6 {
		t.Fatalf("expected 6 tables, got %d (%v)", n, err)
	}
}
