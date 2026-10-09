-- Theme 8: normalized schema for the Albums API.
-- Generated from the Go structs in internal/model following db_migration_prompt.md:
-- BIGSERIAL ids, NOT NULL by default (nullable only for pointer fields), timestamps on
-- every table, named foreign keys with ON DELETE RESTRICT unless a cascade is justified,
-- one index per foreign key column (idx_{table}_{column}), CHECK constraints for
-- enumerated values, idempotent CREATE ... IF NOT EXISTS.

CREATE TABLE IF NOT EXISTS genres (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT      NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    CONSTRAINT uq_genres_name UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS artists (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT      NOT NULL,
    bio        TEXT      NULL, -- optional: *string in Go
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS albums (
    id           BIGSERIAL PRIMARY KEY,
    title        TEXT      NOT NULL,
    label        TEXT      NOT NULL,
    genre_id     BIGINT    NOT NULL,
    release_year INTEGER   NULL, -- optional: *int in Go
    notes        TEXT      NULL, -- optional: *string in Go
    created_at   TIMESTAMP NOT NULL DEFAULT now(),
    updated_at   TIMESTAMP NOT NULL DEFAULT now(),
    CONSTRAINT uq_albums_title_label UNIQUE (title, label),
    CONSTRAINT fk_albums_genre FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_albums_genre_id ON albums (genre_id);

-- Join table of the many-to-many relation albums <-> artists.
-- Deleting an album cascades here: a link row is meaningless without its album.
-- Deleting an artist is restricted while albums still reference it.
CREATE TABLE IF NOT EXISTS album_artists (
    album_id   BIGINT    NOT NULL,
    artist_id  BIGINT    NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    CONSTRAINT pk_album_artists PRIMARY KEY (album_id, artist_id),
    CONSTRAINT fk_album_artists_album  FOREIGN KEY (album_id)  REFERENCES albums (id)  ON DELETE CASCADE,
    CONSTRAINT fk_album_artists_artist FOREIGN KEY (artist_id) REFERENCES artists (id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_album_artists_album_id  ON album_artists (album_id);
CREATE INDEX IF NOT EXISTS idx_album_artists_artist_id ON album_artists (artist_id);

-- Physical copies (vinyl, CD) of an album, tracked by inventory number.
-- Deleting an album cascades to its copies: a copy cannot exist without its album.
CREATE TABLE IF NOT EXISTS copies (
    id           BIGSERIAL PRIMARY KEY,
    album_id     BIGINT    NOT NULL,
    inventory_no TEXT      NOT NULL,
    status       TEXT      NOT NULL DEFAULT 'available',
    created_at   TIMESTAMP NOT NULL DEFAULT now(),
    updated_at   TIMESTAMP NOT NULL DEFAULT now(),
    CONSTRAINT uq_copies_inventory_no UNIQUE (inventory_no),
    CONSTRAINT chk_copies_status CHECK (status IN ('available', 'borrowed', 'lost')),
    CONSTRAINT fk_copies_album FOREIGN KEY (album_id) REFERENCES albums (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_copies_album_id ON copies (album_id);

CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT      NOT NULL,
    role       TEXT      NOT NULL DEFAULT 'member',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    CONSTRAINT uq_users_email UNIQUE (email),
    CONSTRAINT chk_users_role CHECK (role IN ('member', 'staff', 'admin'))
);
