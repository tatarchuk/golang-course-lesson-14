package repository

import (
	"sort"
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

func (r *MemoryRepository) List(opts ListOptions) ([]model.Album, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.Album, 0, len(r.albums)) // non-nil, so an empty result encodes as []
	for _, a := range r.albums {
		if opts.Genre == "" || a.Genre == opts.Genre {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID }) // map order is random
	return paginate(out, opts.Page, opts.Limit), nil
}

// paginate returns one page of items. page >= 1 and limit >= 1 are guaranteed by the caller.
func paginate(items []model.Album, page, limit int) []model.Album {
	// (page-1)*limit can overflow for page = MaxInt64. This guard compares against a small
	// number instead; if it is true the page is past the end and the offset is never computed.
	if page-1 > len(items)/limit {
		return []model.Album{}
	}
	start := (page - 1) * limit
	if start >= len(items) {
		return []model.Album{}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func (r *MemoryRepository) Update(id int64, in model.AlbumInput) (model.Album, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.albums[id]
	if !ok {
		return model.Album{}, ErrNotFound
	}
	// PUT is a full replacement: every field comes from the input,
	// so an omitted optional field becomes nil -> null. ID and CreatedAt are kept.
	a.Title, a.Artist, a.Label, a.Genre = in.Title, in.Artist, in.Label, in.Genre
	a.ReleaseYear, a.Notes = in.ReleaseYear, in.Notes
	a.UpdatedAt = time.Now().UTC()
	r.albums[id] = a // a is a copy; write it back
	return a, nil
}

func (r *MemoryRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.albums[id]; !ok {
		return ErrNotFound
	}
	delete(r.albums, id)
	return nil
}
