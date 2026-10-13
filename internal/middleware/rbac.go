package middleware

import (
	"net/http"
	"slices"
	"strings"

	"homework/internal/auth"
	"homework/internal/handler"
)

// RequireRole lets the request through only when the authenticated user's role is one
// of allowed. It must run after Auth: without claims in the context it answers 401,
// with a role outside allowed it answers 403, and in both cases next never runs.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.UserFromContext(r.Context())
			if !ok {
				handler.WriteError(w, http.StatusUnauthorized, handler.CodeUnauthenticated, "authentication required")
				return
			}
			if !slices.Contains(allowed, claims.Role) {
				handler.WriteError(w, http.StatusForbidden, handler.CodeForbidden, "this action requires role "+strings.Join(allowed, " or "))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
