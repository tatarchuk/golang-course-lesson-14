package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Album is the stored resource; its JSON tags define the response shape.
type Album struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	Label       string    `json:"label"`
	Genre       string    `json:"genre"`
	ReleaseYear *int      `json:"release_year"` // nil -> null; no omitempty: key must always be present
	Notes       *string   `json:"notes"`
	CreatedAt   time.Time `json:"created_at"` // time.Time marshals as RFC 3339 with nanoseconds
	UpdatedAt   time.Time `json:"updated_at"`
}

// AlbumInput is the body of POST and PUT. No id, no timestamps: clients cannot set them.
type AlbumInput struct {
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Label       string  `json:"label"`
	Genre       string  `json:"genre"`
	ReleaseYear *int    `json:"release_year"`
	Notes       *string `json:"notes"`
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
