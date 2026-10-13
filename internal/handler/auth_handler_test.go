package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"homework/internal/auth"
	"homework/internal/model"
	"homework/internal/repository"
)

const testSecret = "handler-test-secret"

// authRoutes registers the auth routes. GET /me runs with the given claims injected the
// way the Auth middleware stores them; nil claims simulate a request that bypassed Auth.
func authRoutes(h *AuthHandler, claims *auth.Claims) func(*http.ServeMux) {
	return func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api/v1/auth/register", h.Register)
		mux.HandleFunc("POST /api/v1/auth/login", h.Login)
		mux.HandleFunc("GET /api/v1/me", func(w http.ResponseWriter, r *http.Request) {
			if claims != nil {
				r = r.WithContext(auth.WithUser(r.Context(), *claims))
			}
			h.Me(w, r)
		})
	}
}

// assertNoSecrets fails when a success response carries password material.
func assertNoSecrets(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := strings.ToLower(rec.Body.String())
	for _, needle := range []string{"password", "hash", "$2a$"} {
		if strings.Contains(body, needle) {
			t.Errorf("response must not expose password material (%q): %s", needle, rec.Body)
		}
	}
}

func TestAuthHandlerRegister(t *testing.T) {
	var gotEmail, gotHash string
	fake := &fakeUsers{
		t: t,
		create: func(email, hash string) (model.User, error) {
			gotEmail, gotHash = email, hash
			if email == "taken@albums.local" {
				return model.User{}, repository.ErrConflict
			}
			return model.User{ID: 7, Email: email, Role: model.UserRoleMember, PasswordHash: hash}, nil
		},
	}
	routes := authRoutes(NewAuthHandler(fake, testSecret), nil)

	rec := serve(routes, http.MethodPost, "/api/v1/auth/register", `{"email":" Reader@Albums.LOCAL ","password":"Correct-Horse-Battery-1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"role":"member"`) || !strings.Contains(body, `"email":"reader@albums.local"`) || !strings.Contains(body, `"id":7`) {
		t.Errorf("register must answer with the member account: %s", body)
	}
	assertNoSecrets(t, rec)
	if gotEmail != "reader@albums.local" {
		t.Errorf("the e-mail must be normalized before storage, got %q", gotEmail)
	}
	if !auth.CheckPassword(gotHash, "Correct-Horse-Battery-1") {
		t.Errorf("the stored value must be a bcrypt hash of the password, got %q", gotHash)
	}

	rec = serve(routes, http.MethodPost, "/api/v1/auth/register", `{"email":"taken@albums.local","password":"Correct-Horse-Battery-1"}`)
	expectError(t, rec, http.StatusUnprocessableEntity, CodeValidation)
	if !strings.Contains(rec.Body.String(), "email already registered") {
		t.Errorf("duplicate e-mail message: %s", rec.Body)
	}

	expectError(t, serve(routes, http.MethodPost, "/api/v1/auth/register", `{not json`), http.StatusUnprocessableEntity, CodeValidation)

	fake.create = nil // a rejected body must never reach the repository
	expectError(t, serve(routes, http.MethodPost, "/api/v1/auth/register", `{"email":"short@albums.local","password":"short"}`), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, serve(routes, http.MethodPost, "/api/v1/auth/register", `{"email":"   ","password":"Correct-Horse-Battery-1"}`), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, serve(routes, http.MethodPost, "/api/v1/auth/register", `{"email":"nopass@albums.local"}`), http.StatusUnprocessableEntity, CodeValidation)
}

func TestAuthHandlerLogin(t *testing.T) {
	hash, err := auth.HashPassword("Correct-Horse-Battery-1")
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeUsers{
		t: t,
		getByEmail: func(email string) (model.User, error) {
			switch email {
			case "reader@albums.local":
				return model.User{ID: 7, Email: email, Role: model.UserRoleMember, PasswordHash: hash}, nil
			case "seed@albums.local": // a seed account before cmd/bootstrap: no password yet
				return model.User{ID: 1, Email: email, Role: model.UserRoleAdmin, PasswordHash: ""}, nil
			case "broken@albums.local":
				return model.User{}, errors.New("boom")
			}
			return model.User{}, repository.ErrNotFound
		},
	}
	routes := authRoutes(NewAuthHandler(fake, testSecret), nil)
	login := func(email, password string) *httptest.ResponseRecorder {
		return serve(routes, http.MethodPost, "/api/v1/auth/login", `{"email":"`+email+`","password":"`+password+`"}`)
	}

	rec := login("Reader@Albums.local", "Correct-Horse-Battery-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	var tok TokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tok); err != nil || tok.Token == "" {
		t.Fatalf("login must answer with a token: %s", rec.Body)
	}
	if claims, err := auth.ParseToken(testSecret, tok.Token); err != nil || claims != (auth.Claims{UserID: 7, Role: model.UserRoleMember}) {
		t.Errorf("token claims: %+v %v", claims, err)
	}

	wrong := login("reader@albums.local", "wrong-password")
	unknown := login("nobody@albums.local", "wrong-password")
	start := time.Now()
	noPassword := login("seed@albums.local", "Correct-Horse-Battery-1")
	elapsed := time.Since(start)
	for name, r := range map[string]*httptest.ResponseRecorder{"wrong password": wrong, "unknown e-mail": unknown, "account without a password": noPassword} {
		expectError(t, r, http.StatusUnauthorized, CodeUnauthenticated)
		if !strings.Contains(r.Body.String(), "invalid credentials") {
			t.Errorf("%s: %s", name, r.Body)
		}
	}
	if wrong.Body.String() != unknown.Body.String() || wrong.Body.String() != noPassword.Body.String() {
		t.Errorf("every failure must answer with the same body:\n%s\n%s\n%s", wrong.Body, unknown.Body, noPassword.Body)
	}
	if elapsed < 20*time.Millisecond {
		t.Errorf("an account without a password must still cost one bcrypt comparison, took %s", elapsed)
	}

	expectError(t, login("reader@albums.local", ""), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, login("broken@albums.local", "Correct-Horse-Battery-1"), http.StatusInternalServerError, CodeInternal)
}

func TestAuthHandlerMe(t *testing.T) {
	fake := &fakeUsers{
		t: t,
		get: func(id int64) (model.User, error) {
			if id == 7 {
				return model.User{ID: 7, Email: "reader@albums.local", Role: model.UserRoleMember, PasswordHash: "$2a$10$secret"}, nil
			}
			return model.User{}, repository.ErrNotFound
		},
	}
	h := NewAuthHandler(fake, testSecret)

	rec := serve(authRoutes(h, &auth.Claims{UserID: 7, Role: model.UserRoleMember}), http.MethodGet, "/api/v1/me", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"email":"reader@albums.local"`) {
		t.Errorf("GET /me must answer with the token's account: %d %s", rec.Code, rec.Body)
	}
	assertNoSecrets(t, rec)

	// A valid token for a deleted account is no longer a credential.
	expectError(t, serve(authRoutes(h, &auth.Claims{UserID: 99, Role: model.UserRoleMember}), http.MethodGet, "/api/v1/me", ""), http.StatusUnauthorized, CodeUnauthenticated)
	// The handler must not trust a request that somehow bypassed Auth.
	expectError(t, serve(authRoutes(h, nil), http.MethodGet, "/api/v1/me", ""), http.StatusUnauthorized, CodeUnauthenticated)
}
