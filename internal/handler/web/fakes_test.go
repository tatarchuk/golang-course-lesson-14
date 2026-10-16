package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"homework/internal/auth"
	"homework/internal/handler"
	"homework/internal/middleware"
	"homework/internal/model"
	"homework/internal/repository"
	webfs "homework/web"
)

const testSecret = "web-handler-test-secret"

// Fake repositories in the style of internal/handler/fakes_test.go: structs of function
// fields, and a call the test did not expect fails it. Only the methods the web handlers
// use are configurable; the others always fail.

type fakeAlbums struct {
	t      *testing.T
	list   func(repository.ListOptions) ([]model.Album, error)
	delete func(int64) error
}

var _ repository.AlbumRepository = (*fakeAlbums)(nil)

func (f *fakeAlbums) Create(context.Context, model.AlbumInput) (model.Album, error) {
	f.t.Fatal("unexpected Create call")
	return model.Album{}, nil
}

func (f *fakeAlbums) Get(context.Context, int64) (model.Album, error) {
	f.t.Fatal("unexpected Get call")
	return model.Album{}, nil
}

func (f *fakeAlbums) List(_ context.Context, opts repository.ListOptions) ([]model.Album, error) {
	if f.list == nil {
		f.t.Fatal("unexpected List call")
	}
	return f.list(opts)
}

func (f *fakeAlbums) Update(context.Context, int64, model.AlbumInput) (model.Album, error) {
	f.t.Fatal("unexpected Update call")
	return model.Album{}, nil
}

func (f *fakeAlbums) Delete(_ context.Context, id int64) error {
	if f.delete == nil {
		f.t.Fatal("unexpected Delete call")
	}
	return f.delete(id)
}

type fakeCopies struct {
	t                     *testing.T
	availabilityForAlbums func([]int64) (map[int64]model.Availability, error)
}

var _ repository.CopyRepository = (*fakeCopies)(nil)

func (f *fakeCopies) Create(context.Context, int64, model.CopyInput) (model.Copy, error) {
	f.t.Fatal("unexpected Create call")
	return model.Copy{}, nil
}

func (f *fakeCopies) ListByAlbum(context.Context, int64) ([]model.Copy, error) {
	f.t.Fatal("unexpected ListByAlbum call")
	return nil, nil
}

func (f *fakeCopies) Availability(context.Context, int64) (model.Availability, error) {
	f.t.Fatal("unexpected Availability call")
	return model.Availability{}, nil
}

func (f *fakeCopies) AvailabilityForAlbums(_ context.Context, albumIDs []int64) (map[int64]model.Availability, error) {
	if f.availabilityForAlbums == nil {
		f.t.Fatal("unexpected AvailabilityForAlbums call")
	}
	return f.availabilityForAlbums(albumIDs)
}

type fakeUsers struct {
	t          *testing.T
	get        func(int64) (model.User, error)
	getByEmail func(string) (model.User, error)
}

var _ repository.UserRepository = (*fakeUsers)(nil)

