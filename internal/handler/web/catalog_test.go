package web

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"homework/internal/auth"
	"homework/internal/handler"
	"homework/internal/model"
	"homework/internal/repository"
)

// Lesson 10, Task 3.2: table-driven tests of the web handlers on fake repositories. No
// database, so they never skip; the routing mirrors app.go (see fixture.server).

func TestCatalogPage(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		cookie   *http.Cookie
		query    string // the Query the fake List must receive
		contains []string
		lacks    []string
	}{
		{
			name:   "an anonymous visitor sees the layout, the search form, every album and the login link",
			target: "/catalog",
			contains: []string{
				`<header class="topbar"`, `href="/login"`, `<table class="albums"`, `id="album-list"`,
				`name="q"`, `hx-get="/web/albums"`, `hx-trigger="keyup changed delay:300ms"`, `hx-target="#album-list"`,
				`src="/static/htmx.min.js"`, `href="/static/app.css"`,
				"Kind of Blue", "Kid A", "Miles Davis, John Coltrane", "Radiohead", "jazz", "rock", "1959", "Columbia",
				`<td class="availability">2/3</td>`, `<td class="availability">0/0</td>`,
			},
			lacks: []string{"hx-delete", `action="/logout"`},
		},
		{
			name:     "HTML in a title is escaped",
			target:   "/catalog",
			contains: []string{"&lt;script&gt;alert(1)&lt;/script&gt;"},
			lacks:    []string{"<script>alert(1)"},
		},
		{
			name:     "the query reaches the repository and is echoed into the input",
			target:   "/catalog?q=kid",
			query:    "kid",
			contains: []string{"Kid A", `value="kid"`},
			lacks:    []string{"Kind of Blue"},
		},
		{
			name:   "a staff cookie shows the account and one delete button per row",
			target: "/catalog",
			cookie: sessionFor(t, 2, model.UserRoleStaff),
			contains: []string{
				"staff@albums.local (staff)", `action="/logout"`,
				`hx-delete="/web/albums/1"`, `hx-delete="/web/albums/2"`, `hx-delete="/web/albums/3"`,
				`hx-target="closest tr"`, `hx-swap="outerHTML"`, `hx-confirm="Delete this album?"`,
			},
			lacks: []string{`href="/login"`},
		},
		{
			name:     "an admin cookie shows the delete buttons too",
			target:   "/catalog",
			cookie:   sessionFor(t, 3, model.UserRoleAdmin),
			contains: []string{"admin@albums.local (admin)", `hx-delete="/web/albums/1"`},
		},
		{
			name:     "a member cookie shows the account but no delete button",
			target:   "/catalog",
			cookie:   sessionFor(t, 1, model.UserRoleMember),
			contains: []string{"member@albums.local (member)", `action="/logout"`},
			lacks:    []string{"hx-delete", `href="/login"`},
		},
		{
			name:     "a cookie of an account that no longer exists renders as anonymous",
			target:   "/catalog",
			cookie:   sessionFor(t, 99, model.UserRoleStaff),
			contains: []string{`href="/login"`},
			lacks:    []string{"hx-delete", `action="/logout"`},
		},
		{
			name:     "the empty state spans the six columns a visitor sees",
			target:   "/catalog?q=zzz",
			query:    "zzz",
			contains: []string{`<tr class="empty"><td colspan="6">`},
		},
		{
			name:     "the empty state spans the seven columns staff see",
			target:   "/catalog?q=zzz",
			cookie:   sessionFor(t, 2, model.UserRoleStaff),
			query:    "zzz",
			contains: []string{`<tr class="empty"><td colspan="7">`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t)
			rec := do(fx.server(), request(http.MethodGet, tc.target, nil, tc.cookie, nil))
			body := htmlBody(t, rec, http.StatusOK)
			if !strings.HasPrefix(body, "<!doctype html>") {
				t.Errorf("the page must start with the doctype, got %.40q", body)
			}
			contains(t, body, tc.contains...)
			lacks(t, body, tc.lacks...)
			if len(fx.listOpts) != 1 || fx.listOpts[0].Query != tc.query || fx.listOpts[0].Limit != pageLimit || fx.listOpts[0].Page != 1 {
				t.Errorf("List options = %+v, want one call with Query %q, Page 1 and Limit %d", fx.listOpts, tc.query, pageLimit)
			}
		})
	}
}

