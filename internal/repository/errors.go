package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Errors returned by every repository. Handlers map them to HTTP statuses and never
// see driver errors directly.
var (
	// ErrNotFound means the requested row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means a UNIQUE constraint rejected the write (duplicate name, title
	// and label, inventory number).
	ErrConflict = errors.New("conflicts with an existing row")
	// ErrInvalidReference means a referenced genre, artist or album does not exist.
	ErrInvalidReference = errors.New("referenced row does not exist")
	// ErrInvalidInput means a CHECK constraint rejected a value (for example a copy
	// status outside the allowed list).
	ErrInvalidInput = errors.New("value rejected by the database")
)

// albumChildConstraints are the foreign keys that point from a child table to albums.
// A violation there means the album vanished between the existence check and the
// write (a concurrent DELETE), which is a 404 for the client, not a bad reference.
var albumChildConstraints = map[string]bool{
	"fk_copies_album":        true,
	"fk_album_artists_album": true,
}

// translate maps the errors GORM produces (with TranslateError enabled the postgres
// driver turns SQLSTATE 23505, 23503 and 23514 into typed errors) to the bare package
// errors above, so no driver text ever reaches a client. Errors that are already
// package errors, and unknown errors, pass through unchanged.
func translate(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrConflict
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		if albumChildConstraints[constraintName(err)] {
			return ErrNotFound
		}
		return ErrInvalidReference
	case errors.Is(err, gorm.ErrCheckConstraintViolated):
		return ErrInvalidInput
	}
	return err
}

// constraintName extracts the violated constraint from the wrapped driver error.
func constraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}
