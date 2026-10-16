package app_test

import (
	"testing"

	"homework/internal/app"
	"homework/internal/repository"
	"homework/web"
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

// TestNewRouterPanicsWithoutTemplatesOrStatic pins the other fail-fast: the web UI cannot
// render without its embedded files, so main must pass both filesystems (Lesson 10).
func TestNewRouterPanicsWithoutTemplatesOrStatic(t *testing.T) {
	for name, cfg := range map[string]app.Config{
		"no templates": {JWTSecret: testJWTSecret, Static: web.Static()},
		"no static":    {JWTSecret: testJWTSecret, Templates: web.Templates()},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("NewRouter must panic with %s", name)
				}
			}()
			app.NewRouter(repository.Repositories{}, cfg)
		})
	}
}
