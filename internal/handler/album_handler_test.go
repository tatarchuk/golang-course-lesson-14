package handler

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
)

func albumRoutes(h *AlbumHandler) func(*http.ServeMux) {
	return func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api/v1/albums", h.Create)
		mux.HandleFunc("GET /api/v1/albums", h.List)
		mux.HandleFunc("GET /api/v1/albums/{id}", h.Get)
		mux.HandleFunc("PUT /api/v1/albums/{id}", h.Update)
		mux.HandleFunc("DELETE /api/v1/albums/{id}", h.Delete)
	}
}

func TestAlbumCreateReturns201WithTheStoredAlbum(t *testing.T) {
	var received model.AlbumInput
	fake := &fakeAlbums{t: t, create: func(in model.AlbumInput) (model.Album, error) {
		received = in
		return model.Album{ID: 7, Title: in.Title, Genre: model.Genre{ID: 1, Name: "jazz"}, Artists: []model.Artist{{ID: 1}, {ID: 2}}}, nil
	}}
	rec := serve(albumRoutes(NewAlbumHandler(fake)), http.MethodPost, "/api/v1/albums", validAlbumBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if received.GenreID != 1 || len(received.ArtistIDs) != 2 || received.ReleaseYear == nil {
		t.Errorf("input not passed through: %+v", received)
	}
	for _, want := range []string{`"id":7`, `"genre":{"id":1,"name":"jazz"`, `"artists":[{"id":1`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body %s should contain %s", rec.Body.String(), want)
		}
	}
}

