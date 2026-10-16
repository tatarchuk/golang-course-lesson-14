package middleware

import "net/http"

// SecurityHeaders adds the browser-side protections of the server-rendered pages: a
// Content-Security-Policy that allows this origin only (htmx is served from /static/ and
// the templates carry no inline script or style), no framing, no MIME sniffing and a
// same-origin referrer. The API does not need them: its JSON is never rendered by a
// browser as a document.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
