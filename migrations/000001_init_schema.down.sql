-- Reverts 000001_init_schema.up.sql: objects are dropped in the exact reverse order of
-- their creation (constraints and indexes before their tables) to avoid dependency errors.

DROP TABLE IF EXISTS users;

DROP INDEX IF EXISTS idx_copies_album_id;
ALTER TABLE IF EXISTS copies DROP CONSTRAINT IF EXISTS fk_copies_album;
DROP TABLE IF EXISTS copies;

DROP INDEX IF EXISTS idx_album_artists_artist_id;
DROP INDEX IF EXISTS idx_album_artists_album_id;
ALTER TABLE IF EXISTS album_artists DROP CONSTRAINT IF EXISTS fk_album_artists_artist;
ALTER TABLE IF EXISTS album_artists DROP CONSTRAINT IF EXISTS fk_album_artists_album;
DROP TABLE IF EXISTS album_artists;

DROP INDEX IF EXISTS idx_albums_genre_id;
ALTER TABLE IF EXISTS albums DROP CONSTRAINT IF EXISTS fk_albums_genre;
DROP TABLE IF EXISTS albums;

DROP TABLE IF EXISTS artists;

DROP TABLE IF EXISTS genres;
