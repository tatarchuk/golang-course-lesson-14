package repository

import (
	"context"
	"fmt"
	"math"
	"sort"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"homework/internal/model"
)

// modelAlbum is a local alias so helper files can refer to the album model briefly.
type modelAlbum = model.Album

// ListOptions carries the filter and the page requested by the client.
// The handler validates them first: Page >= 1, 1 <= Limit <= 100.
type ListOptions struct {
	GenreID int64 // 0 means "no filter"
	Page    int
	Limit   int
}

// AlbumRepository is the storage contract for albums. Handlers depend only on this
// interface, so the implementation can change without touching the HTTP layer.
type AlbumRepository interface {
	Create(ctx context.Context, in model.AlbumInput) (model.Album, error)
	Get(ctx context.Context, id int64) (model.Album, error)
	List(ctx context.Context, opts ListOptions) ([]model.Album, error)
	Update(ctx context.Context, id int64, in model.AlbumInput) (model.Album, error)
	Delete(ctx context.Context, id int64) error
}

type gormAlbumRepository struct {
	db *gorm.DB
}

// Compile-time proof that the implementation satisfies the interface.
var _ AlbumRepository = (*gormAlbumRepository)(nil)

// Create inserts the album and its artist links in one transaction. The referenced
// genre and artists are checked first, and the album row is written with the
// associations omitted: left to itself GORM would upsert the artists and quietly
// create an empty artist for an unknown id.
func (r *gormAlbumRepository) Create(ctx context.Context, in model.AlbumInput) (model.Album, error) {
	var created model.Album
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := checkReferences(tx, in); err != nil {
			return err
		}
		album := albumFromInput(in)
		if err := tx.Omit(clause.Associations).Create(&album).Error; err != nil {
			return err
		}
		if err := linkArtists(tx, album.ID, in.ArtistIDs); err != nil {
			return err
		}
		var err error
		created, err = loadAlbum(tx, album.ID)
		return err
	})
	return created, translate(err)
}

// Get loads one album with its genre and artists.
func (r *gormAlbumRepository) Get(ctx context.Context, id int64) (model.Album, error) {
	album, err := loadAlbum(r.db.WithContext(ctx), id)
	return album, translate(err)
}

// List returns one page of albums ordered by id, optionally filtered by genre.
func (r *gormAlbumRepository) List(ctx context.Context, opts ListOptions) ([]model.Album, error) {
	albums := make([]model.Album, 0) // non-nil, so an empty result encodes as []
	page, limit := sanitizePage(opts.Page, opts.Limit)
	// (page-1)*limit would overflow for page = MaxInt; such a page is past the end anyway.
	if page-1 > math.MaxInt32/limit {
		return albums, nil
	}

	q := r.db.WithContext(ctx).Order("id")
	if opts.GenreID > 0 {
		q = q.Where("genre_id = ?", opts.GenreID)
	}
	if err := q.Offset((page - 1) * limit).Limit(limit).Find(&albums).Error; err != nil {
		return nil, translate(err)
	}

	// TODO(theme8 part 3): load the relations with Preload instead of this loop.
	for i := range albums {
		db := r.db.WithContext(ctx)
		if err := db.First(&albums[i].Genre, albums[i].GenreID).Error; err != nil {
			return nil, translate(err)
		}
		if err := db.Model(&albums[i]).Association("Artists").Find(&albums[i].Artists); err != nil {
			return nil, translate(err)
		}
		normalizeAlbum(&albums[i])
	}
	return albums, nil
}

// Update replaces every field of the album (PUT semantics) and its artist links in
// one transaction. Order of checks: existence, then references.
func (r *gormAlbumRepository) Update(ctx context.Context, id int64, in model.AlbumInput) (model.Album, error) {
	var updated model.Album
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.Album
		if err := tx.First(&existing, id).Error; err != nil {
			return err
		}
		if err := checkReferences(tx, in); err != nil {
			return err
		}
		// Select lists the columns explicitly so that nil pointers are written as NULL:
		// optional fields that are absent from the body are cleared, not kept.
		changes := albumFromInput(in)
		err := tx.Model(&existing).
			Select("title", "label", "genre_id", "release_year", "notes").
			Updates(changes).Error
		if err != nil {
			return err
		}
		if err := tx.Where("album_id = ?", id).Delete(&model.AlbumArtist{}).Error; err != nil {
			return err
		}
		if err := linkArtists(tx, id, in.ArtistIDs); err != nil {
			return err
		}
		updated, err = loadAlbum(tx, id)
		return err
	})
	return updated, translate(err)
}

// Delete removes the album; the database cascades to its copies and artist links.
func (r *gormAlbumRepository) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.Album{}, id)
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func albumFromInput(in model.AlbumInput) model.Album {
	return model.Album{
		Title:       in.Title,
		Label:       in.Label,
		GenreID:     in.GenreID,
		ReleaseYear: in.ReleaseYear,
		Notes:       in.Notes,
	}
}

// checkReferences fails with ErrInvalidReference when the genre or any artist is
// missing, naming the missing ids.
func checkReferences(tx *gorm.DB, in model.AlbumInput) error {
	var genres int64
	if err := tx.Model(&model.Genre{}).Where("id = ?", in.GenreID).Count(&genres).Error; err != nil {
		return err
	}
	if genres == 0 {
		return fmt.Errorf("%w: genre %d", ErrInvalidReference, in.GenreID)
	}

	var found []int64
	if err := tx.Model(&model.Artist{}).Where("id IN ?", in.ArtistIDs).Pluck("id", &found).Error; err != nil {
		return err
	}
	if missing := missingIDs(in.ArtistIDs, found); len(missing) > 0 {
		return fmt.Errorf("%w: artists %v", ErrInvalidReference, missing)
	}
	return nil
}

func missingIDs(wanted, found []int64) []int64 {
	present := make(map[int64]bool, len(found))
	for _, id := range found {
		present[id] = true
	}
	var missing []int64
	for _, id := range wanted {
		if !present[id] {
			missing = append(missing, id)
		}
	}
	return missing
}

// linkArtists writes the album_artists rows for the album.
func linkArtists(tx *gorm.DB, albumID int64, artistIDs []int64) error {
	links := make([]model.AlbumArtist, 0, len(artistIDs))
	for _, artistID := range artistIDs {
		links = append(links, model.AlbumArtist{AlbumID: albumID, ArtistID: artistID})
	}
	return tx.Create(&links).Error
}

// loadAlbum reads one album with its genre and artists.
func loadAlbum(db *gorm.DB, id int64) (model.Album, error) {
	var album model.Album
	if err := db.Preload("Artists").Preload("Genre").First(&album, id).Error; err != nil {
		return model.Album{}, err
	}
	normalizeAlbum(&album)
	return album, nil
}

// normalizeAlbum makes the response deterministic: artists sorted by id and never nil.
func normalizeAlbum(album *model.Album) {
	if album.Artists == nil {
		album.Artists = []model.Artist{}
	}
	sort.Slice(album.Artists, func(i, j int) bool { return album.Artists[i].ID < album.Artists[j].ID })
}

func sanitizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	return page, limit
}
