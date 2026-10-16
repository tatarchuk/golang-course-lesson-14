package handler

import (
	"context"
	"errors"

	"homework/internal/auth"
	"homework/internal/model"
	"homework/internal/repository"
)

// ErrInvalidCredentials is what Authenticate returns for every failed login: an unknown
// e-mail, a wrong password or an account that has no password yet. The three cases are
// deliberately indistinguishable, so neither the API nor the login form can be used to
// tell registered e-mails from unknown ones.
var ErrInvalidCredentials = errors.New(msgInvalidCredentials)

// Authenticate checks a login for the API and the web UI alike. It normalises the
// e-mail, loads the account and runs exactly one bcrypt comparison whatever happens:
// against the stored hash for a known account with a password, against DummyHash
// otherwise (bcrypt would reject an empty hash instantly and the timing difference would
// reveal the account), so the response time does not reveal which e-mails are
// registered. Only a storage failure other than ErrNotFound is returned as itself.
func Authenticate(ctx context.Context, users repository.UserRepository, email, password string) (model.User, error) {
	user, err := users.GetByEmail(ctx, model.NormalizeEmail(email))
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return model.User{}, err
	}
	known := err == nil && user.PasswordHash != ""
	hash := auth.DummyHash
	if known {
		hash = user.PasswordHash
	}
	if !auth.CheckPassword(hash, password) || !known {
		return model.User{}, ErrInvalidCredentials
	}
	return user, nil
}
