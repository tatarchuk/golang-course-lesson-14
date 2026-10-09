package repository

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// driverError mimics what the gorm postgres driver returns with TranslateError on:
// the gorm sentinel wrapping the original pgconn error.
func driverError(sentinel error, code, constraint string) error {
	return fmt.Errorf("%w: %w", sentinel, &pgconn.PgError{
		Severity:       "ERROR",
		Code:           code,
		Message:        "insert or update on table violates constraint " + constraint,
		ConstraintName: constraint,
	})
}

func TestTranslateMapsAVanishedAlbumToNotFound(t *testing.T) {
	for _, constraint := range []string{"fk_copies_album", "fk_album_artists_album"} {
		err := translate(driverError(gorm.ErrForeignKeyViolated, "23503", constraint))
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: expected ErrNotFound, got %v", constraint, err)
		}
	}
}

func TestTranslateHidesDriverDetails(t *testing.T) {
	cases := map[string]struct {
		in   error
		want error
	}{
		"foreign key": {driverError(gorm.ErrForeignKeyViolated, "23503", "fk_albums_genre"), ErrInvalidReference},
		"unique":      {driverError(gorm.ErrDuplicatedKey, "23505", "uq_genres_name"), ErrConflict},
		"check":       {driverError(gorm.ErrCheckConstraintViolated, "23514", "chk_copies_status"), ErrInvalidInput},
	}
	for name, c := range cases {
		err := translate(c.in)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: expected %v, got %v", name, c.want, err)
			continue
		}
		if msg := err.Error(); strings.Contains(msg, "SQLSTATE") || strings.Contains(msg, "constraint") {
			t.Errorf("%s: driver details leak into the message: %q", name, msg)
		}
	}
}

func TestTranslateKeepsPackageAndUnknownErrorsUnchanged(t *testing.T) {
	explicit := fmt.Errorf("%w: artists [999999]", ErrInvalidReference)
	if err := translate(explicit); err != explicit {
		t.Errorf("package errors must pass through unchanged, got %v", err)
	}
	other := errors.New("connection reset")
	if err := translate(other); err != other {
		t.Errorf("unknown errors must pass through unchanged, got %v", err)
	}
	if err := translate(nil); err != nil {
		t.Errorf("nil must stay nil, got %v", err)
	}
}
