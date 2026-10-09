package app_test

import (
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestHealthSwaggerAndJSONFallbacks(t *testing.T) {
	h := newRouter(t)

	r := call(h, "GET", "/health", "")
	if r.Code != 200 || strings.TrimSpace(string(r.Body)) != `{"status":"ok"}` {
		t.Errorf("health: got %s", r)
	}
	if r := call(h, "GET", "/swagger/index.html", ""); r.Code != 200 {
		t.Errorf("swagger ui: expected 200, got %d", r.Code)
	}
	expect(t, call(h, "GET", "/no-such-route", ""), 404, "not_found")
	expect(t, call(h, "PATCH", "/api/v1/albums/1", ""), 404, "not_found")
	if ct := call(h, "GET", "/no-such-route", "").Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("fallback 404 must be JSON, got %q", ct)
	}
}

func TestCORSPreflightAndHeaders(t *testing.T) {
	h := newRouter(t)

	req := call(h, "OPTIONS", "/api/v1/albums", "")
	if req.Code != 204 || req.Header.Get("Access-Control-Allow-Origin") == "" || !strings.Contains(req.Header.Get("Access-Control-Allow-Methods"), "POST") {
		t.Errorf("preflight: got %d %v", req.Code, req.Header)
	}
	if call(h, "GET", "/health", "").Header.Get("Access-Control-Allow-Origin") == "" {
		t.Error("every response must carry Access-Control-Allow-Origin")
	}
}

func TestGenresAndArtistsEndpoints(t *testing.T) {
	h := newRouter(t)

	genre := expect(t, call(h, "POST", "/api/v1/genres", `{"name":"Smoke Genre"}`), 201, "")
	expect(t, call(h, "POST", "/api/v1/genres", `{"name":"Smoke Genre"}`), 409, "conflict")
	expect(t, call(h, "POST", "/api/v1/genres", `{}`), 422, "validation_error")
	got := expect(t, call(h, "GET", "/api/v1/genres/1", ""), 200, "")
	if got["name"] != "Smoke Genre" || id(got) != id(genre) {
		t.Errorf("get genre: %v", got)
	}
	if n := len(list(t, call(h, "GET", "/api/v1/genres", ""))); n != 1 {
		t.Errorf("expected 1 genre, got %d", n)
	}

	a1 := expect(t, call(h, "POST", "/api/v1/artists", `{"name":"Smoke Artist"}`), 201, "")
	expect(t, call(h, "POST", "/api/v1/artists", `{"name":"Smoke Artist Two","bio":"second"}`), 201, "")
	got = expect(t, call(h, "GET", "/api/v1/artists/1", ""), 200, "")
	if id(got) != id(a1) || got["bio"] != nil {
		t.Errorf("get artist: %v", got)
	}
	if n := len(list(t, call(h, "GET", "/api/v1/artists", ""))); n != 2 {
		t.Errorf("expected 2 artists, got %d", n)
	}
	expect(t, call(h, "GET", "/api/v1/artists/999999", ""), 404, "not_found")
}

