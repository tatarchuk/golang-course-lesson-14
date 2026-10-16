package repository_test

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"homework/internal/model"
	"homework/internal/repository"
	"homework/internal/testdb"
)

// Lesson 10: the catalogue page searches by title or artist name and shows the
// availability of every row. Both need repository support that keeps the number of
// statements constant whatever the page holds.

// seedSearchCatalogue creates two genres, three artists and a set of albums with
// titles chosen for the search cases (including LIKE metacharacters) and returns the
// genre ids and the album ids keyed by title. Albums are created in this order, so
// the id order is the order of the slice below.
func seedSearchCatalogue(t *testing.T, repos repository.Repositories) (genres map[string]int64, albums map[string]int64) {
	t.Helper()
	ctx := context.Background()
	genres = map[string]int64{}
	for _, name := range []string{"jazz", "electronic"} {
		g, err := repos.Genres.Create(ctx, model.GenreInput{Name: name})
		if err != nil {
			t.Fatalf("create genre: %v", err)
		}
		genres[name] = g.ID
	}
	artists := map[string]int64{}
	for _, name := range []string{"Miles Davis", "Radiohead", "Daft Punk"} {
		a, err := repos.Artists.Create(ctx, model.ArtistInput{Name: name})
		if err != nil {
			t.Fatalf("create artist: %v", err)
		}
		artists[name] = a.ID
	}
	albums = map[string]int64{}
	for _, row := range []struct {
		title, genre string
		artists      []string
	}{
		{"Kind of Blue", "jazz", []string{"Miles Davis"}},
		{"Kid A", "jazz", []string{"Radiohead"}},
		{"OK Computer", "jazz", []string{"Radiohead"}},
		{"Discovery", "electronic", []string{"Daft Punk"}},
		{"Fifty%Fifty", "jazz", []string{"Miles Davis"}},
		{"Fifty Fifty", "jazz", []string{"Miles Davis"}},
		{"Low_Key", "jazz", []string{"Miles Davis"}},
		{"Low Key", "jazz", []string{"Miles Davis"}},
		{`Back\slash`, "jazz", []string{"Miles Davis"}},
	} {
		var ids []int64
		for _, name := range row.artists {
			ids = append(ids, artists[name])
		}
		a, err := repos.Albums.Create(ctx, albumInput(row.title, genres[row.genre], ids))
		if err != nil {
			t.Fatalf("create album %q: %v", row.title, err)
		}
		albums[row.title] = a.ID
	}
	return genres, albums
}

func titles(albums []model.Album) []string {
	out := make([]string, 0, len(albums))
	for _, a := range albums {
		out = append(out, a.Title)
	}
	return out
}

