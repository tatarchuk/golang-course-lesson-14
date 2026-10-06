package app

import (
	"net/http"

	"homework/internal/handler"
	"homework/internal/middleware"
	"homework/internal/repository"
)

// NewRouter builds a complete HTTP handler with its own empty in-memory store.
func NewRouter() http.Handler {
	repo := repository.NewMemoryRepository() // fresh store per router: no globals, tests stay isolated
	albums := handler.NewAlbumHandler(repo)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /api/v1/albums", albums.Create)
	mux.HandleFunc("GET /api/v1/albums/{id}", albums.Get)
	mux.HandleFunc("PUT /api/v1/albums/{id}", albums.Update)
	mux.HandleFunc("DELETE /api/v1/albums/{id}", albums.Delete)
	mux.HandleFunc("GET /api/v1/albums", albums.List)
	mux.HandleFunc("/", handler.NotFound) // everything else: JSON 404

	return middleware.Logging(middleware.Recover(middleware.CORS(mux)))
}