func TestAlbumsEndpoints(t *testing.T) {
	h := newRouter(t)
	genre, artists := fixtures(t, h)

	body := albumBody("Smoke Album", genre, artists[0])
	created := expect(t, call(h, "POST", "/api/v1/albums", body), 201, "")
	album := id(created)
	if arts := created["artists"].([]any); len(arts) != 1 {
		t.Errorf("created album should contain its artist: %v", created)
	}
	if g := created["genre"].(map[string]any); int64(g["id"].(float64)) != genre {
		t.Errorf("created album should contain its genre: %v", created)
	}
	for _, key := range []string{"release_year", "notes"} {
		if v, ok := created[key]; !ok || v != nil {
			t.Errorf("optional %s should be present and null: %v", key, created)
		}
	}

	expect(t, call(h, "POST", "/api/v1/albums", body), 409, "conflict")
	expect(t, call(h, "POST", "/api/v1/albums", albumBody("Bad Artist", genre, 999999)), 422, "invalid_reference")
	expect(t, call(h, "GET", "/api/v1/artists/999999", ""), 404, "not_found")
	expect(t, call(h, "POST", "/api/v1/albums", albumBody("Bad Genre", 999999, artists[0])), 422, "invalid_reference")
	expect(t, call(h, "POST", "/api/v1/albums", `{"label":"Smoke Label"}`), 422, "validation_error")
	expect(t, call(h, "POST", "/api/v1/albums", albumBody("Bad Empty", genre)), 422, "validation_error")
	if n := countRows(t, "albums", "1 = 1"); n != 1 {
		t.Errorf("rejected requests must not create albums, found %d", n)
	}

	got := expect(t, call(h, "GET", "/api/v1/albums/1", ""), 200, "")
	if len(got["artists"].([]any)) != 1 {
		t.Errorf("get should return artists: %v", got)
	}
	expect(t, call(h, "GET", "/api/v1/albums/999999", ""), 404, "not_found")

	updated := expect(t, call(h, "PUT", "/api/v1/albums/1", albumBody("Smoke Album Updated", genre, artists[1])), 200, "")
	arts := updated["artists"].([]any)
	if updated["title"] != "Smoke Album Updated" || len(arts) != 1 || int64(arts[0].(map[string]any)["id"].(float64)) != artists[1] {
		t.Errorf("update should replace title and artists: %v", updated)
	}
	if updated["created_at"] != created["created_at"] {
		t.Errorf("created_at must survive a PUT: %v vs %v", updated["created_at"], created["created_at"])
	}
	expect(t, call(h, "PUT", "/api/v1/albums/999999", albumBody("X", genre, artists[0])), 404, "not_found")
	_ = album
}

func TestListFilterAndPagination(t *testing.T) {
	h := newRouter(t)
	genre, artists := fixtures(t, h)
	other := id(expect(t, call(h, "POST", "/api/v1/genres", `{"name":"Other"}`), 201, ""))
	for i, g := range []int64{genre, other, other} {
		expect(t, call(h, "POST", "/api/v1/albums", albumBody("Album "+string(rune('A'+i)), g, artists...)), 201, "")
	}

	filtered := list(t, call(h, "GET", "/api/v1/albums?genre_id=1", ""))
	if len(filtered) != 1 || filtered[0]["title"] != "Album A" {
		t.Errorf("genre filter: %v", filtered)
	}
	all := list(t, call(h, "GET", "/api/v1/albums?limit=100", ""))
	if len(all) != 3 {
		t.Errorf("expected 3 albums, got %d", len(all))
	}
	for _, a := range all {
		if a["genre"] == nil || len(a["artists"].([]any)) == 0 {
			t.Errorf("every listed album needs its genre and artists: %v", a)
		}
	}
	if page := list(t, call(h, "GET", "/api/v1/albums?limit=2&page=1", "")); len(page) != 2 || page[0]["title"] != "Album A" {
		t.Errorf("limit=2 page=1: %v", page)
	}
	if page := list(t, call(h, "GET", "/api/v1/albums?limit=2&page=2", "")); len(page) != 1 {
		t.Errorf("limit=2 page=2: %v", page)
	}
	expect(t, call(h, "GET", "/api/v1/albums?page=0", ""), 400, "invalid_pagination")
}

func TestCopiesAndAvailabilityEndpoints(t *testing.T) {
	h := newRouter(t)
	genre, artists := fixtures(t, h)
	expect(t, call(h, "POST", "/api/v1/albums", albumBody("Smoke Album", genre, artists[0])), 201, "")

	first := expect(t, call(h, "POST", "/api/v1/albums/1/copies", `{"inventory_no":"SMOKE-1"}`), 201, "")
	if first["status"] != "available" || int64(first["album_id"].(float64)) != 1 {
		t.Errorf("first copy: %v", first)
	}
	expect(t, call(h, "POST", "/api/v1/albums/1/copies", `{"inventory_no":"SMOKE-2","status":"available"}`), 201, "")
	expect(t, call(h, "POST", "/api/v1/albums/1/copies", `{"inventory_no":"SMOKE-3","status":"lost"}`), 201, "")
	expect(t, call(h, "POST", "/api/v1/albums/1/copies", `{"inventory_no":"SMOKE-1"}`), 409, "conflict")
	expect(t, call(h, "POST", "/api/v1/albums/999999/copies", `{"inventory_no":"SMOKE-X"}`), 404, "not_found")
	expect(t, call(h, "POST", "/api/v1/albums/1/copies", `{"inventory_no":"SMOKE-4","status":"broken"}`), 422, "validation_error")

	if copies := list(t, call(h, "GET", "/api/v1/albums/1/copies", "")); len(copies) != 3 {
		t.Errorf("expected 3 copies, got %v", copies)
	}
	av := expect(t, call(h, "GET", "/api/v1/albums/1/availability", ""), 200, "")
	if av["album_id"] != 1.0 || av["total_copies"] != 3.0 || av["available_copies"] != 2.0 {
		t.Errorf("availability: %v", av)
	}
	expect(t, call(h, "GET", "/api/v1/albums/999999/availability", ""), 404, "not_found")
}

