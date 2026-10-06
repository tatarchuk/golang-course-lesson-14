package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"homework/internal/handler"
)

// Recover turns a panic in any downstream handler into a 500 with the contract's error body.
// The panic value and the stack trace go to the log only; the client never sees them.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v\n%s", rec, debug.Stack())
				handler.WriteError(w, http.StatusInternalServerError, handler.CodeInternal, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
