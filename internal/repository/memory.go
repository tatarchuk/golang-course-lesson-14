package repository

import (
	"errors"
	"sync"
	"time"

	"homework/internal/model"
)

// MemoryRepository keeps albums in a map guarded by a read/write mutex.
type MemoryRepository struct {
	mu     sync.RWMutex
	albums map[int64]model.Album // values, not pointers: Get/List hand out copies
	nextID int64
}

// Compile-time proof that MemoryRepository satisfies the interface.
var _ AlbumRepository = (*MemoryRepository)(nil)

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{albums: make(map[int64]model.Album), nextID: 1}
}

func (r *MemoryRepository) Create(in model.AlbumInput) (model.Album, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	a := model.Album{
		ID:          r.nextID,
		Title:       in.Title,
		Artist:      in.Artist,
		Label:       in.Label,
		Genre:       in.Genre,
		ReleaseYear: in.ReleaseYear,
		Notes:       in.Notes,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.albums[a.ID] = a
	r.nextID++ // never decremented, so an id is never reused after a delete
	return a, nil
}

func (r *MemoryRepository) Get(id int64) (model.Album, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.albums[id]
	if !ok {
		return model.Album{}, ErrNotFound
	}
	return a, nil // a copy of the stored value
}

// Temporary stubs so the interface is satisfied; implemented in stages 2-4.
func (r *MemoryRepository) List(opts ListOptions) ([]model.Album, error) {
	return nil, errors.New("not implemented")
}

func (r *MemoryRepository) Update(id int64, in model.AlbumInput) (model.Album, error) {
	return model.Album{}, errors.New("not implemented")
}

func (r *MemoryRepository) Delete(id int64) error {
	return errors.New("not implemented")
}
