package handler

import (
	"net/http"
	"strings"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
)

func userRoutes(h *UserHandler) func(*http.ServeMux) {
	return func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api/v1/users", h.List)
		mux.HandleFunc("PUT /api/v1/users/{id}/role", h.UpdateRole)
	}
}

func TestUserHandlerList(t *testing.T) {
	fake := &fakeUsers{t: t, list: func() ([]model.User, error) {
		return []model.User{{ID: 1, Email: "admin@albums.local", Role: model.UserRoleAdmin, PasswordHash: "$2a$10$secret"}}, nil
	}}
	routes := userRoutes(NewUserHandler(fake))

	rec := serve(routes, http.MethodGet, "/api/v1/users", "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), `[{"id":1,"email":"admin@albums.local","role":"admin"`) {
		t.Errorf("list: %d %s", rec.Code, rec.Body)
	}
	assertNoSecrets(t, rec)

	fake.list = func() ([]model.User, error) { return []model.User{}, nil }
	if rec := serve(routes, http.MethodGet, "/api/v1/users", ""); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("an empty list must encode as []: %s", rec.Body)
	}
}

func TestUserHandlerUpdateRole(t *testing.T) {
	fake := &fakeUsers{t: t, updateRole: func(id int64, role string) (model.User, error) {
		if id == 999 {
			return model.User{}, repository.ErrNotFound
		}
		return model.User{ID: id, Email: "promoted@albums.local", Role: role, PasswordHash: "$2a$10$secret"}, nil
	}}
	routes := userRoutes(NewUserHandler(fake))

	// Check order: id (400), then body (422), then existence (404).
	expectError(t, serve(routes, http.MethodPut, "/api/v1/users/abc/role", `{"role":"staff"}`), http.StatusBadRequest, CodeInvalidID)
	expectError(t, serve(routes, http.MethodPut, "/api/v1/users/999/role", `{"role":"staff"}`), http.StatusNotFound, CodeNotFound)

	rec := serve(routes, http.MethodPut, "/api/v1/users/7/role", `{"role":"staff"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"role":"staff"`) || !strings.Contains(rec.Body.String(), `"id":7`) {
		t.Errorf("update role: %d %s", rec.Code, rec.Body)
	}
	assertNoSecrets(t, rec)

	fake.updateRole = nil // an invalid body must never reach the repository
	expectError(t, serve(routes, http.MethodPut, "/api/v1/users/7/role", `{"role":"boss"}`), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, serve(routes, http.MethodPut, "/api/v1/users/7/role", `{}`), http.StatusUnprocessableEntity, CodeValidation)
	expectError(t, serve(routes, http.MethodPut, "/api/v1/users/7/role", `{not json`), http.StatusUnprocessableEntity, CodeValidation)
}
