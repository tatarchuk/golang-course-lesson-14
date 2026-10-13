package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// expectValidation asserts that err wraps ErrValidation and mentions want, or that err
// is nil when want is empty.
func expectValidation(t *testing.T, name string, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Errorf("%s: unexpected error %v", name, err)
		}
		return
	}
	if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), want) {
		t.Errorf("%s: expected ErrValidation mentioning %q, got %v", name, want, err)
	}
}

func TestRegisterInputValidate(t *testing.T) {
	cases := []struct {
		name string
		in   RegisterInput
		want string
	}{
		{"ok", RegisterInput{Email: "a@b.c", Password: "Correct-Horse-Battery-1"}, ""},
		{"blank email", RegisterInput{Email: "   ", Password: "Correct-Horse-Battery-1"}, "email"},
		{"missing password", RegisterInput{Email: "a@b.c"}, "password"},
		{"short password", RegisterInput{Email: "a@b.c", Password: "short"}, "between 8 and 72"},
		{"long password", RegisterInput{Email: "a@b.c", Password: strings.Repeat("x", 73)}, "between 8 and 72"},
	}
	for _, tc := range cases {
		expectValidation(t, tc.name, tc.in.Validate(), tc.want)
	}
}

func TestLoginInputValidate(t *testing.T) {
	cases := []struct {
		name string
		in   LoginInput
		want string
	}{
		{"ok", LoginInput{Email: "a@b.c", Password: "Correct-Horse-Battery-1"}, ""},
		{"short password is still a login attempt", LoginInput{Email: "a@b.c", Password: "x"}, ""},
		{"blank email", LoginInput{Email: " ", Password: "Correct-Horse-Battery-1"}, "email"},
		{"missing password", LoginInput{Email: "a@b.c"}, "password"},
		{"long password", LoginInput{Email: "a@b.c", Password: strings.Repeat("x", 73)}, "at most 72"},
	}
	for _, tc := range cases {
		expectValidation(t, tc.name, tc.in.Validate(), tc.want)
	}
}

func TestRoleInputValidate(t *testing.T) {
	for _, role := range []string{UserRoleMember, UserRoleStaff, UserRoleAdmin} {
		expectValidation(t, "role "+role, RoleInput{Role: role}.Validate(), "")
	}
	for _, role := range []string{"", "   ", "boss", "Admin"} {
		expectValidation(t, "role "+strings.TrimSpace(role), RoleInput{Role: role}.Validate(), "role")
	}
}

func TestValidRole(t *testing.T) {
	if !ValidRole("member") || !ValidRole("staff") || !ValidRole("admin") {
		t.Error("the three schema roles must be valid")
	}
	if ValidRole("") || ValidRole("boss") || ValidRole("Admin") {
		t.Error("anything outside the CHECK list must be invalid")
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail(" Reader@Albums.LOCAL "); got != "reader@albums.local" {
		t.Errorf("got %q", got)
	}
}

func TestUserJSONHidesPasswordHash(t *testing.T) {
	raw, err := json.Marshal(User{ID: 1, Email: "a@b.c", Role: UserRoleMember, PasswordHash: "$2a$10$secret"})
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(raw))
	if strings.Contains(s, "password") || strings.Contains(s, "hash") || strings.Contains(s, "$2a$") {
		t.Errorf("the hash leaked into the JSON: %s", raw)
	}
	for _, key := range []string{`"id":1`, `"email":"a@b.c"`, `"role":"member"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("expected %s in %s", key, raw)
		}
	}
}

func TestEmailShapeAndLength(t *testing.T) {
	long := strings.Repeat("a", 250) + "@b.co"
	for _, tc := range []struct{ name, email, want string }{
		{"no at sign", "nobody", "name@domain"},
		{"two at signs", "a@b@c", "name@domain"},
		{"empty local part", "@albums.local", "name@domain"},
		{"empty domain", "reader@", "name@domain"},
		{"space inside", "rea der@albums.local", "name@domain"},
		{"too long", long, "at most 254"},
	} {
		expectValidation(t, "register "+tc.name, RegisterInput{Email: tc.email, Password: "Correct-Horse-Battery-1"}.Validate(), tc.want)
		expectValidation(t, "login "+tc.name, LoginInput{Email: tc.email, Password: "Correct-Horse-Battery-1"}.Validate(), tc.want)
	}
	expectValidation(t, "register ok", RegisterInput{Email: " Reader@Albums.LOCAL ", Password: "Correct-Horse-Battery-1"}.Validate(), "")
}