func equalStrings(a, b []string) bool {
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

func TestAlbumListSearchesTitleAndArtistNames(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	genres, _ := seedSearchCatalogue(t, repos)
	all := []string{"Kind of Blue", "Kid A", "OK Computer", "Discovery", "Fifty%Fifty", "Fifty Fifty", "Low_Key", "Low Key", `Back\slash`}

	cases := []struct {
		name string
		opts repository.ListOptions
		want []string
	}{
		{"a title fragment, case-insensitive", repository.ListOptions{Query: "blue"}, []string{"Kind of Blue"}},
		{"upper case matches lower case", repository.ListOptions{Query: "KID"}, []string{"Kid A"}},
		{"an artist name finds every album of the artist, in id order", repository.ListOptions{Query: "radiohead"}, []string{"Kid A", "OK Computer"}},
		{"surrounding spaces are ignored", repository.ListOptions{Query: "  kid  "}, []string{"Kid A"}},
		{"no match is an empty page", repository.ListOptions{Query: "zzz"}, []string{}},
		{"a blank query means no filter", repository.ListOptions{Query: "   "}, all},
		{"the query combines with the genre filter", repository.ListOptions{Query: "disc", GenreID: genres["jazz"]}, []string{}},
		{"the query combines with the matching genre", repository.ListOptions{Query: "disc", GenreID: genres["electronic"]}, []string{"Discovery"}},
		{"the query paginates", repository.ListOptions{Query: "radiohead", Page: 2, Limit: 1}, []string{"OK Computer"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.opts.Limit == 0 {
				tc.opts.Page, tc.opts.Limit = 1, 100
			}
			got, err := repos.Albums.List(ctx, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !equalStrings(titles(got), tc.want) {
				t.Errorf("List(%+v) = %v, want %v", tc.opts, titles(got), tc.want)
			}
			for _, a := range got {
				if a.Genre.ID == 0 || len(a.Artists) == 0 {
					t.Errorf("a searched album must still carry its genre and artists: %+v", a)
				}
			}
		})
	}
}

func TestAlbumListSearchTreatsLikeMetacharactersLiterally(t *testing.T) {
	repos := newRepos(t)
	ctx := context.Background()
	seedSearchCatalogue(t, repos)

	cases := []struct {
		name, query string
		want        []string
	}{
		{"percent is a literal percent sign", "%", []string{"Fifty%Fifty"}},
		{"underscore is a literal underscore", "_", []string{"Low_Key"}},
		{"backslash is a literal backslash", `\`, []string{`Back\slash`}},
		{"a percent inside the text is not a wildcard", "y%f", []string{"Fifty%Fifty"}},
		{"an underscore inside the text is not a wildcard", "w_k", []string{"Low_Key"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repos.Albums.List(ctx, repository.ListOptions{Query: tc.query, Page: 1, Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			if !equalStrings(titles(got), tc.want) {
				t.Errorf("List(Query=%q) = %v, want %v", tc.query, titles(got), tc.want)
			}
		})
	}
}

func TestAlbumListSearchKeepsTheQueryCountConstant(t *testing.T) {
	testdb.Reset(t)
	counter := &selectCounter{}
	repos := repository.NewGorm(testdb.DB(t).Session(&gorm.Session{Logger: counter}))
	ctx := context.Background()
	seedSearchCatalogue(t, repos)

	measure := func(opts repository.ListOptions) int {
		counter.selects = 0
		if _, err := repos.Albums.List(ctx, opts); err != nil {
			t.Fatal(err)
		}
		return counter.selects
	}
	plain := measure(repository.ListOptions{Page: 1, Limit: 100})
	searched := measure(repository.ListOptions{Query: "i", Page: 1, Limit: 100})
	if searched != plain {
		t.Fatalf("a search must not add statements: %d SELECTs without a query, %d with one (the artist match belongs in the same statement)", plain, searched)
	}
}

func TestCopyAvailabilityForAlbumsUsesOneQuery(t *testing.T) {
	testdb.Reset(t)
	counter := &selectCounter{}
	repos := repository.NewGorm(testdb.DB(t).Session(&gorm.Session{Logger: counter}))
	ctx := context.Background()
	genreID, ids := seedGenreAndArtists(t, repos)

	create := func(title string) int64 {
		a, err := repos.Albums.Create(ctx, albumInput(title, genreID, ids))
		if err != nil {
			t.Fatal(err)
		}
		return a.ID
	}
	withCopies, borrowedOnly, noCopies := create("With copies"), create("Borrowed only"), create("No copies")
	for i, status := range []string{model.CopyStatusAvailable, model.CopyStatusAvailable, model.CopyStatusLost} {
		if _, err := repos.Copies.Create(ctx, withCopies, model.CopyInput{InventoryNo: "A-" + string(rune('1'+i)), Status: status}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repos.Copies.Create(ctx, borrowedOnly, model.CopyInput{InventoryNo: "B-1", Status: model.CopyStatusBorrowed}); err != nil {
		t.Fatal(err)
	}

	counter.selects = 0
	got, err := repos.Copies.AvailabilityForAlbums(ctx, []int64{withCopies, borrowedOnly, noCopies, 999999})
	if err != nil {
		t.Fatal(err)
	}
	if counter.selects != 1 {
		t.Errorf("expected exactly one SELECT for the whole page, got %d", counter.selects)
	}
	want := map[int64]model.Availability{
		withCopies:   {AlbumID: withCopies, TotalCopies: 3, AvailableCopies: 2},
		borrowedOnly: {AlbumID: borrowedOnly, TotalCopies: 1, AvailableCopies: 0},
	}
	if len(got) != len(want) {
		t.Errorf("albums without copies must be absent: got %v", got)
	}
	for id, av := range want {
		if got[id] != av {
			t.Errorf("album %d: got %+v, want %+v", id, got[id], av)
		}
	}

	counter.selects = 0
	empty, err := repos.Copies.AvailabilityForAlbums(ctx, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("no ids: expected an empty, non-nil map, got %v %v", empty, err)
	}
	if counter.selects != 0 {
		t.Errorf("no ids must not touch the database, got %d SELECTs", counter.selects)
	}
}
