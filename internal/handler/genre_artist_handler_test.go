package handler

import (
	"net/http"
	"strings"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
)

func TestGenreHandlerCreateGetList(t *testing.T) {
	fake := &fakeGenres{
		t: t,
		create: func(in model.GenreInput) (model.Genre, error) {
			if in.Name == "jazz" {
				return model.Genre{}, repository.ErrConflict
			}
			return model.Genre{ID: 2, Name: in.Name}, nil
		},
		get: func(id int64) (model.Genre, error) {
			if id == 1 {
				return model.Genre{ID: 1, Name: "jazz"}, nil
			}
			return model.Genre{}, repository.ErrNotFound
		},
		list: func() ([]model.Genre, error) { return []model.Genre{{ID: 1, Name: "jazz"}}, nil },
	}
	h := NewGenreHandler(fake)
	routes := func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api/v1/genres", h.Create)
		mux.HandleFunc("GET /api/v1/genres", h.List)
		mux.HandleFunc("GET /api/v1/genres/{id}", h.Get)
	}

	rec := serve(routes, http.MethodPost, "/api/v1/genres", `{"name":"rock"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"id":2`) {
		t.Errorf("create: expected 201, got %d %s", rec.Code, rec.Body.String())
	}
	expectError(t, serve(routes, http.MethodPost, "/api/v1/genres", `{"name":"jazz"}`), http.StatusConflict, CodeConflict)
	expectError(t, serve(routes, http.MethodPost, "/api/v1/genres", `{}`), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, serve(routes, http.MethodPost, "/api/v1/genres", `{"name":"  "}`), http.StatusUnprocessableEntity, CodeValidation)
	rec = serve(routes, http.MethodGet, "/api/v1/genres/1", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"jazz"`) {
		t.Errorf("get: expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	rec = serve(routes, http.MethodGet, "/api/v1/genres/5", "")
	expectError(t, rec, http.StatusNotFound, CodeNotFound)
	if !strings.Contains(rec.Body.String(), "Genre not found") {
		t.Errorf("expected a genre-specific message, got %s", rec.Body.String())
	}
	expectError(t, serve(routes, http.MethodGet, "/api/v1/genres/x", ""), http.StatusBadRequest, CodeInvalidID)
	rec = serve(routes, http.MethodGet, "/api/v1/genres", "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), `[{"id":1`) {
		t.Errorf("list: expected 200 with an array, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestArtistHandlerCreateGetList(t *testing.T) {
	fake := &fakeArtists{
		t: t,
		create: func(in model.ArtistInput) (model.Artist, error) {
			return model.Artist{ID: 3, Name: in.Name, Bio: in.Bio}, nil
		},
		get: func(id int64) (model.Artist, error) {
			if id == 1 {
				return model.Artist{ID: 1, Name: "Miles Davis"}, nil
			}
			return model.Artist{}, repository.ErrNotFound
		},
		list: func() ([]model.Artist, error) { return []model.Artist{}, nil },
	}
	h := NewArtistHandler(fake)
	routes := func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api/v1/artists", h.Create)
		mux.HandleFunc("GET /api/v1/artists", h.List)
		mux.HandleFunc("GET /api/v1/artists/{id}", h.Get)
	}

	rec := serve(routes, http.MethodPost, "/api/v1/artists", `{"name":"Nirvana"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"bio":null`) {
		t.Errorf("create: expected 201 with bio null, got %d %s", rec.Code, rec.Body.String())
	}
	expectError(t, serve(routes, http.MethodPost, "/api/v1/artists", `{"name":""}`), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, serve(routes, http.MethodPost, "/api/v1/artists", `{"name":"X","bio":7}`), http.StatusUnprocessableEntity, CodeValidation)
	rec = serve(routes, http.MethodGet, "/api/v1/artists/1", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"Miles Davis"`) {
		t.Errorf("get: expected 200, got %d %s", rec.Code, rec.Body.String())
	}
	rec = serve(routes, http.MethodGet, "/api/v1/artists/999999", "")
	expectError(t, rec, http.StatusNotFound, CodeNotFound)
	if !strings.Contains(rec.Body.String(), "Artist not found") {
		t.Errorf("expected an artist-specific message, got %s", rec.Body.String())
	}
	rec = serve(routes, http.MethodGet, "/api/v1/artists", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Errorf("list: expected 200 [], got %d %q", rec.Code, rec.Body.String())
	}
}
