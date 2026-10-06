package repository

import (
	"errors"
	"homework/internal/model"
)

// ErrNotFound is returned when no album has the requested id.
var ErrNotFound = errors.New("album not found")

// ListOptions carries the filter and the page requested by the client.
// The handler validates them first: Page >= 1, 1 <= Limit <= 100.
type ListOptions struct {
	Genre string // exact, case-sensitive match; "" means "no filter"
	Page  int
	Limit int
}

// AlbumRepository is the storage contract. Handlers depend only on this interface,
// so the in-memory store can be swapped for a database without touching them.
type AlbumRepository interface {
	Create(in model.AlbumInput) (model.Album, error)
	Get(id int64) (model.Album, error)
	List(opts ListOptions) ([]model.Album, error)
	Update(id int64, in model.AlbumInput) (model.Album, error)
	Delete(id int64) error
}
