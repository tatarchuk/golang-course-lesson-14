package repository

import (
	"context"

	"gorm.io/gorm"

	"homework/internal/model"
)

// GenreRepository is the storage contract for genres.
type GenreRepository interface {
	Create(ctx context.Context, in model.GenreInput) (model.Genre, error)
	Get(ctx context.Context, id int64) (model.Genre, error)
	List(ctx context.Context) ([]model.Genre, error)
}

type gormGenreRepository struct {
	db *gorm.DB
}

var _ GenreRepository = (*gormGenreRepository)(nil)

func (r *gormGenreRepository) Create(ctx context.Context, in model.GenreInput) (model.Genre, error) {
	genre := model.Genre{Name: in.Name}
	if err := r.db.WithContext(ctx).Create(&genre).Error; err != nil {
		return model.Genre{}, translate(err) // duplicate name: ErrConflict
	}
	return genre, nil
}

func (r *gormGenreRepository) Get(ctx context.Context, id int64) (model.Genre, error) {
	var genre model.Genre
	if err := r.db.WithContext(ctx).First(&genre, id).Error; err != nil {
		return model.Genre{}, translate(err)
	}
	return genre, nil
}

func (r *gormGenreRepository) List(ctx context.Context) ([]model.Genre, error) {
	genres := make([]model.Genre, 0)
	if err := r.db.WithContext(ctx).Order("id").Find(&genres).Error; err != nil {
		return nil, translate(err)
	}
	return genres, nil
}
