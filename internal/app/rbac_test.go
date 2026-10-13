package app_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Lesson 9 acceptance tests for authorization. Roles of the albums variant: member
// (every new account), staff (catalogue writes) and admin (user management).
// An anonymous request to a protected route gets 401 unauthenticated, an authenticated
// user with the wrong role gets 403 forbidden, and a rejected write changes nothing.

var allRoles = []string{"anonymous", "member", "staff", "admin"}

// roleTokens registers one account per role and returns a bearer token for each.
// Registration always yields a member, so the staff and admin tokens are minted with
// the shared secret: the same thing the server does after PUT /users/{id}/role and a
// fresh login (TestRBACRoleChange covers that path). The anonymous "token" is empty.
func roleTokens(t *testing.T, h http.Handler) (tokens map[string]string, ids map[string]int64) {
	t.Helper()
	tokens = map[string]string{"anonymous": ""}
	ids = map[string]int64{}
	for _, role := range []string{"member", "staff", "admin"} {
		ids[role] = id(register(t, h, role+"@albums.local"))
		tokens[role] = mintToken(ids[role], role, time.Hour)
	}
	return tokens, ids
}

// errorCode is the error code an authorization failure must carry; "" for success.
func errorCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "unauthenticated"
	case http.StatusForbidden:
		return "forbidden"
	}
	return ""
}

// TestRBACMatrix runs every route of the access matrix as every role.
func TestRBACMatrix(t *testing.T) {
	h := newAuthRouter(t)
	tokens, ids := roleTokens(t, h)

	// Fixtures for the album routes, created as staff.
	genre := id(expect(t, authCall(h, "POST", "/api/v1/genres", `{"name":"Matrix Genre"}`, tokens["staff"]), http.StatusCreated, ""))
	artist := id(expect(t, authCall(h, "POST", "/api/v1/artists", `{"name":"Matrix Artist"}`, tokens["staff"]), http.StatusCreated, ""))
	newAlbum := func(t *testing.T, title string) string {
		t.Helper()
		r := authCall(h, "POST", "/api/v1/albums", albumBody(title, genre, artist), tokens["staff"])
		return fmt.Sprintf("/api/v1/albums/%d", id(expect(t, r, http.StatusCreated, "")))
	}
	fixed := func(path string) func(*testing.T, string) string {
		return func(*testing.T, string) string { return path }
	}

	tests := []struct {
		name   string
		method string
		path   func(t *testing.T, role string) string // the role keeps paths and bodies unique
		body   func(role string) string
		want   map[string]int // expected status per role
	}{
		{"public GET /albums", "GET", fixed("/api/v1/albums"), nil,
			map[string]int{"anonymous": 200, "member": 200, "staff": 200, "admin": 200}},
		{"public GET /genres", "GET", fixed("/api/v1/genres"), nil,
			map[string]int{"anonymous": 200, "member": 200, "staff": 200, "admin": 200}},
		{"authenticated GET /me", "GET", fixed("/api/v1/me"), nil,
			map[string]int{"anonymous": 401, "member": 200, "staff": 200, "admin": 200}},
		{"staff POST /genres", "POST", fixed("/api/v1/genres"),
			func(role string) string { return fmt.Sprintf(`{"name":"Genre by %s"}`, role) },
			map[string]int{"anonymous": 401, "member": 403, "staff": 201, "admin": 201}},
		{"staff POST /artists", "POST", fixed("/api/v1/artists"),
			func(role string) string { return fmt.Sprintf(`{"name":"Artist by %s"}`, role) },
			map[string]int{"anonymous": 401, "member": 403, "staff": 201, "admin": 201}},
		{"staff POST /albums", "POST", fixed("/api/v1/albums"),
			func(role string) string { return albumBody("Album by "+role, genre, artist) },
			map[string]int{"anonymous": 401, "member": 403, "staff": 201, "admin": 201}},
		{"staff PUT /albums/{id}", "PUT",
			func(t *testing.T, role string) string { return newAlbum(t, "Update by "+role) },
			func(role string) string { return albumBody("Updated by "+role, genre, artist) },
			map[string]int{"anonymous": 401, "member": 403, "staff": 200, "admin": 200}},
		{"staff DELETE /albums/{id}", "DELETE",
			func(t *testing.T, role string) string { return newAlbum(t, "Delete by "+role) }, nil,
			map[string]int{"anonymous": 401, "member": 403, "staff": 204, "admin": 204}},
		{"staff POST /albums/{id}/copies", "POST",
			func(t *testing.T, role string) string { return newAlbum(t, "Copies by "+role) + "/copies" },
			func(role string) string { return fmt.Sprintf(`{"inventory_no":"INV-%s"}`, role) },
			map[string]int{"anonymous": 401, "member": 403, "staff": 201, "admin": 201}},
		{"admin GET /users", "GET", fixed("/api/v1/users"), nil,
			map[string]int{"anonymous": 401, "member": 403, "staff": 403, "admin": 200}},
		{"admin PUT /users/{id}/role", "PUT", fixed(fmt.Sprintf("/api/v1/users/%d/role", ids["member"])),
			func(string) string { return `{"role":"member"}` },
			map[string]int{"anonymous": 401, "member": 403, "staff": 403, "admin": 200}},
	}
	for _, tt := range tests {
		for _, role := range allRoles {
			t.Run(tt.name+" as "+role, func(t *testing.T) {
				body := ""
				if tt.body != nil {
					body = tt.body(role)
				}
				r := authCall(h, tt.method, tt.path(t, role), body, tokens[role])
				expect(t, r, tt.want[role], errorCode(tt.want[role]))
			})
		}
	}

	// Only the staff and admin attempts went through.
	if n := countRows(t, "genres", "name LIKE ?", "Genre by %"); n != 2 {
		t.Errorf("rejected requests must not create genres: expected 2 rows, found %d", n)
	}
	if n := countRows(t, "copies", "inventory_no LIKE ?", "INV-%"); n != 2 {
		t.Errorf("rejected requests must not create copies: expected 2 rows, found %d", n)
	}
}

