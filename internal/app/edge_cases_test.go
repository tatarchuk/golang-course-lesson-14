package app_test

import (
	"net/http"
	"testing"
)

// TestContractEdgeCases covers boundary cases from the README that the staged tests leave out:
// the uint32 id range, wrong JSON types, non-object bodies, huge pagination values and the
// JSON catch-all. It reuses the helpers from helpers_test.go.
func TestContractEdgeCases(t *testing.T) {
	paths := newEnv(t) // used only to build variant-specific paths
	rest := `"artist":"b","label":"c","genre":"d"`

	cases := []struct {
		name, method, path, body string
		status                   int
		code                     string // "" means only the status is checked
	}{
		{"id 0 is valid but missing", http.MethodGet, paths.item(0), "", 404, "not_found"},
		{"id max uint32", http.MethodGet, paths.item("4294967295"), "", 404, "not_found"},
		{"id max uint32 plus one", http.MethodGet, paths.item("4294967296"), "", 400, "invalid_id"},
		{"fraction for int", http.MethodPost, paths.base(), `{"title":"a",` + rest + `,"release_year":2015.5}`, 422, "validation_error"},
		{"string for int", http.MethodPost, paths.base(), `{"title":"a",` + rest + `,"release_year":"2015"}`, 422, "validation_error"},
		{"number for text", http.MethodPost, paths.base(), `{"title":"a",` + rest + `,"notes":1}`, 422, "validation_error"},
		{"null required field", http.MethodPost, paths.base(), `{"title":null,` + rest + `}`, 422, "validation_error"},
		{"whitespace required field", http.MethodPost, paths.base(), `{"title":"  ",` + rest + `}`, 422, "validation_error"},
		{"number required field", http.MethodPost, paths.base(), `{"title":1,` + rest + `}`, 422, "validation_error"},
		{"null body", http.MethodPost, paths.base(), `null`, 422, "validation_error"},
		{"string body", http.MethodPost, paths.base(), `"x"`, 422, "validation_error"},
		{"trailing garbage", http.MethodPost, paths.base(), `{"title":"a",` + rest + `} x`, 422, "validation_error"},
		{"null optional is accepted", http.MethodPost, paths.base(), `{"title":"a",` + rest + `,"release_year":null}`, 201, ""},
		{"page beyond int64", http.MethodGet, paths.base() + "?page=99999999999999999999", "", 200, ""},
		{"page max int64", http.MethodGet, paths.base() + "?page=9223372036854775807", "", 200, ""},
		{"limit beyond int64", http.MethodGet, paths.base() + "?limit=99999999999999999999", "", 200, ""},
		{"negative huge page", http.MethodGet, paths.base() + "?page=-99999999999999999999", "", 400, "invalid_pagination"},
		{"unsupported method", http.MethodPatch, paths.item(1), "", 404, "not_found"},
		{"trailing slash", http.MethodGet, paths.base() + "/", "", 404, "not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.create(e.valid()) // album 1 exists in every case

			r := e.do(tc.method, tc.path, tc.body)

			if tc.code != "" {
				e.wantError(r, tc.status, tc.code)
			} else {
				e.expect(r, tc.status)
			}
			if !isJSONContent(r) {
				t.Errorf("Content-Type must be application/json, got %q", r.Header.Get("Content-Type"))
			}
		})
	}
}

func TestHugePaginationValuesAreHarmless(t *testing.T) {
	e := newEnv(t)
	e.create(e.payload("a", "x"))
	e.create(e.payload("b", "x"))

	if r := e.do(http.MethodGet, e.base()+"?page=9223372036854775807", ""); r.text() != "[]" {
		t.Errorf("a page far past the end must be [], got %s", r.Body)
	}
	if r := e.do(http.MethodGet, e.base()+"?limit=99999999999999999999", ""); len(r.list(t)) != 2 {
		t.Errorf("an oversized limit is clamped and still returns the data, got %s", r.Body)
	}
}

func TestRejectedPutChangesNothing(t *testing.T) {
	e := newEnv(t)
	id := idOf(t, e.create(e.valid()))

	e.wantError(e.do(http.MethodPut, e.item(id), `{"title":5}`), http.StatusUnprocessableEntity, "validation_error")

	if got := e.do(http.MethodGet, e.item(id), "").obj(t); got[e.v.Required[0]] != "title-x" {
		t.Errorf("a PUT rejected by validation must not change the album, got %v", got)
	}
}