func TestAlbumDeleteCascades(t *testing.T) {
	h := newRouter(t)
	genre, artists := fixtures(t, h)
	expect(t, call(h, "POST", "/api/v1/albums", albumBody("Gone", genre, artists...)), 201, "")
	expect(t, call(h, "POST", "/api/v1/albums/1/copies", `{"inventory_no":"SMOKE-1"}`), 201, "")

	expect(t, call(h, "DELETE", "/api/v1/albums/1", ""), 204, "")
	expect(t, call(h, "GET", "/api/v1/albums/1", ""), 404, "not_found")
	if n := countRows(t, "copies", "album_id = ?", 1); n != 0 {
		t.Errorf("copies should cascade, found %d", n)
	}
	if n := countRows(t, "album_artists", "album_id = ?", 1); n != 0 {
		t.Errorf("artist links should cascade, found %d", n)
	}
	expect(t, call(h, "DELETE", "/api/v1/albums/999999", ""), 404, "not_found")
	if n := countRows(t, "artists", "1 = 1"); n != 2 {
		t.Errorf("deleting an album must not delete artists, found %d", n)
	}
}

// Edge cases carried over from the Theme 7 contract.
func TestContractEdgeCases(t *testing.T) {
	h := newRouter(t)
	genre, artists := fixtures(t, h)
	body := albumBody("Edge", genre, artists[0])
	expect(t, call(h, "POST", "/api/v1/albums", body), 201, "")

	for _, bad := range []string{"abc", "-1", "1.5", "4294967296"} {
		expect(t, call(h, "GET", "/api/v1/albums/"+bad, ""), 400, "invalid_id")
	}
	expect(t, call(h, "GET", "/api/v1/albums/0", ""), 404, "not_found")
	for _, q := range []string{"page=9223372036854775807", "limit=99999999999999999999", "page=" + strings.Repeat("9", 30)} {
		r := call(h, "GET", "/api/v1/albums?"+q, "")
		if r.Code != http.StatusOK {
			t.Errorf("%s: huge values must not fail, got %s", q, r)
		}
	}
	if r := call(h, "GET", "/api/v1/albums?page=1&limit=100", ""); len(list(t, r)) != 1 {
		t.Errorf("normal list after huge values: %s", r)
	}
	expect(t, call(h, "POST", "/api/v1/albums", body+` {}`), 422, "validation_error")
	expect(t, call(h, "POST", "/api/v1/albums", `[1,2,3]`), 422, "validation_error")
	expect(t, call(h, "PUT", "/api/v1/albums/abc", `{not json`), 400, "invalid_id")
	expect(t, call(h, "PUT", "/api/v1/albums/999", `{not json`), 422, "validation_error")

	// A rejected PUT changes nothing.
	before := expect(t, call(h, "GET", "/api/v1/albums/1", ""), 200, "")
	expect(t, call(h, "PUT", "/api/v1/albums/1", `{"title":"Changed"}`), 422, "validation_error")
	expect(t, call(h, "PUT", "/api/v1/albums/1", albumBody("Changed", genre, 999999)), 422, "invalid_reference")
	after := expect(t, call(h, "GET", "/api/v1/albums/1", ""), 200, "")
	if after["title"] != before["title"] || after["updated_at"] != before["updated_at"] {
		t.Errorf("rejected PUT must change nothing: %v -> %v", before, after)
	}
	_ = math.MaxInt64
}