func TestRowsFragment(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		htmx     bool
		cookie   *http.Cookie
		query    string
		rows     int
		location string
		contains []string
		lacks    []string
	}{
		{
			name: "an htmx request gets the matching rows only", target: "/web/albums?q=blue", htmx: true, query: "blue", rows: 1,
			contains: []string{`<tr id="album-1">`, "Kind of Blue", `<td class="availability">2/3</td>`},
			lacks:    []string{"<html", "<table", "<!doctype", "Kid A", "hx-delete"},
		},
		{
			name: "no match renders exactly one empty row", target: "/web/albums?q=zzz", htmx: true, query: "zzz", rows: 1,
			contains: []string{`<tr class="empty">`, "No albums match your search."},
		},
		{
			name: "a staff cookie adds the delete button to the rows", target: "/web/albums?q=kid", htmx: true, query: "kid", rows: 1,
			cookie:   sessionFor(t, 2, model.UserRoleStaff),
			contains: []string{`hx-delete="/web/albums/2"`, `hx-swap="outerHTML"`},
		},
		{
			name: "an empty query lists everything", target: "/web/albums", htmx: true, rows: 3,
			contains: []string{"Kind of Blue", "Kid A", "&lt;script&gt;alert(1)&lt;/script&gt;"},
			lacks:    []string{"<script>alert(1)"},
		},
		{name: "a plain browser request is redirected to the page", target: "/web/albums?q=blue", location: "/catalog?q=blue"},
		{name: "a plain request without a query goes to the bare page", target: "/web/albums", location: "/catalog"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t)
			var headers map[string]string
			if tc.htmx {
				headers = map[string]string{"HX-Request": "true"}
			}
			rec := do(fx.server(), request(http.MethodGet, tc.target, nil, tc.cookie, headers))
			if tc.location != "" {
				if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tc.location {
					t.Fatalf("expected 303 to %s, got %d %q", tc.location, rec.Code, rec.Header().Get("Location"))
				}
				if len(fx.listOpts) != 0 {
					t.Errorf("a redirect must not query the repository: %+v", fx.listOpts)
				}
				return
			}
			body := strings.TrimSpace(htmlBody(t, rec, http.StatusOK))
			if !strings.HasPrefix(body, "<tr") || !strings.HasSuffix(body, "</tr>") {
				t.Errorf("a fragment must be table rows only, got %.80q", body)
			}
			if n := strings.Count(body, "<tr"); n != tc.rows {
				t.Errorf("expected %d rows, got %d: %s", tc.rows, n, body)
			}
			contains(t, body, tc.contains...)
			lacks(t, body, tc.lacks...)
			if len(fx.listOpts) != 1 || fx.listOpts[0].Query != tc.query {
				t.Errorf("List options = %+v, want one call with Query %q", fx.listOpts, tc.query)
			}
		})
	}
}

func TestDeleteRow(t *testing.T) {
	staff := sessionFor(t, 2, model.UserRoleStaff)
	cases := []struct {
		name    string
		target  string
		cookie  *http.Cookie
		headers map[string]string
		status  int
		code    string  // the JSON error code, "" for a non-JSON answer
		deleted []int64 // what reached the repository
	}{
		{name: "no cookie is 401", target: "/web/albums/1", status: http.StatusUnauthorized, code: handler.CodeUnauthenticated},
		{name: "a member is 403", target: "/web/albums/1", cookie: sessionFor(t, 1, model.UserRoleMember), status: http.StatusForbidden, code: handler.CodeForbidden},
		{name: "a bearer header is not a web session", target: "/web/albums/1", headers: map[string]string{"Authorization": "Bearer " + staff.Value}, status: http.StatusUnauthorized, code: handler.CodeUnauthenticated},
		{name: "staff deletes with an empty 200", target: "/web/albums/1", cookie: staff, status: http.StatusOK, deleted: []int64{1}},
		{name: "admin deletes with an empty 200", target: "/web/albums/2", cookie: sessionFor(t, 3, model.UserRoleAdmin), status: http.StatusOK, deleted: []int64{2}},
		{name: "an unknown album is 404", target: "/web/albums/404", cookie: staff, status: http.StatusNotFound, code: handler.CodeNotFound, deleted: []int64{notFoundID}},
		{name: "a malformed id is 400 before the repository is asked", target: "/web/albums/abc", cookie: staff, status: http.StatusBadRequest, code: handler.CodeInvalidID},
		{name: "a cross-site request is refused even with a valid cookie", target: "/web/albums/1", cookie: staff, headers: map[string]string{"Sec-Fetch-Site": "cross-site"}, status: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t)
			rec := do(fx.server(), request(http.MethodDelete, tc.target, nil, tc.cookie, tc.headers))
			switch {
			case tc.code != "":
				expectError(t, rec, tc.status, tc.code)
			case tc.status == http.StatusOK:
				if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
					t.Fatalf("expected an empty 200 (htmx swaps nothing on a 204), got %d %q", rec.Code, rec.Body.String())
				}
			default:
				if rec.Code != tc.status {
					t.Fatalf("expected %d, got %d %s", tc.status, rec.Code, rec.Body.String())
				}
			}
			if !equalIDs(fx.deleted, tc.deleted) {
				t.Errorf("Delete calls = %v, want %v", fx.deleted, tc.deleted)
			}
		})
	}
}

