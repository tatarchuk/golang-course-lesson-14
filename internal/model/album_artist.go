package model

// AlbumArtist is one row of the album_artists join table. The repository writes the
// link rows itself instead of letting GORM upsert the associated artists, so an
// unknown artist id is rejected rather than silently created.
type AlbumArtist struct {
	AlbumID  int64 `gorm:"primaryKey"`
	ArtistID int64 `gorm:"primaryKey"`
}

// TableName pins the join table name.
func (AlbumArtist) TableName() string { return "album_artists" }
