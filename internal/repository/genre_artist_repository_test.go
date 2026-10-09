package repository_test

import (
	"context"
	"errors"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
)

func TestGenreCreateGetListAndConflict(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()

	jazz, err := repos.Genres.Create(ctx, model.GenreInput{Name: "jazz"})
	if err != nil || jazz.ID != 1 || jazz.Name != "jazz" || jazz.CreatedAt.IsZero() {
		t.Fatalf("create: %+v %v", jazz, err)
	}
	if _, err := repos.Genres.Create(ctx, model.GenreInput{Name: "rock"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.Genres.Create(ctx, model.GenreInput{Name: "jazz"}); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("duplicate name: expected ErrConflict, got %v", err)
	}
	got, err := repos.Genres.Get(ctx, jazz.ID)
	if err != nil || got.Name != "jazz" {
		t.Errorf("get: %+v %v", got, err)
	}
	if _, err := repos.Genres.Get(ctx, 999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("unknown id: expected ErrNotFound, got %v", err)
	}
	list, err := repos.Genres.List(ctx)
	if err != nil || len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 {
		t.Errorf("list should be ordered by id: %+v %v", list, err)
	}
}

func TestArtistCreateGetList(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()

	bio := "Trumpeter"
	miles, err := repos.Artists.Create(ctx, model.ArtistInput{Name: "Miles Davis", Bio: &bio})
	if err != nil || miles.ID != 1 || miles.Bio == nil || *miles.Bio != bio {
		t.Fatalf("create: %+v %v", miles, err)
	}
	daft, err := repos.Artists.Create(ctx, model.ArtistInput{Name: "Daft Punk"})
	if err != nil || daft.Bio != nil {
		t.Fatalf("create without bio: %+v %v", daft, err)
	}
	got, err := repos.Artists.Get(ctx, miles.ID)
	if err != nil || got.Name != "Miles Davis" {
		t.Errorf("get: %+v %v", got, err)
	}
	if _, err := repos.Artists.Get(ctx, 999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("unknown id: expected ErrNotFound, got %v", err)
	}
	list, err := repos.Artists.List(ctx)
	if err != nil || len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 {
		t.Errorf("list should be ordered by id: %+v %v", list, err)
	}
	empty, err := newRepos(t).Artists.List(ctx)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("empty list should be a non-nil empty slice: %v %v", empty, err)
	}
}