func TestLoginAndLogout(t *testing.T) {
	hash, err := auth.HashPassword("Correct-Horse-Battery-1")
	if err != nil {
		t.Fatal(err)
	}
	alice := model.User{ID: 7, Email: "alice@example.com", Role: model.UserRoleMember, PasswordHash: hash}
	// newServer knows alice by e-mail, like the real repository would after a registration.
	newServerSecure := func(t *testing.T, secure bool) http.Handler {
		fx := newFixture(t)
		fx.secure = secure
		fx.users.getByEmail = func(email string) (model.User, error) {
			if email == alice.Email {
				return alice, nil
			}
			return model.User{}, repository.ErrNotFound
		}
		return fx.server()
	}
	newServer := func(t *testing.T) http.Handler { return newServerSecure(t, false) }
	credentials := func(email, password string) url.Values {
		return url.Values{"email": {email}, "password": {password}}
	}

	t.Run("GET /login renders the form", func(t *testing.T) {
		body := htmlBody(t, do(newServer(t), request(http.MethodGet, "/login", nil, nil, nil)), http.StatusOK)
		contains(t, body, `<form method="post" action="/login"`, `name="email"`, `name="password"`, `type="password"`, `href="/login"`)
		lacks(t, body, `value="`, "invalid credentials")
	})
	t.Run("a valid login sets the session cookie and redirects to the catalogue", func(t *testing.T) {
		rec := do(newServer(t), request(http.MethodPost, "/login", credentials("alice@example.com", "Correct-Horse-Battery-1"), nil, nil))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/catalog" {
			t.Fatalf("expected 303 to /catalog, got %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
		}
		c := setCookie(t, rec, CookieName)
		if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != int(auth.TokenTTL.Seconds()) || c.Secure {
			t.Errorf("cookie attributes: %+v", c)
		}
		claims, err := auth.ParseToken(testSecret, c.Value)
		if err != nil || claims.UserID != alice.ID || claims.Role != alice.Role {
			t.Errorf("the cookie must carry alice's token, got %+v (%v)", claims, err)
		}
	})
	t.Run("COOKIE_SECURE marks the cookie Secure", func(t *testing.T) {
		rec := do(newServerSecure(t, true), request(http.MethodPost, "/login", credentials("alice@example.com", "Correct-Horse-Battery-1"), nil, nil))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303, got %d %s", rec.Code, rec.Body.String())
		}
		if c := setCookie(t, rec, CookieName); !c.Secure || !c.HttpOnly {
			t.Errorf("expected a Secure, HttpOnly cookie, got %+v", c)
		}
	})
	t.Run("an oversized form body is a 400 page, not a bcrypt run", func(t *testing.T) {
		huge := credentials("alice@example.com", strings.Repeat("x", 20<<10))
		body := htmlBody(t, do(newServer(t), request(http.MethodPost, "/login", huge, nil, nil)), http.StatusBadRequest)
		contains(t, body, "the form could not be read", `<form method="post" action="/login"`)
	})
	t.Run("the e-mail is normalised before the lookup", func(t *testing.T) {
		rec := do(newServer(t), request(http.MethodPost, "/login", credentials("  Alice@Example.COM ", "Correct-Horse-Battery-1"), nil, nil))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303, got %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("a wrong password and an unknown e-mail are indistinguishable", func(t *testing.T) {
		wrong := do(newServer(t), request(http.MethodPost, "/login", credentials("alice@example.com", "not-the-password"), nil, nil))
		unknown := do(newServer(t), request(http.MethodPost, "/login", credentials("nobody@example.com", "Correct-Horse-Battery-1"), nil, nil))
		for _, rec := range []*http.Response{wrong.Result(), unknown.Result()} {
			if rec.StatusCode != http.StatusUnauthorized || len(rec.Cookies()) != 0 {
				t.Fatalf("expected a 401 without Set-Cookie, got %d with %d cookies", rec.StatusCode, len(rec.Cookies()))
			}
		}
		body := htmlBody(t, wrong, http.StatusUnauthorized)
		contains(t, body, "invalid credentials", `<form method="post" action="/login"`)
		lacks(t, body, "alice@example.com")
		if !bytes.Equal(wrong.Body.Bytes(), unknown.Body.Bytes()) {
			t.Error("the two failed logins must have byte-identical bodies")
		}
	})
	t.Run("a form that cannot be parsed is a 400 page", func(t *testing.T) {
		req := request(http.MethodPost, "/login", nil, nil, map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
		req.Body = io.NopCloser(strings.NewReader("email=%zz&password=x"))
		body := htmlBody(t, do(newServer(t), req), http.StatusBadRequest)
		contains(t, body, "the form could not be read", `<form method="post" action="/login"`)
	})
	t.Run("logout expires the cookie and redirects", func(t *testing.T) {
		rec := do(newServer(t), request(http.MethodPost, "/logout", url.Values{}, sessionFor(t, 7, model.UserRoleMember), nil))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/catalog" {
			t.Fatalf("expected 303 to /catalog, got %d %q", rec.Code, rec.Header().Get("Location"))
		}
		c := setCookie(t, rec, CookieName)
		if c.Value != "" || c.MaxAge >= 0 {
			t.Errorf("logout must expire the cookie, got %+v", c)
		}
	})
}
