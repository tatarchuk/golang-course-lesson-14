package handler

import "net/http"

// Health handles GET /health.
func Health(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// NotFound is the catch-all for unknown routes, so even a 404 is JSON.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	WriteError(w, http.StatusNotFound, CodeNotFound, "route not found")
}
