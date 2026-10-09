package model

import "time"

// Genre is the one-to-many side of albums: every album belongs to exactly one genre.
type Genre struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (Genre) TableName() string { return "genres" }

// GenreInput is the body of POST /genres.
type GenreInput struct {
	Name string `json:"name" validate:"required" example:"jazz"`
}

// Validate checks that the name is present and not blank.
func (in GenreInput) Validate() error {
	return validationError(requiredProblem(field{"name", in.Name}))
}
