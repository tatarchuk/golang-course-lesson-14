package repository

import (
	"context"

	"gorm.io/gorm"

	"homework/internal/model"
)

// CopyRepository is the storage contract for the physical copies of an album.
// Every method checks that the album exists first, so an unknown album is a 404 for
// the client rather than a foreign key error.
type CopyRepository interface {
	Create(ctx context.Context, albumID int64, in model.CopyInput) (model.Copy, error)
	ListByAlbum(ctx context.Context, albumID int64) ([]model.Copy, error)
	Availability(ctx context.Context, albumID int64) (model.Availability, error)
}

type gormCopyRepository struct {
	db *gorm.DB
}

var _ CopyRepository = (*gormCopyRepository)(nil)

func (r *gormCopyRepository) Create(ctx context.Context, albumID int64, in model.CopyInput) (model.Copy, error) {
	db := r.db.WithContext(ctx)
	if err := ensureAlbum(db, albumID); err != nil {
		return model.Copy{}, err
	}
	copyRow := model.Copy{AlbumID: albumID, InventoryNo: in.InventoryNo, Status: in.StatusOrDefault()}
	if err := db.Create(&copyRow).Error; err != nil {
		return model.Copy{}, translate(err) // duplicate inventory_no: ErrConflict; bad status: ErrInvalidInput
	}
	return copyRow, nil
}

func (r *gormCopyRepository) ListByAlbum(ctx context.Context, albumID int64) ([]model.Copy, error) {
	db := r.db.WithContext(ctx)
	if err := ensureAlbum(db, albumID); err != nil {
		return nil, err
	}
	copies := make([]model.Copy, 0)
	if err := db.Where("album_id = ?", albumID).Order("id").Find(&copies).Error; err != nil {
		return nil, translate(err)
	}
	return copies, nil
}

func (r *gormCopyRepository) Availability(ctx context.Context, albumID int64) (model.Availability, error) {
	db := r.db.WithContext(ctx)
	if err := ensureAlbum(db, albumID); err != nil {
		return model.Availability{}, err
	}
	var av model.Availability
	err := db.Model(&model.Copy{}).
		Select("count(*) AS total_copies, count(*) FILTER (WHERE status = ?) AS available_copies", model.CopyStatusAvailable).
		Where("album_id = ?", albumID).
		Scan(&av).Error
	if err != nil {
		return model.Availability{}, translate(err)
	}
	av.AlbumID = albumID // Scan fills only the selected columns
	return av, nil
}
