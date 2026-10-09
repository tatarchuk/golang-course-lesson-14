package model

import "time"

// Artist is the many-to-many side of albums: an album can have several artists and
// an artist several albums.
type Artist struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	Bio       *string   `json:"bio"` // null when not provided
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (Artist) TableName() string { return "artists" }

// ArtistInput is the body of POST /artists.
type ArtistInput struct {
	Name string  `json:"name" validate:"required" example:"Miles Davis"`
	Bio  *string `json:"bio" example:"Trumpeter and bandleader."`
}

// Validate checks that the name is present and not blank.
func (in ArtistInput) Validate() error {
	return validationError(requiredProblem(field{"name", in.Name}))
}
