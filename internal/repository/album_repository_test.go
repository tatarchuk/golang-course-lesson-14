package repository_test

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"homework/internal/model"
	"homework/internal/repository"
	"homework/internal/testdb"
)

func TestAlbumCreateStoresRelationsAndReturnsThem(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)

	notes := "first pressing"
	in := albumInput("Kind of Blue", genreID, []int64{ids[1], ids[0]})
	in.Notes = &notes
	created, err := repos.Albums.Create(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID != 1 || created.Title != "Kind of Blue" || created.GenreID != genreID {
		t.Fatalf("unexpected album %+v", created)
	}
	if created.Genre.Name != "jazz" {
		t.Errorf("genre not loaded: %+v", created.Genre)
	}
	if !equalIDs(artistIDs(created), []int64{ids[0], ids[1]}) {
		t.Errorf("artists should be loaded and sorted by id, got %v", artistIDs(created))
	}
	if created.ReleaseYear == nil || *created.ReleaseYear != 1959 || created.Notes == nil || *created.Notes != notes {
		t.Errorf("optional fields lost: %+v", created)
	}
	if created.CreatedAt.IsZero() || created.CreatedAt.Location() != time.UTC {
		t.Errorf("created_at should be a UTC timestamp, got %v", created.CreatedAt)
	}

	got, err := repos.Albums.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Genre.Name != "jazz" || !equalIDs(artistIDs(got), []int64{ids[0], ids[1]}) {
		t.Errorf("get should load relations, got %+v", got)
	}
}

func TestAlbumCreateRejectsUnknownGenre(t *testing.T) {
	repos := newRepos(t)
	_, ids := seedGenreAndArtists(t, repos)

	_, err := repos.Albums.Create(context.Background(), albumInput("X", 999999, ids))
	if !errors.Is(err, repository.ErrInvalidReference) {
		t.Fatalf("expected ErrInvalidReference, got %v", err)
	}
	if n := countRows(t, "albums"); n != 0 {
		t.Errorf("rejected create must not leave an album row, found %d", n)
	}
}

func TestAlbumCreateRejectsUnknownArtistWithoutCreatingIt(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)

	_, err := repos.Albums.Create(ctx, albumInput("X", genreID, []int64{ids[0], 999999}))
	if !errors.Is(err, repository.ErrInvalidReference) {
		t.Fatalf("expected ErrInvalidReference, got %v", err)
	}
	if !strings.Contains(err.Error(), "999999") {
		t.Errorf("error should name the missing id, got %q", err)
	}
	if _, err := repos.Artists.Get(ctx, 999999); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("unknown artist must not be created silently, got %v", err)
	}
	if n := countRows(t, "albums"); n != 0 {
		t.Errorf("rejected create must not leave an album row, found %d", n)
	}
}