func TestAlbumCreateRejectsInvalidBodiesBeforeStorage(t *testing.T) {
	bodies := map[string]string{
		"missing title":    `{"label":"Columbia","genre_id":1,"artist_ids":[1]}`,
		"blank label":      `{"title":"T","label":"  ","genre_id":1,"artist_ids":[1]}`,
		"no artists":       `{"title":"T","label":"L","genre_id":1,"artist_ids":[]}`,
		"zero genre":       `{"title":"T","label":"L","genre_id":0,"artist_ids":[1]}`,
		"wrong type":       `{"title":"T","label":"L","genre_id":"1","artist_ids":[1]}`,
		"broken json":      `{not json`,
		"empty body":       ``,
		"array body":       `[1,2]`,
		"trailing garbage": validAlbumBody + ` {}`,
	}
	for name, body := range bodies {
		fake := &fakeAlbums{t: t} // every call fails the test
		rec := serve(albumRoutes(NewAlbumHandler(fake)), http.MethodPost, "/api/v1/albums", body)
		if rec.Code != http.StatusUnprocessableEntity || errorCode(t, rec) != CodeValidation {
			t.Errorf("%s: expected 422 validation_error, got %d %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestAlbumCreateMapsRepositoryErrors(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{repository.ErrInvalidReference, http.StatusUnprocessableEntity, CodeInvalidReference},
		{repository.ErrConflict, http.StatusConflict, CodeConflict},
		{errors.New("connection reset"), http.StatusInternalServerError, CodeInternal},
	}
	for _, c := range cases {
		fake := &fakeAlbums{t: t, create: func(model.AlbumInput) (model.Album, error) { return model.Album{}, c.err }}
		rec := serve(albumRoutes(NewAlbumHandler(fake)), http.MethodPost, "/api/v1/albums", validAlbumBody)
		expectError(t, rec, c.status, c.code)
		if c.status == http.StatusInternalServerError && strings.Contains(rec.Body.String(), "connection reset") {
			t.Errorf("internal details must not leak: %s", rec.Body.String())
		}
	}
}

func TestAlbumGetValidatesIDAndMapsNotFound(t *testing.T) {
	fake := &fakeAlbums{t: t, get: func(id int64) (model.Album, error) {
		if id == 1 {
			return model.Album{ID: 1, Title: "Kind of Blue", Artists: []model.Artist{}}, nil
		}
		return model.Album{}, repository.ErrNotFound
	}}
	routes := albumRoutes(NewAlbumHandler(fake))

	for _, bad := range []string{"abc", "-1", "1.5", "4294967296"} {
		expectError(t, serve(routes, http.MethodGet, "/api/v1/albums/"+bad, ""), http.StatusBadRequest, CodeInvalidID)
	}
	rec := serve(routes, http.MethodGet, "/api/v1/albums/999", "")
	expectError(t, rec, http.StatusNotFound, CodeNotFound)
	if !strings.Contains(rec.Body.String(), "Album not found") {
		t.Errorf("expected a resource-specific message, got %s", rec.Body.String())
	}
	rec = serve(routes, http.MethodGet, "/api/v1/albums/1", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"artists":[]`) {
		t.Errorf("expected 200 with an empty artists array, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAlbumListParsesPaginationAndGenreFilter(t *testing.T) {
	var got repository.ListOptions
	fake := &fakeAlbums{t: t, list: func(opts repository.ListOptions) ([]model.Album, error) {
		got = opts
		return []model.Album{}, nil
	}}
	routes := albumRoutes(NewAlbumHandler(fake))

	rec := serve(routes, http.MethodGet, "/api/v1/albums", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Fatalf("expected 200 [], got %d %q", rec.Code, rec.Body.String())
	}
	if got != (repository.ListOptions{Page: 1, Limit: 10}) {
		t.Errorf("defaults: got %+v", got)
	}
	serve(routes, http.MethodGet, "/api/v1/albums?page=2&limit=5&genre_id=3", "")
	if got != (repository.ListOptions{GenreID: 3, Page: 2, Limit: 5}) {
		t.Errorf("explicit values: got %+v", got)
	}
	serve(routes, http.MethodGet, "/api/v1/albums?limit=1000&genre_id=", "")
	if got.Limit != 100 || got.GenreID != 0 {
		t.Errorf("limit should be capped at 100 and an empty genre_id ignored: %+v", got)
	}
	for _, q := range []string{"page=0", "limit=abc", "page=-1", "limit=1.5"} {
		expectError(t, serve(routes, http.MethodGet, "/api/v1/albums?"+q, ""), http.StatusBadRequest, CodeInvalidPagination)
	}
	for _, q := range []string{"genre_id=abc", "genre_id=0", "genre_id=-5"} {
		expectError(t, serve(routes, http.MethodGet, "/api/v1/albums?"+q, ""), http.StatusBadRequest, CodeValidation)
	}
}

func TestAlbumUpdateChecksIDThenBodyThenExistence(t *testing.T) {
	calls := 0
	fake := &fakeAlbums{t: t, update: func(id int64, in model.AlbumInput) (model.Album, error) {
		calls++
		if id == 999 {
			return model.Album{}, repository.ErrNotFound
		}
		return model.Album{ID: id, Title: in.Title, Artists: []model.Artist{}}, nil
	}}
	routes := albumRoutes(NewAlbumHandler(fake))

	expectError(t, serve(routes, http.MethodPut, "/api/v1/albums/abc", `{not json`), http.StatusBadRequest, CodeInvalidID)
	expectError(t, serve(routes, http.MethodPut, "/api/v1/albums/999", `{not json`), http.StatusUnprocessableEntity, CodeValidation)
	if calls != 0 {
		t.Fatalf("storage must not be touched before the body is valid, got %d calls", calls)
	}
	expectError(t, serve(routes, http.MethodPut, "/api/v1/albums/999", validAlbumBody), http.StatusNotFound, CodeNotFound)
	rec := serve(routes, http.MethodPut, "/api/v1/albums/1", validAlbumBody)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":1`) {
		t.Errorf("expected 200 with the stored album, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAlbumDeleteReturns204OrNotFound(t *testing.T) {
	fake := &fakeAlbums{t: t, delete: func(id int64) error {
		if id == 1 {
			return nil
		}
		return repository.ErrNotFound
	}}
	routes := albumRoutes(NewAlbumHandler(fake))

	rec := serve(routes, http.MethodDelete, "/api/v1/albums/1", "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Errorf("expected an empty 204, got %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	expectError(t, serve(routes, http.MethodDelete, "/api/v1/albums/2", ""), http.StatusNotFound, CodeNotFound)
	expectError(t, serve(routes, http.MethodDelete, "/api/v1/albums/x", ""), http.StatusBadRequest, CodeInvalidID)
}
