package repository_test

import (
	"os"
	"testing"

	"homework/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m))
}
