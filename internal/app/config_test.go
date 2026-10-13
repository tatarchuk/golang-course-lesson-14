package app_test

import (
	"testing"

	"homework/internal/app"
	"homework/internal/repository"
)

// TestNewRouterPanicsWithoutSecret pins the fail-fast: an empty secret must never sign
// tokens with an empty key (main refuses to start first, the router refuses second).
func TestNewRouterPanicsWithoutSecret(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewRouter must panic when JWTSecret is empty")
		}
	}()
	app.NewRouter(repository.Repositories{}, app.Config{})
}
