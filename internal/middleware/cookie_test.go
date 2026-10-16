package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"homework/internal/auth"
	"homework/internal/handler"
)

// claimsRecorder is a terminal handler that records whether it ran and which claims it
// found in the request context.
func claimsRecorder(called *bool, got *auth.Claims, found *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		*got, *found = auth.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func TestCookieAuthIdentifiesTheVisitorFromTheCookieOnly(t *testing.T) {
	token, _ := auth.IssueToken(testSecret, 42, "staff", time.Hour)
	expired, _ := auth.IssueToken(testSecret, 42, "staff", -time.Minute)
	other, _ := auth.IssueToken("other-secret", 42, "staff", time.Hour)
	cases := []struct {
		name       string
		cookie     *http.Cookie
		header     string
		wantClaims bool
	}{
		{"a valid cookie puts the claims in the context", &http.Cookie{Name: "session", Value: token}, "", true},
		{"no cookie means an anonymous request", nil, "", false},
		{"an expired cookie means an anonymous request", &http.Cookie{Name: "session", Value: expired}, "", false},
		{"a cookie signed with another secret is ignored", &http.Cookie{Name: "session", Value: other}, "", false},
		{"a cookie under another name is ignored", &http.Cookie{Name: "other", Value: token}, "", false},
		{"a garbage cookie is ignored", &http.Cookie{Name: "session", Value: "not.a.jwt"}, "", false},
		{"the Authorization header is never a web session", nil, "Bearer " + token, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var called, found bool
			var got auth.Claims
			h := CookieAuth(testSecret, "session")(claimsRecorder(&called, &got, &found))
			req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if !called || w.Code != http.StatusOK {
				t.Fatalf("next must always run and answer itself: called=%v status=%d body=%s", called, w.Code, w.Body)
			}
			if found != tc.wantClaims {
				t.Fatalf("claims in context = %v, want %v", found, tc.wantClaims)
			}
			if tc.wantClaims && (got.UserID != 42 || got.Role != "staff") {
				t.Errorf("claims = %+v, want user 42 with role staff", got)
			}
		})
	}
}

// TestCookieAuthWithRequireRole pins the pair the web routes use: the cookie identifies
// the visitor, the role check refuses, with the API's status codes and envelope.
func TestCookieAuthWithRequireRole(t *testing.T) {
	member, _ := auth.IssueToken(testSecret, 1, "member", time.Hour)
	staff, _ := auth.IssueToken(testSecret, 2, "staff", time.Hour)
	cases := []struct {
		name   string
		cookie *http.Cookie
		status int
		code   string
	}{
		{"anonymous", nil, http.StatusUnauthorized, handler.CodeUnauthenticated},
		{"member", &http.Cookie{Name: "session", Value: member}, http.StatusForbidden, handler.CodeForbidden},
		{"staff", &http.Cookie{Name: "session", Value: staff}, http.StatusOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := CookieAuth(testSecret, "session")(RequireRole("staff", "admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})))
			req := httptest.NewRequest(http.MethodDelete, "/web/albums/1", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status || called != (tc.status == http.StatusOK) {
				t.Fatalf("want %d (handler called=%v), got %d (called=%v) %s", tc.status, tc.status == http.StatusOK, w.Code, called, w.Body)
			}
			if tc.code != "" && errorCodeOf(t, w) != tc.code {
				t.Errorf("want error code %q, got %s", tc.code, w.Body)
			}
		})
	}
}
