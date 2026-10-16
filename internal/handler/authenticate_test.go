package handler

import (
	"context"
	"errors"
	"testing"

	"homework/internal/auth"
	"homework/internal/model"
	"homework/internal/repository"
)

func TestAuthenticate(t *testing.T) {
	hash, err := auth.HashPassword("Correct-Horse-Battery-1")
	if err != nil {
		t.Fatal(err)
	}
	accounts := map[string]model.User{
		"alice@example.com": {ID: 7, Email: "alice@example.com", Role: model.UserRoleStaff, PasswordHash: hash},
		"seed@albums.local": {ID: 8, Email: "seed@albums.local", Role: model.UserRoleMember}, // no password yet
	}
	var asked []string
	users := &fakeUsers{t: t, getByEmail: func(email string) (model.User, error) {
		asked = append(asked, email)
		u, ok := accounts[email]
		if !ok {
			return model.User{}, repository.ErrNotFound
		}
		return u, nil
	}}

	cases := []struct {
		name, email, password string
		wantID                int64
		wantErr               error
	}{
		{"the right password logs in", "alice@example.com", "Correct-Horse-Battery-1", 7, nil},
		{"the e-mail is normalised first", "  Alice@Example.COM ", "Correct-Horse-Battery-1", 7, nil},
		{"a wrong password is invalid credentials", "alice@example.com", "wrong", 0, ErrInvalidCredentials},
		{"an unknown e-mail is invalid credentials", "nobody@example.com", "Correct-Horse-Battery-1", 0, ErrInvalidCredentials},
		{"an account without a password is invalid credentials", "seed@albums.local", "", 0, ErrInvalidCredentials},
		{"blank credentials are invalid credentials", "", "", 0, ErrInvalidCredentials},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user, err := Authenticate(context.Background(), users, tc.email, tc.password)
			if !errors.Is(err, tc.wantErr) || user.ID != tc.wantID {
				t.Fatalf("Authenticate(%q, %q) = %+v, %v; want id %d and error %v", tc.email, tc.password, user, err, tc.wantID, tc.wantErr)
			}
		})
	}
	if len(asked) < 2 || asked[1] != "alice@example.com" {
		t.Errorf("the repository must receive the normalised e-mail, got %q", asked)
	}

	t.Run("a storage failure is reported, not hidden behind invalid credentials", func(t *testing.T) {
		boom := errors.New("connection lost")
		broken := &fakeUsers{t: t, getByEmail: func(string) (model.User, error) { return model.User{}, boom }}
		if _, err := Authenticate(context.Background(), broken, "alice@example.com", "x"); !errors.Is(err, boom) {
			t.Fatalf("expected the storage error, got %v", err)
		}
	})
}
