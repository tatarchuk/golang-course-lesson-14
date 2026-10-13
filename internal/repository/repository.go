package repository

import "gorm.io/gorm"

// Repositories groups the storage interfaces the HTTP layer depends on. Handlers
// receive this struct, so tests can substitute fakes for any part of it.
type Repositories struct {
	Albums  AlbumRepository
	Artists ArtistRepository
	Genres  GenreRepository
	Copies  CopyRepository
	Users   UserRepository
}

// NewGorm builds the GORM-backed repositories on one shared connection.
func NewGorm(db *gorm.DB) Repositories {
	return Repositories{
		Albums:  &gormAlbumRepository{db: db},
		Artists: &gormArtistRepository{db: db},
		Genres:  &gormGenreRepository{db: db},
		Copies:  &gormCopyRepository{db: db},
		Users:   &gormUserRepository{db: db},
	}
}

// ensureAlbum returns ErrNotFound when no album has the given id.
func ensureAlbum(db *gorm.DB, albumID int64) error {
	var n int64
	if err := db.Model(&modelAlbum{}).Where("id = ?", albumID).Count(&n).Error; err != nil {
		return translate(err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
