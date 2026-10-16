package middleware

import (
	"net/http"

	"homework/internal/auth"
)

// CookieAuth is the web UI's counterpart of Auth. It reads the session cookie and, when
// it carries a valid token, stores the claims in the request context; otherwise the
// request simply continues anonymously. A missing or stale cookie is not an error on a
// public page, and RequireRole behind this middleware still answers 401 or 403 where a
// role is required, with the same codes as the API. It never reads the Authorization
// header: the web routes accept the cookie only and the API accepts the header only, so
// a bearer token cannot be replayed against the web routes nor a cookie against the API.
func CookieAuth(secret, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c, err := r.Cookie(cookieName); err == nil {
				if claims, err := auth.ParseToken(secret, c.Value); err == nil {
					r = r.WithContext(auth.WithUser(r.Context(), claims))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
