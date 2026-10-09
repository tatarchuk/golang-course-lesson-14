package repository_test

import (
	"context"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
	"homework/internal/testdb"
)

// newRepos resets the database and returns repositories bound to it.
func newRepos(t *testing.T) repository.Repositories {
	t.Helper()
	testdb.Reset(t)
	return repository.NewGorm(testdb.DB(t))
}

// seedGenreAndArtists creates one genre and two artists and returns their ids.
func seedGenreAndArtists(t *testing.T, repos repository.Repositories) (int64, []int64) {
	t.Helper()
	ctx := context.Background()
	genre, err := repos.Genres.Create(ctx, model.GenreInput{Name: "jazz"})
	if err != nil {
		t.Fatalf("create genre: %v", err)
	}
	var ids []int64
	for _, name := range []string{"Miles Davis", "John Coltrane"} {
		a, err := repos.Artists.Create(ctx, model.ArtistInput{Name: name})
		if err != nil {
			t.Fatalf("create artist: %v", err)
		}
		ids = append(ids, a.ID)
	}
	return genre.ID, ids
}

func albumInput(title string, genreID int64, artistIDs []int64) model.AlbumInput {
	year := 1959
	return model.AlbumInput{Title: title, Label: "Columbia", GenreID: genreID, ArtistIDs: artistIDs, ReleaseYear: &year}
}

func artistIDs(a model.Album) []int64 {
	ids := make([]int64, 0, len(a.Artists))
	for _, ar := range a.Artists {
		ids = append(ids, ar.ID)
	}
	return ids
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
