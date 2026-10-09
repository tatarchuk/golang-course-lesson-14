package handler

import (
	"net/http"
	"strings"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
)

func copyRoutes(h *CopyHandler) func(*http.ServeMux) {
	return func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api/v1/albums/{id}/copies", h.Create)
		mux.HandleFunc("GET /api/v1/albums/{id}/copies", h.List)
		mux.HandleFunc("GET /api/v1/albums/{id}/availability", h.Availability)
	}
}

func TestCopyHandlerCreate(t *testing.T) {
	var gotAlbum int64
	var gotInput model.CopyInput
	fake := &fakeCopies{t: t, create: func(albumID int64, in model.CopyInput) (model.Copy, error) {
		gotAlbum, gotInput = albumID, in
		switch {
		case albumID == 999:
			return model.Copy{}, repository.ErrNotFound
		case in.InventoryNo == "DUP":
			return model.Copy{}, repository.ErrConflict
		}
		return model.Copy{ID: 1, AlbumID: albumID, InventoryNo: in.InventoryNo, Status: in.StatusOrDefault()}, nil
	}}
	routes := copyRoutes(NewCopyHandler(fake))

	rec := serve(routes, http.MethodPost, "/api/v1/albums/1/copies", `{"inventory_no":"SMOKE-1"}`)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"status":"available"`) {
		t.Errorf("create: expected 201 with the default status, got %d %s", rec.Code, rec.Body.String())
	}
	if gotAlbum != 1 || gotInput.InventoryNo != "SMOKE-1" {
		t.Errorf("input not passed through: album %d input %+v", gotAlbum, gotInput)
	}
	rec = serve(routes, http.MethodPost, "/api/v1/albums/999/copies", `{"inventory_no":"SMOKE-X"}`)
	expectError(t, rec, http.StatusNotFound, CodeNotFound)
	if !strings.Contains(rec.Body.String(), "Album not found") {
		t.Errorf("unknown album should name the album, got %s", rec.Body.String())
	}
	expectError(t, serve(routes, http.MethodPost, "/api/v1/albums/1/copies", `{"inventory_no":"DUP"}`), http.StatusConflict, CodeConflict)
	expectError(t, serve(routes, http.MethodPost, "/api/v1/albums/1/copies", `{"inventory_no":"SMOKE-4","status":"broken"}`), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, serve(routes, http.MethodPost, "/api/v1/albums/1/copies", `{}`), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, serve(routes, http.MethodPost, "/api/v1/albums/abc/copies", `{"inventory_no":"X"}`), http.StatusBadRequest, CodeInvalidID)
}

func TestCopyHandlerListAndAvailability(t *testing.T) {
	fake := &fakeCopies{
		t: t,
		listByAlbum: func(albumID int64) ([]model.Copy, error) {
			if albumID == 999 {
				return nil, repository.ErrNotFound
			}
			return []model.Copy{{ID: 1, AlbumID: albumID, InventoryNo: "A", Status: "available"}}, nil
		},
		availability: func(albumID int64) (model.Availability, error) {
			if albumID == 999 {
				return model.Availability{}, repository.ErrNotFound
			}
			return model.Availability{AlbumID: albumID, TotalCopies: 3, AvailableCopies: 2}, nil
		},
	}
	routes := copyRoutes(NewCopyHandler(fake))

	rec := serve(routes, http.MethodGet, "/api/v1/albums/1/copies", "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), `[{"id":1,"album_id":1`) {
		t.Errorf("list: expected 200 with an array, got %d %s", rec.Code, rec.Body.String())
	}
	expectError(t, serve(routes, http.MethodGet, "/api/v1/albums/999/copies", ""), http.StatusNotFound, CodeNotFound)
	rec = serve(routes, http.MethodGet, "/api/v1/albums/1/availability", "")
	if rec.Code != http.StatusOK || rec.Body.String() != `{"album_id":1,"total_copies":3,"available_copies":2}`+"\n" {
		t.Errorf("availability: expected the summary object, got %d %q", rec.Code, rec.Body.String())
	}
	expectError(t, serve(routes, http.MethodGet, "/api/v1/albums/999/availability", ""), http.StatusNotFound, CodeNotFound)
	expectError(t, serve(routes, http.MethodGet, "/api/v1/albums/x/availability", ""), http.StatusBadRequest, CodeInvalidID)
}
