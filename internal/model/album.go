package model

import "time"

// Album is the stored resource; its JSON tags define the response shape.
// The genre is a belongs-to relation and the artists a many-to-many relation through
// album_artists; the repository loads both explicitly. Optional fields are pointers
// so that an unset value is encoded as null, never omitted (no omitempty: the
// contract requires the key to be present).
type Album struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	Title       string    `json:"title"`
	Label       string    `json:"label"`
	GenreID     int64     `json:"genre_id"`
	Genre       Genre     `json:"genre" gorm:"foreignKey:GenreID"`
	Artists     []Artist  `json:"artists" gorm:"many2many:album_artists;"`
	ReleaseYear *int      `json:"release_year"` // null when not provided
	Notes       *string   `json:"notes"`        // null when not provided
	CreatedAt   time.Time `json:"created_at"`   // RFC 3339, set once on creation
	UpdatedAt   time.Time `json:"updated_at"`   // RFC 3339, changes on every PUT
}

// TableName pins the table name instead of relying on GORM's pluralisation.
func (Album) TableName() string { return "albums" }

// AlbumInput is the body of POST and PUT /albums. Relations are given by id, so the
// client can only reference genres and artists that already exist. No id and no
// timestamps: clients cannot set them. The validate and example tags are read by
// swag when generating the OpenAPI spec; the actual validation is done by Validate.
type AlbumInput struct {
	Title       string  `json:"title" validate:"required" example:"Kind of Blue"`
	Label       string  `json:"label" validate:"required" example:"Columbia"`
	GenreID     int64   `json:"genre_id" validate:"required" example:"1"`
	ArtistIDs   []int64 `json:"artist_ids" validate:"required"`
	ReleaseYear *int    `json:"release_year" example:"1959"`
	Notes       *string `json:"notes" example:"Recorded in 1959"`
}

// Validate checks the required fields and the shape of the references. Whether the
// referenced genre and artists exist is the repository's job.
func (in AlbumInput) Validate() error {
	problems := requiredProblem(field{"title", in.Title}, field{"label", in.Label})
	if in.GenreID < 1 {
		problems = append(problems, "genre_id must be a positive integer")
	}
	switch {
	case len(in.ArtistIDs) == 0:
		problems = append(problems, "artist_ids must contain at least one artist id")
	default:
		seen := make(map[int64]bool, len(in.ArtistIDs))
		for _, id := range in.ArtistIDs {
			if id < 1 {
				problems = append(problems, "artist_ids must contain positive integers")
				break
			}
			if seen[id] {
				problems = append(problems, "artist_ids must not repeat an id")
				break
			}
			seen[id] = true
		}
	}
	return validationError(problems)
}
