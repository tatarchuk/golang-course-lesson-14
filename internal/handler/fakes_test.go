package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"homework/internal/model"
	"homework/internal/repository"
)

// Fakes implement the repository interfaces with function fields, so every test
// states exactly what the storage returns. A nil function fails the test when called.

type fakeAlbums struct {
	t      *testing.T
	create func(model.AlbumInput) (model.Album, error)
	get    func(int64) (model.Album, error)
	list   func(repository.ListOptions) ([]model.Album, error)
	update func(int64, model.AlbumInput) (model.Album, error)
	delete func(int64) error
}

func (f *fakeAlbums) Create(_ context.Context, in model.AlbumInput) (model.Album, error) {
	if f.create == nil {
		f.t.Fatal("unexpected Create call")
	}
	return f.create(in)
}

func (f *fakeAlbums) Get(_ context.Context, id int64) (model.Album, error) {
	if f.get == nil {
		f.t.Fatal("unexpected Get call")
	}
	return f.get(id)
}

func (f *fakeAlbums) List(_ context.Context, opts repository.ListOptions) ([]model.Album, error) {
	if f.list == nil {
		f.t.Fatal("unexpected List call")
	}
	return f.list(opts)
}

func (f *fakeAlbums) Update(_ context.Context, id int64, in model.AlbumInput) (model.Album, error) {
	if f.update == nil {
		f.t.Fatal("unexpected Update call")
	}
	return f.update(id, in)
}

func (f *fakeAlbums) Delete(_ context.Context, id int64) error {
	if f.delete == nil {
		f.t.Fatal("unexpected Delete call")
	}
	return f.delete(id)
}

type fakeGenres struct {
	t      *testing.T
	create func(model.GenreInput) (model.Genre, error)
	get    func(int64) (model.Genre, error)
	list   func() ([]model.Genre, error)
}

func (f *fakeGenres) Create(_ context.Context, in model.GenreInput) (model.Genre, error) {
	if f.create == nil {
		f.t.Fatal("unexpected Create call")
	}
	return f.create(in)
}

func (f *fakeGenres) Get(_ context.Context, id int64) (model.Genre, error) {
	if f.get == nil {
		f.t.Fatal("unexpected Get call")
	}
	return f.get(id)
}

func (f *fakeGenres) List(context.Context) ([]model.Genre, error) {
	if f.list == nil {
		f.t.Fatal("unexpected List call")
	}
	return f.list()
}

type fakeArtists struct {
	t      *testing.T
	create func(model.ArtistInput) (model.Artist, error)
	get    func(int64) (model.Artist, error)
	list   func() ([]model.Artist, error)
}

func (f *fakeArtists) Create(_ context.Context, in model.ArtistInput) (model.Artist, error) {
	if f.create == nil {
		f.t.Fatal("unexpected Create call")
	}
	return f.create(in)
}

func (f *fakeArtists) Get(_ context.Context, id int64) (model.Artist, error) {
	if f.get == nil {
		f.t.Fatal("unexpected Get call")
	}
	return f.get(id)
}

func (f *fakeArtists) List(context.Context) ([]model.Artist, error) {
	if f.list == nil {
		f.t.Fatal("unexpected List call")
	}
	return f.list()
}

type fakeCopies struct {
	t                     *testing.T
	create                func(int64, model.CopyInput) (model.Copy, error)
	listByAlbum           func(int64) ([]model.Copy, error)
	availability          func(int64) (model.Availability, error)
	availabilityForAlbums func([]int64) (map[int64]model.Availability, error)
}

func (f *fakeCopies) Create(_ context.Context, albumID int64, in model.CopyInput) (model.Copy, error) {
	if f.create == nil {
		f.t.Fatal("unexpected Create call")
	}
	return f.create(albumID, in)
}

func (f *fakeCopies) ListByAlbum(_ context.Context, albumID int64) ([]model.Copy, error) {
	if f.listByAlbum == nil {
		f.t.Fatal("unexpected ListByAlbum call")
	}
	return f.listByAlbum(albumID)
}

func (f *fakeCopies) Availability(_ context.Context, albumID int64) (model.Availability, error) {
	if f.availability == nil {
		f.t.Fatal("unexpected Availability call")
	}
	return f.availability(albumID)
}

func (f *fakeCopies) AvailabilityForAlbums(_ context.Context, albumIDs []int64) (map[int64]model.Availability, error) {
	if f.availabilityForAlbums == nil {
		f.t.Fatal("unexpected AvailabilityForAlbums call")
	}
	return f.availabilityForAlbums(albumIDs)
}

// serve routes one request through a mux registered by the caller, so handlers see
// path values exactly as they do in the real router.
func serve(register func(*http.ServeMux), method, target, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	register(mux)
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// errorCode decodes the error envelope and returns its code.
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("response %q is not an error envelope: %v", rec.Body.String(), err)
	}
	if env.Error.Message == "" {
		t.Errorf("error message must not be empty: %s", rec.Body.String())
	}
	return env.Error.Code
}

func expectError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, rec.Code, rec.Body.String())
	}
	if got := errorCode(t, rec); got != code {
		t.Fatalf("expected code %q, got %q", code, got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected JSON content type, got %q", ct)
	}
}

const validAlbumBody = `{"title":"Kind of Blue","label":"Columbia","genre_id":1,"artist_ids":[1,2],"release_year":1959}`

type fakeUsers struct {
	t           *testing.T
	create      func(email, hash string) (model.User, error)
	get         func(int64) (model.User, error)
	getByEmail  func(string) (model.User, error)
	list        func() ([]model.User, error)
	updateRole  func(int64, string) (model.User, error)
	upsertAdmin func(email, hash string) (model.User, error)
}

var _ repository.UserRepository = (*fakeUsers)(nil)

func (f *fakeUsers) Create(_ context.Context, email, hash string) (model.User, error) {
	if f.create == nil {
		f.t.Fatal("unexpected Create call")
	}
	return f.create(email, hash)
}

func (f *fakeUsers) Get(_ context.Context, id int64) (model.User, error) {
	if f.get == nil {
		f.t.Fatal("unexpected Get call")
	}
	return f.get(id)
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (model.User, error) {
	if f.getByEmail == nil {
		f.t.Fatal("unexpected GetByEmail call")
	}
	return f.getByEmail(email)
}

func (f *fakeUsers) List(_ context.Context) ([]model.User, error) {
	if f.list == nil {
		f.t.Fatal("unexpected List call")
	}
	return f.list()
}

func (f *fakeUsers) UpdateRole(_ context.Context, id int64, role string) (model.User, error) {
	if f.updateRole == nil {
		f.t.Fatal("unexpected UpdateRole call")
	}
	return f.updateRole(id, role)
}

func (f *fakeUsers) UpsertAdmin(_ context.Context, email, hash string) (model.User, error) {
	if f.upsertAdmin == nil {
		f.t.Fatal("unexpected UpsertAdmin call")
	}
	return f.upsertAdmin(email, hash)
}
