package middleware

import (
	"net/http"
	"strings"

	"homework/internal/auth"
	"homework/internal/handler"
)

// Auth verifies the bearer token of a request and stores its claims in the context.
// Every failure answers 401 unauthenticated and ends the chain: next never runs (the
// early return is the net/http equivalent of AbortWithStatusJSON).
func Auth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme, raw, _ := strings.Cut(r.Header.Get("Authorization"), " ")
			raw = strings.TrimSpace(raw)
			if !strings.EqualFold(scheme, "Bearer") || raw == "" {
				handler.WriteError(w, http.StatusUnauthorized, handler.CodeUnauthenticated, "a bearer token is required")
				return
			}
			claims, err := auth.ParseToken(secret, raw)
			if err != nil {
				handler.WriteError(w, http.StatusUnauthorized, handler.CodeUnauthenticated, "invalid or expired token")
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), claims)))
		})
	}
}
