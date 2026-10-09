package model

import (
	"slices"
	"strings"
	"time"
)

// Copy statuses; the same list is enforced by the CHECK constraint on copies.status.
const (
	CopyStatusAvailable = "available"
	CopyStatusBorrowed  = "borrowed"
	CopyStatusLost      = "lost"
)

// CopyStatuses lists every accepted status.
var CopyStatuses = []string{CopyStatusAvailable, CopyStatusBorrowed, CopyStatusLost}

// Copy is one physical copy (vinyl, CD) of an album, identified by its inventory number.
type Copy struct {
	ID          int64     `json:"id" gorm:"primaryKey"`
	AlbumID     int64     `json:"album_id"`
	InventoryNo string    `json:"inventory_no"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName pins the table name ("copies" is irregular enough not to guess).
func (Copy) TableName() string { return "copies" }

// CopyInput is the body of POST /albums/{id}/copies. An empty status means available.
type CopyInput struct {
	InventoryNo string `json:"inventory_no" validate:"required" example:"INV-0001"`
	Status      string `json:"status" example:"available"`
}

// Validate checks the inventory number and that the status, if given, is known.
func (in CopyInput) Validate() error {
	problems := requiredProblem(field{"inventory_no", in.InventoryNo})
	if in.Status != "" && !slices.Contains(CopyStatuses, in.Status) {
		problems = append(problems, "status must be one of: "+strings.Join(CopyStatuses, ", "))
	}
	return validationError(problems)
}

// StatusOrDefault returns the requested status, or available when none was given.
func (in CopyInput) StatusOrDefault() string {
	if in.Status == "" {
		return CopyStatusAvailable
	}
	return in.Status
}

// Availability summarises the copies of one album.
type Availability struct {
	AlbumID         int64 `json:"album_id"`
	TotalCopies     int64 `json:"total_copies"`
	AvailableCopies int64 `json:"available_copies"`
}
