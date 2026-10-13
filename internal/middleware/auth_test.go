package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"homework/internal/auth"
	"homework/internal/handler"
)

const testSecret = "middleware-test-secret"

// errorCodeOf decodes the error envelope of a rejected request.
func errorCodeOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var env handler.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Error.Message == "" {
		t.Fatalf("not an error envelope: %s (%v)", w.Body, err)
	}
	return env.Error.Code
}

func TestAuthRejectsBadCredentials(t *testing.T) {
	expired, _ := auth.IssueToken(testSecret, 1, "member", -time.Minute)
	other, _ := auth.IssueToken("other-secret", 1, "member", time.Hour)
	cases := map[string]string{
		"no header":            "",
		"basic scheme":         "Basic cmVhZGVyOnNlY3JldA==",
		"bearer without token": "Bearer ",
		"bearer only":          "Bearer",
		"not a jwt":            "Bearer not.a.jwt",
		"expired":              "Bearer " + expired,
		"other secret":         "Bearer " + other,
	}
	for name, header := range cases {
		called := false
		h := Auth(testSecret)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized || called {
			t.Errorf("%s: want 401 without reaching the handler, got %d (called=%v)", name, w.Code, called)
			continue
		}
		if code := errorCodeOf(t, w); code != handler.CodeUnauthenticated {
			t.Errorf("%s: want code %q, got %q", name, handler.CodeUnauthenticated, code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("%s: want a JSON error, got %q", name, ct)
		}
	}
}

func TestAuthStoresClaimsAndCallsNext(t *testing.T) {
	token, err := auth.IssueToken(testSecret, 42, "staff", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, header := range []string{"Bearer " + token, "bearer " + token, "Bearer   " + token} {
		var got auth.Claims
		var ok bool
		h := Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, ok = auth.UserFromContext(r.Context())
			w.WriteHeader(http.StatusNoContent)
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", header)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent || !ok || got != (auth.Claims{UserID: 42, Role: "staff"}) {
			t.Errorf("%q: want 204 with the claims in the context, got %d %+v (ok=%v)", header, w.Code, got, ok)
		}
	}
}

func TestRequireRole(t *testing.T) {
	cases := []struct {
		name   string
		claims *auth.Claims
		status int
		code   string
	}{
		{"anonymous", nil, http.StatusUnauthorized, handler.CodeUnauthenticated},
		{"member", &auth.Claims{UserID: 1, Role: "member"}, http.StatusForbidden, handler.CodeForbidden},
		{"staff", &auth.Claims{UserID: 2, Role: "staff"}, http.StatusCreated, ""},
		{"admin", &auth.Claims{UserID: 3, Role: "admin"}, http.StatusCreated, ""},
	}
	for _, tc := range cases {
		called := false
		h := RequireRole("staff", "admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusCreated)
		}))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/genres", nil)
		if tc.claims != nil {
			req = req.WithContext(auth.WithUser(req.Context(), *tc.claims))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Errorf("%s: want %d, got %d: %s", tc.name, tc.status, w.Code, w.Body)
			continue
		}
		if tc.code == "" {
			if !called {
				t.Errorf("%s: the handler must run", tc.name)
			}
			continue
		}
		if called {
			t.Errorf("%s: the handler must not run on a rejected request", tc.name)
		}
		if code := errorCodeOf(t, w); code != tc.code {
			t.Errorf("%s: want code %q, got %q", tc.name, tc.code, code)
		}
	}
}

func TestCORSAllowsTheAuthorizationHeader(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/me", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	CORS(okHandler()).ServeHTTP(w, req)

	allowed := strings.ToLower(w.Header().Get("Access-Control-Allow-Headers"))
	if w.Code != http.StatusNoContent || !strings.Contains(allowed, "authorization") || !strings.Contains(allowed, "content-type") {
		t.Errorf("preflight must allow Authorization and Content-Type, got %d %q", w.Code, allowed)
	}
}