// TestRBACRoleChange is the admin path that turns a member into staff.
func TestRBACRoleChange(t *testing.T) {
	h := newAuthRouter(t)
	member := id(register(t, h, "promoted@albums.local"))
	admin := id(register(t, h, "boss@albums.local"))
	adminToken := mintToken(admin, "admin", time.Hour)
	memberToken := login(t, h, "promoted@albums.local")
	path := fmt.Sprintf("/api/v1/users/%d/role", member)

	expect(t, authCall(h, "POST", "/api/v1/genres", `{"name":"Too Early"}`, memberToken), http.StatusForbidden, "forbidden")

	expect(t, authCall(h, "PUT", path, `{"role":"boss"}`, adminToken), http.StatusUnprocessableEntity, "validation_error")
	expect(t, authCall(h, "PUT", path, `{}`, adminToken), http.StatusUnprocessableEntity, "validation_error")
	expect(t, authCall(h, "PUT", "/api/v1/users/999999/role", `{"role":"staff"}`, adminToken), http.StatusNotFound, "not_found")
	expect(t, authCall(h, "PUT", "/api/v1/users/abc/role", `{"role":"staff"}`, adminToken), http.StatusBadRequest, "invalid_id")

	r := authCall(h, "PUT", path, `{"role":"staff"}`, adminToken)
	updated := expect(t, r, http.StatusOK, "")
	if updated["role"] != "staff" || updated["id"] != float64(member) || updated["email"] != "promoted@albums.local" {
		t.Errorf("PUT /users/{id}/role must answer with the updated user: %s", r)
	}
	assertNoSecrets(t, r)

	// The change shows up in the next token, and the staff gate opens for it.
	staffToken := login(t, h, "promoted@albums.local")
	if claims := verifyJWT(t, staffToken, testJWTSecret); claims["role"] != "staff" {
		t.Errorf("a fresh login must carry the new role: %v", claims)
	}
	expect(t, authCall(h, "POST", "/api/v1/genres", `{"name":"Now Allowed"}`, staffToken), http.StatusCreated, "")
	expect(t, authCall(h, "GET", "/api/v1/users", "", staffToken), http.StatusForbidden, "forbidden")

	// Admins see every account, never a hash.
	r = authCall(h, "GET", "/api/v1/users", "", adminToken)
	if users := list(t, r); len(users) != 2 {
		t.Errorf("expected the 2 registered users, got %v", users)
	}
	assertNoSecrets(t, r)
}
