package repository

import (
	"errors"
	"fmt"

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

// translate maps the errors GORM produces (with TranslateError enabled the postgres
// driver turns SQLSTATE 23505, 23503 and 23514 into typed errors) to the package
// errors above. Errors that are already package errors, and unknown errors, pass
// through unchanged.
func translate(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return fmt.Errorf("%w: %v", ErrConflict, err)
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return fmt.Errorf("%w: %v", ErrInvalidReference, err)
	case errors.Is(err, gorm.ErrCheckConstraintViolated):
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return err
}