func (f *fakeUsers) Create(context.Context, string, string) (model.User, error) {
	f.t.Fatal("unexpected Create call")
	return model.User{}, nil
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

func (f *fakeUsers) List(context.Context) ([]model.User, error) {
	f.t.Fatal("unexpected List call")
	return nil, nil
}

func (f *fakeUsers) UpdateRole(context.Context, int64, string) (model.User, error) {
	f.t.Fatal("unexpected UpdateRole call")
	return model.User{}, nil
}

func (f *fakeUsers) UpsertAdmin(context.Context, string, string) (model.User, error) {
	f.t.Fatal("unexpected UpsertAdmin call")
	return model.User{}, nil
}

func year(y int) *int { return &y }

// catalogue is the album list the fake repository serves: two ordinary albums and one
// whose title carries HTML, to prove the autoescaping.
func catalogue() []model.Album {
	return []model.Album{
		{ID: 1, Title: "Kind of Blue", Label: "Columbia", Genre: model.Genre{ID: 1, Name: "jazz"},
			Artists: []model.Artist{{ID: 1, Name: "Miles Davis"}, {ID: 2, Name: "John Coltrane"}}, ReleaseYear: year(1959)},
		{ID: 2, Title: "Kid A", Label: "Parlophone", Genre: model.Genre{ID: 2, Name: "rock"},
			Artists: []model.Artist{{ID: 3, Name: "Radiohead"}}, ReleaseYear: year(2000)},
		{ID: 3, Title: "<script>alert(1)</script>", Label: "Nowhere", Genre: model.Genre{ID: 2, Name: "rock"},
			Artists: []model.Artist{{ID: 3, Name: "Radiohead"}}},
	}
}

// filterByTitle mimics the repository's search closely enough for the handler tests: a
// case-insensitive substring of the title.
func filterByTitle(albums []model.Album, query string) []model.Album {
	if query == "" {
		return albums
	}
	out := []model.Album{}
	for _, a := range albums {
		if strings.Contains(strings.ToLower(a.Title), strings.ToLower(query)) {
			out = append(out, a)
		}
	}
	return out
}

// accounts are the users the fake knows by id: one per role.
var accounts = map[int64]model.User{
	1: {ID: 1, Email: "member@albums.local", Role: model.UserRoleMember},
	2: {ID: 2, Email: "staff@albums.local", Role: model.UserRoleStaff},
	3: {ID: 3, Email: "admin@albums.local", Role: model.UserRoleAdmin},
}

// notFoundID is the album id the fake Delete reports as missing.
const notFoundID = 404

// fixture bundles the fakes of one test and records the calls the handlers made.
type fixture struct {
	t        *testing.T
	albums   *fakeAlbums
	copies   *fakeCopies
	users    *fakeUsers
	listOpts []repository.ListOptions
	deleted  []int64
}

func newFixture(t *testing.T) *fixture {
	fx := &fixture{t: t}
	fx.albums = &fakeAlbums{
		t: t,
		list: func(opts repository.ListOptions) ([]model.Album, error) {
			fx.listOpts = append(fx.listOpts, opts)
			return filterByTitle(catalogue(), opts.Query), nil
		},
		delete: func(id int64) error {
			fx.deleted = append(fx.deleted, id)
			if id == notFoundID {
				return repository.ErrNotFound
			}
			return nil
		},
	}
	fx.copies = &fakeCopies{t: t, availabilityForAlbums: func([]int64) (map[int64]model.Availability, error) {
		// Album 1 has two available copies out of three; the others have none.
		return map[int64]model.Availability{1: {AlbumID: 1, TotalCopies: 3, AvailableCopies: 2}}, nil
	}}
	fx.users = &fakeUsers{
		t: t,
		get: func(id int64) (model.User, error) {
			u, ok := accounts[id]
			if !ok {
				return model.User{}, repository.ErrNotFound
			}
			return u, nil
		},
		getByEmail: func(string) (model.User, error) { return model.User{}, repository.ErrNotFound },
	}
	return fx
}

// server routes the handler exactly as app.go does: CookieAuth identifies the visitor,
// RequireRole guards the delete route, CrossOriginProtection wraps the web routes, and
// a fresh ServeMux gives the handlers their path values.
func (fx *fixture) server() http.Handler {
	fx.t.Helper()
	h, err := New(repository.Repositories{Albums: fx.albums, Copies: fx.copies, Users: fx.users}, webfs.Templates(), testSecret, false)
	if err != nil {
		fx.t.Fatal(err)
	}
	cookie := middleware.CookieAuth(testSecret, CookieName)
	staff := func(hf http.HandlerFunc) http.Handler {
		return cookie(middleware.RequireRole(model.UserRoleStaff, model.UserRoleAdmin)(hf))
	}
	csrf := http.NewCrossOriginProtection()
	mux := http.NewServeMux()
	mux.Handle("GET /catalog", csrf.Handler(cookie(http.HandlerFunc(h.Catalog))))
	mux.Handle("GET /login", csrf.Handler(cookie(http.HandlerFunc(h.LoginForm))))
	mux.Handle("POST /login", csrf.Handler(http.HandlerFunc(h.Login)))
	mux.Handle("POST /logout", csrf.Handler(http.HandlerFunc(h.Logout)))
	mux.Handle("GET /web/albums", csrf.Handler(cookie(http.HandlerFunc(h.Rows))))
	mux.Handle("DELETE /web/albums/{id}", csrf.Handler(staff(h.Delete)))
	mux.Handle("GET /static/", StaticFiles(webfs.Static()))
	return mux
}

// sessionFor mints the session cookie of the account with the given id and role.
func sessionFor(t *testing.T, id int64, role string) *http.Cookie {
	t.Helper()
	token, err := auth.IssueToken(testSecret, id, role, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: CookieName, Value: token}
}

// request builds one request. A non-nil form is posted form-encoded, the way a browser
// submits an HTML form; the cookie and the headers are optional.
func request(method, target string, form url.Values, cookie *http.Cookie, headers map[string]string) *http.Request {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// htmlBody asserts an HTML response with the status and returns its body.
func htmlBody(t *testing.T, rec *httptest.ResponseRecorder, status int) string {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected an HTML content type, got %q", ct)
	}
	return rec.Body.String()
}

// expectError asserts the API's JSON error envelope with the status and code.
func expectError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected a JSON error, got %q", ct)
	}
	var env handler.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != code || env.Error.Message == "" {
		t.Fatalf("expected error code %q, got %s (%v)", code, rec.Body.String(), err)
	}
}

// setCookie returns the cookie of the name from the response, or fails.
func setCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no Set-Cookie %q in %v", name, rec.Header())
	return nil
}

// contains and lacks fail the test for every marker that is missing from, or present in, body.
func contains(t *testing.T, body string, markers ...string) {
	t.Helper()
	for _, m := range markers {
		if !strings.Contains(body, m) {
			t.Errorf("missing %q", m)
		}
	}
}

func lacks(t *testing.T, body string, markers ...string) {
	t.Helper()
	for _, m := range markers {
		if strings.Contains(body, m) {
			t.Errorf("must not contain %q", m)
		}
	}
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
