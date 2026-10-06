package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Album is the stored resource; its JSON tags define the response shape.
// Optional fields are pointers so that an unset value is encoded as null, never omitted
// (no omitempty: the contract requires the key to be present). Field comments become
// descriptions in the generated OpenAPI spec.
type Album struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	Label       string    `json:"label"`
	Genre       string    `json:"genre"`
	ReleaseYear *int      `json:"release_year"` // null when not provided
	Notes       *string   `json:"notes"`        // null when not provided
	CreatedAt   time.Time `json:"created_at"`   // RFC 3339, set once on creation
	UpdatedAt   time.Time `json:"updated_at"`   // RFC 3339, changes on every PUT
}

// AlbumInput is the body of POST and PUT. No id, no timestamps: clients cannot set them.
// The validate and example tags are read by swag when generating the OpenAPI spec;
// the actual validation is done by Validate.
type AlbumInput struct {
	Title       string  `json:"title" validate:"required" example:"Kind of Blue"`
	Artist      string  `json:"artist" validate:"required" example:"Miles Davis"`
	Label       string  `json:"label" validate:"required" example:"Columbia"`
	Genre       string  `json:"genre" validate:"required" example:"jazz"`
	ReleaseYear *int    `json:"release_year" example:"1959"`
	Notes       *string `json:"notes" example:"Recorded in 1959"`
}

// ErrValidation marks errors that become a 422 response.
var ErrValidation = errors.New("validation error")

// Validate checks the required fields. Missing, null, "" and "   " all end up as an empty
// string after TrimSpace. Wrong JSON types never get here: the decoder rejects them first.
func (in AlbumInput) Validate() error {
	fields := []struct{ name, value string }{
		{"title", in.Title},
		{"artist", in.Artist},
		{"label", in.Label},
		{"genre", in.Genre},
	}
	var missing []string
	for _, f := range fields {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: required fields missing or empty: %s", ErrValidation, strings.Join(missing, ", "))
	}
	return nil
}
