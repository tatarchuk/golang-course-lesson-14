package repository

import (
	"context"

	"gorm.io/gorm"

	"homework/internal/model"
)

// ArtistRepository is the storage contract for artists.
type ArtistRepository interface {
	Create(ctx context.Context, in model.ArtistInput) (model.Artist, error)
	Get(ctx context.Context, id int64) (model.Artist, error)
	List(ctx context.Context) ([]model.Artist, error)
}

type gormArtistRepository struct {
	db *gorm.DB
}

var _ ArtistRepository = (*gormArtistRepository)(nil)

func (r *gormArtistRepository) Create(ctx context.Context, in model.ArtistInput) (model.Artist, error) {
	artist := model.Artist{Name: in.Name, Bio: in.Bio}
	if err := r.db.WithContext(ctx).Create(&artist).Error; err != nil {
		return model.Artist{}, translate(err)
	}
	return artist, nil
}

func (r *gormArtistRepository) Get(ctx context.Context, id int64) (model.Artist, error) {
	var artist model.Artist
	if err := r.db.WithContext(ctx).First(&artist, id).Error; err != nil {
		return model.Artist{}, translate(err)
	}
	return artist, nil
}

func (r *gormArtistRepository) List(ctx context.Context) ([]model.Artist, error) {
	artists := make([]model.Artist, 0)
	if err := r.db.WithContext(ctx).Order("id").Find(&artists).Error; err != nil {
		return nil, translate(err)
	}
	return artists, nil
}