func TestAlbumCreateRejectsDuplicateTitleAndLabel(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)

	if _, err := repos.Albums.Create(ctx, albumInput("Same", genreID, ids)); err != nil {
		t.Fatal(err)
	}
	_, err := repos.Albums.Create(ctx, albumInput("Same", genreID, ids))
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestAlbumGetReturnsNotFoundForUnknownID(t *testing.T) {
	repos := newRepos(t)
	if _, err := repos.Albums.Get(context.Background(), 42); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAlbumListOrdersFiltersAndPaginates(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	jazzID, ids := seedGenreAndArtists(t, repos)
	rock, err := repos.Genres.Create(ctx, model.GenreInput{Name: "rock"})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []struct {
		title string
		genre int64
	}{{"A", jazzID}, {"B", rock.ID}, {"C", jazzID}} {
		if _, err := repos.Albums.Create(ctx, albumInput(a.title, a.genre, ids[:1])); err != nil {
			t.Fatal(err)
		}
	}

	all, err := repos.Albums.List(ctx, repository.ListOptions{Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !equalIDs(albumIDs(all), []int64{1, 2, 3}) {
		t.Errorf("expected ids 1,2,3 in order, got %v", albumIDs(all))
	}
	for _, a := range all {
		if a.Genre.ID == 0 || len(a.Artists) != 1 {
			t.Errorf("list must load relations, got %+v", a)
		}
	}

	jazz, _ := repos.Albums.List(ctx, repository.ListOptions{GenreID: jazzID, Page: 1, Limit: 10})
	if !equalIDs(albumIDs(jazz), []int64{1, 3}) {
		t.Errorf("genre filter: expected 1,3, got %v", albumIDs(jazz))
	}
	page2, _ := repos.Albums.List(ctx, repository.ListOptions{Page: 2, Limit: 2})
	if !equalIDs(albumIDs(page2), []int64{3}) {
		t.Errorf("page 2 limit 2: expected 3, got %v", albumIDs(page2))
	}
	for _, page := range []int{4, math.MaxInt} {
		empty, err := repos.Albums.List(ctx, repository.ListOptions{Page: page, Limit: 2})
		if err != nil || empty == nil || len(empty) != 0 {
			t.Errorf("page %d: expected empty non-nil slice, got %v %v", page, empty, err)
		}
	}
}

func TestAlbumUpdateReplacesArtistsAndNullsOmittedOptionals(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)
	created, err := repos.Albums.Create(ctx, albumInput("Old", genreID, ids))
	if err != nil {
		t.Fatal(err)
	}

	updated, err := repos.Albums.Update(ctx, created.ID, model.AlbumInput{
		Title: "New", Label: "Columbia", GenreID: genreID, ArtistIDs: []int64{ids[1]},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "New" || updated.ReleaseYear != nil || updated.Notes != nil {
		t.Errorf("full replacement expected, got %+v", updated)
	}
	if !equalIDs(artistIDs(updated), []int64{ids[1]}) {
		t.Errorf("artists should be replaced, got %v", artistIDs(updated))
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) || updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Errorf("created_at must be kept and updated_at advanced: %v -> %v", created, updated)
	}
	if n := countRows(t, "album_artists"); n != 1 {
		t.Errorf("expected exactly one link row, got %d", n)
	}
}

func TestAlbumUpdateErrors(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)
	created, err := repos.Albums.Create(ctx, albumInput("Old", genreID, ids))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repos.Albums.Update(ctx, 999, albumInput("X", genreID, ids)); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("unknown id: expected ErrNotFound, got %v", err)
	}
	_, err = repos.Albums.Update(ctx, created.ID, albumInput("X", genreID, []int64{999999}))
	if !errors.Is(err, repository.ErrInvalidReference) {
		t.Errorf("unknown artist: expected ErrInvalidReference, got %v", err)
	}
	got, _ := repos.Albums.Get(ctx, created.ID)
	if got.Title != "Old" || !equalIDs(artistIDs(got), ids) {
		t.Errorf("rejected update must change nothing, got %+v", got)
	}
}

func TestAlbumDeleteCascadesCopiesAndLinks(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)
	created, err := repos.Albums.Create(ctx, albumInput("Gone", genreID, ids))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repos.Copies.Create(ctx, created.ID, model.CopyInput{InventoryNo: "INV-1"}); err != nil {
		t.Fatal(err)
	}

	if err := repos.Albums.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repos.Albums.Get(ctx, created.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
	if n := countRows(t, "copies"); n != 0 {
		t.Errorf("copies should cascade, found %d", n)
	}
	if n := countRows(t, "album_artists"); n != 0 {
		t.Errorf("links should cascade, found %d", n)
	}
	if err := repos.Albums.Delete(ctx, created.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("second delete: expected ErrNotFound, got %v", err)
	}
}

// selectCounter counts SELECT statements executed through a GORM session.
type selectCounter struct {
	logger.Interface
	selects int
}

func (c *selectCounter) LogMode(logger.LogLevel) logger.Interface      { return c }
func (c *selectCounter) Info(context.Context, string, ...interface{})  {}
func (c *selectCounter) Warn(context.Context, string, ...interface{})  {}
func (c *selectCounter) Error(context.Context, string, ...interface{}) {}
func (c *selectCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		c.selects++
	}
}

// TestListQueryCountIsConstant is the N+1 guard: the number of SELECTs issued by
// List must not grow with the number of albums.
func TestListQueryCountIsConstant(t *testing.T) {
	testdb.Reset(t)
	counter := &selectCounter{}
	repos := repository.NewGorm(testdb.DB(t).Session(&gorm.Session{Logger: counter}))
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)

	createAlbums := func(from, to int) {
		for i := from; i <= to; i++ {
			if _, err := repos.Albums.Create(ctx, albumInput("Album "+string(rune('A'+i)), genreID, ids)); err != nil {
				t.Fatal(err)
			}
		}
	}
	measure := func() int {
		counter.selects = 0
		if _, err := repos.Albums.List(ctx, repository.ListOptions{Page: 1, Limit: 100}); err != nil {
			t.Fatal(err)
		}
		return counter.selects
	}

	createAlbums(0, 1)
	small := measure()
	createAlbums(2, 9)
	large := measure()
	if small != large || large > 6 {
		t.Fatalf("List issues N+1 queries: %d SELECTs for 2 albums, %d for 10 (expected a constant <= 6; load relations with Preload)", small, large)
	}
}

func albumIDs(albums []model.Album) []int64 {
	ids := make([]int64, 0, len(albums))
	for _, a := range albums {
		ids = append(ids, a.ID)
	}
	return ids
}

func countRows(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	if err := testdb.DB(t).Table(table).Count(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
