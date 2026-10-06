package repository

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"testing"

	"homework/internal/model"
)

func input(tag, genre string) model.AlbumInput {
	return model.AlbumInput{Title: "title-" + tag, Artist: "artist-" + tag, Label: "label-" + tag, Genre: genre}
}

func ids(albums []model.Album) []int64 {
	out := make([]int64, 0, len(albums))
	for _, a := range albums {
		out = append(out, a.ID)
	}
	return out
}

func TestCreateAssignsSequentialIDsAndTimestamps(t *testing.T) {
	repo := NewMemoryRepository()
	for want := int64(1); want <= 3; want++ {
		a, err := repo.Create(input(fmt.Sprint(want), "g"))
		if err != nil {
			t.Fatal(err)
		}
		if a.ID != want {
			t.Errorf("id: want %d, got %d", want, a.ID)
		}
		if a.CreatedAt.IsZero() || !a.UpdatedAt.Equal(a.CreatedAt) {
			t.Errorf("timestamps: created=%v updated=%v", a.CreatedAt, a.UpdatedAt)
		}
	}
}

func TestGetReturnsStoredAlbumOrNotFound(t *testing.T) {
	repo := NewMemoryRepository()
	created, _ := repo.Create(input("a", "jazz"))

	got, err := repo.Get(created.ID)
	if err != nil || got.ID != created.ID || got.Title != "title-a" {
		t.Errorf("Get(%d) = %+v, %v", created.ID, got, err)
	}
	if _, err := repo.Get(42); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing id: want ErrNotFound, got %v", err)
	}
}

// The store holds values, so what Get hands out is a copy that cannot alter stored data.
func TestGetReturnsACopy(t *testing.T) {
	repo := NewMemoryRepository()
	created, _ := repo.Create(input("a", "jazz"))

	got, _ := repo.Get(created.ID)
	got.Title = "changed locally"

	again, _ := repo.Get(created.ID)
	if again.Title != "title-a" {
		t.Errorf("modifying a returned album must not touch the store, got %q", again.Title)
	}
}

func TestUpdateReplacesEverythingExceptIDAndCreatedAt(t *testing.T) {
	repo := NewMemoryRepository()
	year, notes := 1959, "n"
	in := input("a", "jazz")
	in.ReleaseYear, in.Notes = &year, &notes
	created, _ := repo.Create(in)

	updated, err := repo.Update(created.ID, input("b", "rock")) // no optional fields
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("id and created_at must be kept: %+v", updated)
	}
	if updated.Title != "title-b" || updated.Genre != "rock" {
		t.Errorf("required fields must be replaced: %+v", updated)
	}
	if updated.ReleaseYear != nil || updated.Notes != nil {
		t.Errorf("omitted optional fields must become nil: %+v", updated)
	}
	if updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Error("updated_at must not go backwards")
	}
	if _, err := repo.Update(99, input("c", "g")); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing id: want ErrNotFound, got %v", err)
	}
}

func TestDeleteRemovesAndIDsAreNeverReused(t *testing.T) {
	repo := NewMemoryRepository()
	_, _ = repo.Create(input("a", "g"))
	second, _ := repo.Create(input("b", "g"))

	if err := repo.Delete(second.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(second.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: want ErrNotFound, got %v", err)
	}
	if _, err := repo.Get(second.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get after delete: want ErrNotFound, got %v", err)
	}
	third, _ := repo.Create(input("c", "g"))
	if third.ID != 3 {
		t.Errorf("a deleted id must not be reused: want 3, got %d", third.ID)
	}
}

func TestListSortsByIDAndFiltersByGenre(t *testing.T) {
	repo := NewMemoryRepository()
	for i := 1; i <= 25; i++ {
		genre := "a"
		if i%2 == 0 {
			genre = "b"
		}
		_, _ = repo.Create(input(fmt.Sprint(i), genre))
	}

	all, _ := repo.List(ListOptions{Page: 1, Limit: 100})
	for i, id := range ids(all) {
		if id != int64(i+1) {
			t.Fatalf("not sorted by id: %v", ids(all))
		}
	}
	onlyB, _ := repo.List(ListOptions{Genre: "b", Page: 1, Limit: 100})
	if got := ids(onlyB); len(got) != 12 || got[0] != 2 || got[11] != 24 {
		t.Errorf("filter b: got ids %v", got)
	}
	none, _ := repo.List(ListOptions{Genre: "zzz", Page: 1, Limit: 10})
	if none == nil || len(none) != 0 {
		t.Errorf("no match must be an empty, non-nil slice, got %#v", none)
	}
}

func TestPaginate(t *testing.T) {
	items := make([]model.Album, 5)
	for i := range items {
		items[i].ID = int64(i + 1)
	}
	cases := []struct {
		name        string
		page, limit int
		want        []int64
	}{
		{"first page", 1, 2, []int64{1, 2}},
		{"middle page", 2, 2, []int64{3, 4}},
		{"partial last page", 3, 2, []int64{5}},
		{"page past the end", 4, 2, []int64{}},
		{"limit larger than data", 1, 10, []int64{1, 2, 3, 4, 5}},
		{"second page of three", 2, 3, []int64{4, 5}},
		{"huge page does not overflow", math.MaxInt, 100, []int64{}},
		{"huge page with limit one", math.MaxInt, 1, []int64{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(paginate(items, tc.page, tc.limit))
			if !slices.Equal(got, tc.want) {
				t.Errorf("paginate(page=%d, limit=%d) = %v, want %v", tc.page, tc.limit, got, tc.want)
			}
		})
	}
	if got := paginate(nil, 1, 10); got == nil || len(got) != 0 {
		t.Errorf("empty input must give an empty, non-nil slice, got %#v", got)
	}
}

func TestConcurrentCreatesGetDistinctIDs(t *testing.T) {
	repo := NewMemoryRepository()
	const n = 100
	results := make(chan int64, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := repo.Create(input("x", "g"))
			if err != nil {
				t.Error(err)
				return
			}
			results <- a.ID
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[int64]bool, n)
	for id := range results {
		if seen[id] {
			t.Errorf("duplicate id %d", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Errorf("want %d distinct ids, got %d", n, len(seen))
	}
}
