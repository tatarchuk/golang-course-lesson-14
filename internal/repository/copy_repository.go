package repository

import (
	"context"

	"gorm.io/gorm"

	"homework/internal/model"
)

// CopyRepository is the storage contract for the physical copies of an album.
// Every single-album method checks that the album exists first, so an unknown album
// is a 404 for the client rather than a foreign key error. AvailabilityForAlbums
// serves a whole page of the catalogue at once and simply leaves unknown ids out.
type CopyRepository interface {
	Create(ctx context.Context, albumID int64, in model.CopyInput) (model.Copy, error)
	ListByAlbum(ctx context.Context, albumID int64) ([]model.Copy, error)
	Availability(ctx context.Context, albumID int64) (model.Availability, error)
	AvailabilityForAlbums(ctx context.Context, albumIDs []int64) (map[int64]model.Availability, error)
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

// AvailabilityForAlbums summarises the copies of many albums in one statement, for a
// page of the catalogue: the map holds one entry per album that has copies; albums
// without copies and unknown ids are absent. No ids means no query at all.
func (r *gormCopyRepository) AvailabilityForAlbums(ctx context.Context, albumIDs []int64) (map[int64]model.Availability, error) {
	result := make(map[int64]model.Availability, len(albumIDs))
	if len(albumIDs) == 0 {
		return result, nil
	}
	var rows []model.Availability
	err := r.db.WithContext(ctx).Model(&model.Copy{}).
		Select("album_id, count(*) AS total_copies, count(*) FILTER (WHERE status = ?) AS available_copies", model.CopyStatusAvailable).
		Where("album_id IN ?", albumIDs).
		Group("album_id").
		Scan(&rows).Error
	if err != nil {
		return nil, translate(err)
	}
	for _, av := range rows {
		result[av.AlbumID] = av
	}
	return result, nil
}
