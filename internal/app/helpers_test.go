package app_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"homework/internal/app"
	"homework/internal/model"
	"homework/internal/repository"
	"homework/internal/testdb"
	"homework/web"
)

// newRouter resets the database and builds the full HTTP handler on top of it. The
// secret is set through the environment, as the Lesson 9 tests do, and read back the
// way main does; the web UI gets the same embedded files main passes (Lesson 10).
func newRouter(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv("JWT_SECRET", testJWTSecret)
	testdb.Reset(t)
	cfg := app.Config{JWTSecret: os.Getenv("JWT_SECRET"), Templates: web.Templates(), Static: web.Static()}
	return app.NewRouter(repository.NewGorm(testdb.DB(t)), cfg)
}

// staffCall serves one request with a staff token minted with the shared secret: since
// Lesson 9 the catalogue writes of the Theme 8 tests need it.
func staffCall(h http.Handler, method, path, body string) resp {
	return authCall(h, method, path, body, mintToken(1, model.UserRoleStaff, time.Hour))
}

type resp struct {
	Code   int
	Header http.Header
	Body   []byte
}

func (r resp) String() string { return fmt.Sprintf("%d %s", r.Code, strings.TrimSpace(string(r.Body))) }

// newRequest builds one request; a non-empty body is sent as JSON.
func newRequest(method, path, body string) *http.Request {
	if body == "" {
		return httptest.NewRequest(method, path, nil)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// serve runs one request through the handler and captures the reply.
func serve(h http.Handler, req *http.Request) resp {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return resp{Code: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes()}
}

// call serves one request without credentials.
func call(h http.Handler, method, path, body string) resp {
	return serve(h, newRequest(method, path, body))
}

// authCall serves one request with a bearer token; an empty token sends no Authorization header.
func authCall(h http.Handler, method, path, body, token string) resp {
	req := newRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return serve(h, req)
}

// expect asserts the status and, for errors, the error code.
func expect(t *testing.T, r resp, status int, code string) map[string]any {
	t.Helper()
	if r.Code != status {
		t.Fatalf("expected %d, got %s", status, r)
	}
	if status == http.StatusNoContent {
		return nil
	}
	if code == "" && bytes.HasPrefix(bytes.TrimSpace(r.Body), []byte("[")) {
		// A list answers with a JSON array: the caller asked for the status only (see list()).
		return nil
	}
	obj := object(t, r)
	if code != "" {
		e, _ := obj["error"].(map[string]any)
		if e["code"] != code || e["message"] == "" {
			t.Fatalf("expected error code %q, got %s", code, r)
		}
	}
	return obj
}

func object(t *testing.T, r resp) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(r.Body, &obj); err != nil {
		t.Fatalf("not a JSON object: %s (%v)", r, err)
	}
	return obj
}

func list(t *testing.T, r resp) []map[string]any {
	t.Helper()
	if r.Code != http.StatusOK {
		t.Fatalf("expected 200, got %s", r)
	}
	var items []map[string]any
	if err := json.Unmarshal(r.Body, &items); err != nil || items == nil {
		t.Fatalf("not a JSON array: %s (%v)", r, err)
	}
	return items
}

func id(obj map[string]any) int64 { return int64(obj["id"].(float64)) }

// fixtures creates one genre and two artists through the API and returns their ids.
func fixtures(t *testing.T, h http.Handler) (genre int64, artists []int64) {
	t.Helper()
	genre = id(expect(t, staffCall(h, "POST", "/api/v1/genres", `{"name":"Smoke Genre"}`), 201, ""))
	for _, name := range []string{"Smoke Artist", "Smoke Artist Two"} {
		artists = append(artists, id(expect(t, staffCall(h, "POST", "/api/v1/artists", fmt.Sprintf(`{"name":%q}`, name)), 201, "")))
	}
	return genre, artists
}

func albumBody(title string, genre int64, artists ...int64) string {
	ids := make([]string, len(artists))
	for i, a := range artists {
		ids[i] = fmt.Sprint(a)
	}
	return fmt.Sprintf(`{"title":%q,"label":"Smoke Label","genre_id":%d,"artist_ids":[%s]}`, title, genre, strings.Join(ids, ","))
}

func countRows(t *testing.T, table string, where string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := testdb.DB(t).Table(table).Where(where, args...).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
