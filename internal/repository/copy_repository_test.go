package repository_test

import (
	"context"
	"errors"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
)

func TestCopyCreateListAndAvailability(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)
	album, err := repos.Albums.Create(ctx, albumInput("Kind of Blue", genreID, ids))
	if err != nil {
		t.Fatal(err)
	}

	first, err := repos.Copies.Create(ctx, album.ID, model.CopyInput{InventoryNo: "INV-1"})
	if err != nil || first.ID != 1 || first.AlbumID != album.ID || first.Status != model.CopyStatusAvailable {
		t.Fatalf("create with default status: %+v %v", first, err)
	}
	if _, err := repos.Copies.Create(ctx, album.ID, model.CopyInput{InventoryNo: "INV-2", Status: model.CopyStatusAvailable}); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.Copies.Create(ctx, album.ID, model.CopyInput{InventoryNo: "INV-3", Status: model.CopyStatusLost}); err != nil {
		t.Fatal(err)
	}
	if _, err := repos.Copies.Create(ctx, album.ID, model.CopyInput{InventoryNo: "INV-1"}); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("duplicate inventory_no: expected ErrConflict, got %v", err)
	}
	if _, err := repos.Copies.Create(ctx, 999, model.CopyInput{InventoryNo: "INV-9"}); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("unknown album: expected ErrNotFound, got %v", err)
	}

	list, err := repos.Copies.ListByAlbum(ctx, album.ID)
	if err != nil || len(list) != 3 || list[0].InventoryNo != "INV-1" || list[2].Status != model.CopyStatusLost {
		t.Errorf("list: %+v %v", list, err)
	}
	if _, err := repos.Copies.ListByAlbum(ctx, 999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("list for unknown album: expected ErrNotFound, got %v", err)
	}

	av, err := repos.Copies.Availability(ctx, album.ID)
	if err != nil || av.AlbumID != album.ID || av.TotalCopies != 3 || av.AvailableCopies != 2 {
		t.Errorf("availability: %+v %v", av, err)
	}
	if _, err := repos.Copies.Availability(ctx, 999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("availability for unknown album: expected ErrNotFound, got %v", err)
	}
}

func TestCopyCreateRejectsInvalidStatusAtDatabaseLevel(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)
	album, err := repos.Albums.Create(ctx, albumInput("X", genreID, ids))
	if err != nil {
		t.Fatal(err)
	}
	_, err = repos.Copies.Create(ctx, album.ID, model.CopyInput{InventoryNo: "INV-1", Status: "broken"})
	if !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("CHECK violation should map to ErrInvalidInput, got %v", err)
	}
}
