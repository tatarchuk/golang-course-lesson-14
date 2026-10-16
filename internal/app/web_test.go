package app_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Lesson 10 acceptance tests for the web UI: the server-rendered catalogue page, the HTMX
// row fragments and row deletion, the cookie login and the embedded static files. They run
// against the real router and database like the rest of this package and stay red until
// the web routes exist. The contract they pin down:
//
//	GET /catalog                     public page; delete buttons rendered for staff and admin only
//	GET /web/albums?q=               row fragments for an HX-Request, 303 to /catalog?q= otherwise
//	DELETE /web/albums/{id}          cookie only: anonymous 401, member 403, staff and admin empty 200
//	GET /login, POST /login, POST /logout   HttpOnly SameSite=Lax cookie albums_token with the JWT
//	GET /static/*                    embedded stylesheet and htmx, no directory listing

// webCookie is the name of the cookie that carries the web session's JWT.
const webCookie = "albums_token"

// webFixtures creates a catalogue with known titles and artists through the API: three
// albums, one of them with three copies (two available), and one with HTML in its title
// to prove autoescaping. It returns the album ids keyed by title.
func webFixtures(t *testing.T, h http.Handler) map[string]int64 {
	t.Helper()
	genre := id(expect(t, staffCall(h, "POST", "/api/v1/genres", `{"name":"Jazz"}`), http.StatusCreated, ""))
	artists := map[string]int64{}
	for _, name := range []string{"Miles Davis", "Radiohead"} {
		artists[name] = id(expect(t, staffCall(h, "POST", "/api/v1/artists", fmt.Sprintf(`{"name":%q}`, name)), http.StatusCreated, ""))
	}
	albums := map[string]int64{}
	for title, names := range map[string][]string{
		"Kind of Blue":      {"Miles Davis"},
		"Kid A":             {"Radiohead"},
		"<b>Bold</b> Title": {"Miles Davis", "Radiohead"},
	} {
		var ids []int64
		for _, n := range names {
			ids = append(ids, artists[n])
		}
		albums[title] = id(expect(t, staffCall(h, "POST", "/api/v1/albums", albumBody(title, genre, ids...)), http.StatusCreated, ""))
	}
	for i, status := range []string{"available", "available", "lost"} {
		body := fmt.Sprintf(`{"inventory_no":"KOB-%d","status":%q}`, i+1, status)
		expect(t, staffCall(h, "POST", fmt.Sprintf("/api/v1/albums/%d/copies", albums["Kind of Blue"]), body), http.StatusCreated, "")
	}
	return albums
}

// roleCookies registers one account per role and returns a session cookie for each; the
// anonymous entry is nil. Like roleTokens, the staff and admin tokens are minted with the
// shared secret and the real account ids.
func roleCookies(t *testing.T, h http.Handler) map[string]*http.Cookie {
	t.Helper()
	tokens, _ := roleTokens(t, h)
	cookies := map[string]*http.Cookie{"anonymous": nil}
	for _, role := range []string{"member", "staff", "admin"} {
		cookies[role] = &http.Cookie{Name: webCookie, Value: tokens[role]}
	}
	return cookies
}

// webRequest builds a request with an optional session cookie and extra headers.
func webRequest(method, path string, cookie *http.Cookie, headers map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

// webCall serves one request with an optional session cookie, the way a browser does.
func webCall(h http.Handler, method, path string, cookie *http.Cookie) resp {
	return serve(h, webRequest(method, path, cookie, nil))
}

// htmxCall serves one request the way htmx sends it, with the HX-Request header.
func htmxCall(h http.Handler, method, path string, cookie *http.Cookie) resp {
	return serve(h, webRequest(method, path, cookie, map[string]string{"HX-Request": "true"}))
}

// formPost posts form-encoded fields, the way a browser submits an HTML form.
func formPost(h http.Handler, path string, form url.Values, cookie *http.Cookie) resp {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return serve(h, req)
}

// setCookie returns the cookie of the given name from the response's Set-Cookie headers.
func setCookie(t *testing.T, r resp, name string) *http.Cookie {
	t.Helper()
	for _, c := range (&http.Response{Header: r.Header}).Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no Set-Cookie %q in %v", name, r.Header)
	return nil
}

// html asserts an HTML response with the given status and returns its body.
func html(t *testing.T, r resp, status int) string {
	t.Helper()
	if r.Code != status {
		t.Fatalf("expected %d, got %s", status, r)
	}
	if ct := r.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected an HTML content type, got %q", ct)
	}
	return string(r.Body)
}

// contains fails the test for every marker missing from body.
func contains(t *testing.T, what, body string, markers ...string) {
	t.Helper()
	for _, m := range markers {
		if !strings.Contains(body, m) {
			t.Errorf("%s: missing %q", what, m)
		}
	}
}

// lacks fails the test for every marker present in body.
func lacks(t *testing.T, what, body string, markers ...string) {
	t.Helper()
	for _, m := range markers {
		if strings.Contains(body, m) {
			t.Errorf("%s: must not contain %q", what, m)
		}
	}
}

func TestWebCatalogPage(t *testing.T) {
	h := newRouter(t)
	albums := webFixtures(t, h)
	cookies := roleCookies(t, h)

	t.Run("renders the layout, the search form and every album for an anonymous visitor", func(t *testing.T) {
		body := html(t, webCall(h, "GET", "/catalog", nil), http.StatusOK)
		contains(t, "catalog", strings.ToLower(body), "<!doctype html>")
		contains(t, "catalog", body,
			"<table", `id="album-list"`, `name="q"`,
			`hx-get="/web/albums"`, `hx-trigger="keyup changed delay:300ms"`, `hx-target="#album-list"`,
			`src="/static/htmx.min.js"`, `href="/static/app.css"`,
			"Kind of Blue", "Kid A", "Miles Davis", "Radiohead", "2/3", `href="/login"`)
		lacks(t, "catalog", body, "hx-delete")
	})
	t.Run("escapes HTML in album titles", func(t *testing.T) {
		body := html(t, webCall(h, "GET", "/catalog", nil), http.StatusOK)
		contains(t, "catalog", body, "&lt;b&gt;Bold&lt;/b&gt; Title")
		lacks(t, "catalog", body, "<b>Bold</b>")
	})
	t.Run("filters server-side with ?q= and echoes the query", func(t *testing.T) {
		body := html(t, webCall(h, "GET", "/catalog?q=kid", nil), http.StatusOK)
		contains(t, "catalog?q=kid", body, "Kid A", `value="kid"`)
		lacks(t, "catalog?q=kid", body, "Kind of Blue")
	})
	t.Run("shows delete buttons to staff and admin only", func(t *testing.T) {
		del := fmt.Sprintf(`hx-delete="/web/albums/%d"`, albums["Kind of Blue"])
		for _, role := range []string{"staff", "admin"} {
			body := html(t, webCall(h, "GET", "/catalog", cookies[role]), http.StatusOK)
			contains(t, role, body, del, `hx-target="closest tr"`, `hx-swap="outerHTML"`, `hx-confirm="Delete this album?"`)
		}
		for _, role := range []string{"anonymous", "member"} {
			lacks(t, role, html(t, webCall(h, "GET", "/catalog", cookies[role]), http.StatusOK), "hx-delete")
		}
	})
}

func TestWebAlbumRowsFragment(t *testing.T) {
	h := newRouter(t)
	albums := webFixtures(t, h)
	cookies := roleCookies(t, h)

	// rows fetches a fragment the way htmx does and checks that it is table rows only.
	rows := func(t *testing.T, q string, cookie *http.Cookie) string {
		t.Helper()
		body := strings.TrimSpace(html(t, htmxCall(h, "GET", "/web/albums?q="+url.QueryEscape(q), cookie), http.StatusOK))
		if !strings.HasPrefix(body, "<tr") || !strings.HasSuffix(body, "</tr>") {
			t.Fatalf("q=%q: a fragment must be table rows only, got %.120s", q, body)
		}
		lacks(t, "fragment q="+q, strings.ToLower(body), "<html", "<table", "<!doctype")
		return body
	}

	t.Run("matches the title case-insensitively", func(t *testing.T) {
		body := rows(t, "blue", nil)
		contains(t, "q=blue", body, "Kind of Blue", "2/3")
		lacks(t, "q=blue", body, "Kid A", "Bold")
	})
	t.Run("matches an artist name", func(t *testing.T) {
		body := rows(t, "radiohead", nil)
		contains(t, "q=radiohead", body, "Kid A", "Bold")
		lacks(t, "q=radiohead", body, "Kind of Blue")
	})
	t.Run("escapes HTML in a fragment too", func(t *testing.T) {
		body := rows(t, "bold", nil)
		contains(t, "q=bold", body, "&lt;b&gt;Bold&lt;/b&gt; Title")
		lacks(t, "q=bold", body, "<b>Bold</b>")
	})
	t.Run("renders one empty-state row when nothing matches", func(t *testing.T) {
		body := rows(t, "zzz", nil)
		if n := strings.Count(body, "<tr"); n != 1 || !strings.Contains(body, `class="empty"`) {
			t.Errorf("q=zzz: expected one empty row, got %s", body)
		}
	})
	t.Run("includes the delete button for staff only", func(t *testing.T) {
		del := fmt.Sprintf(`hx-delete="/web/albums/%d"`, albums["Kind of Blue"])
		contains(t, "staff fragment", rows(t, "blue", cookies["staff"]), del)
		lacks(t, "member fragment", rows(t, "blue", cookies["member"]), "hx-delete")
		lacks(t, "anonymous fragment", rows(t, "blue", nil), "hx-delete")
	})
	t.Run("redirects a plain browser request to the catalogue page", func(t *testing.T) {
		r := webCall(h, "GET", "/web/albums?q=blue", nil)
		if r.Code != http.StatusSeeOther || r.Header.Get("Location") != "/catalog?q=blue" {
			t.Errorf("expected 303 to /catalog?q=blue, got %d %q", r.Code, r.Header.Get("Location"))
		}
	})
}

func TestWebDeleteRow(t *testing.T) {
	h := newRouter(t)
	albums := webFixtures(t, h)
	cookies := roleCookies(t, h)
	path := func(title string) string { return fmt.Sprintf("/web/albums/%d", albums[title]) }
	before := countRows(t, "albums", "1 = 1")

	t.Run("anonymous gets 401", func(t *testing.T) {
		expect(t, webCall(h, "DELETE", path("Kind of Blue"), nil), http.StatusUnauthorized, "unauthenticated")
	})
	t.Run("member gets 403", func(t *testing.T) {
		expect(t, webCall(h, "DELETE", path("Kind of Blue"), cookies["member"]), http.StatusForbidden, "forbidden")
	})
	t.Run("a bearer header is not a web session", func(t *testing.T) {
		expect(t, authCall(h, "DELETE", path("Kind of Blue"), "", mintToken(1, "staff", time.Hour)), http.StatusUnauthorized, "unauthenticated")
	})
	t.Run("an expired cookie gets 401", func(t *testing.T) {
		expired := &http.Cookie{Name: webCookie, Value: mintToken(1, "staff", -time.Hour)}
		expect(t, webCall(h, "DELETE", path("Kind of Blue"), expired), http.StatusUnauthorized, "unauthenticated")
	})
	t.Run("a cross-site request is refused even with a valid cookie", func(t *testing.T) {
		req := webRequest("DELETE", path("Kind of Blue"), cookies["staff"], map[string]string{"Sec-Fetch-Site": "cross-site"})
		if r := serve(h, req); r.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %s", r)
		}
	})
	if n := countRows(t, "albums", "1 = 1"); n != before {
		t.Fatalf("rejected deletes must change nothing: %d albums before, %d after", before, n)
	}
	t.Run("staff deletes with an empty 200 and the album and its copies are gone", func(t *testing.T) {
		r := webCall(h, "DELETE", path("Kind of Blue"), cookies["staff"])
		if r.Code != http.StatusOK || len(r.Body) != 0 {
			t.Fatalf("expected an empty 200, got %s", r)
		}
		expect(t, call(h, "GET", fmt.Sprintf("/api/v1/albums/%d", albums["Kind of Blue"]), ""), http.StatusNotFound, "not_found")
		if n := countRows(t, "copies", "album_id = ?", albums["Kind of Blue"]); n != 0 {
			t.Errorf("copies should cascade, found %d", n)
		}
	})
	t.Run("admin deletes too", func(t *testing.T) {
		if r := webCall(h, "DELETE", path("Kid A"), cookies["admin"]); r.Code != http.StatusOK || len(r.Body) != 0 {
			t.Errorf("expected an empty 200, got %s", r)
		}
	})
	t.Run("unknown and malformed ids", func(t *testing.T) {
		expect(t, webCall(h, "DELETE", "/web/albums/999999", cookies["staff"]), http.StatusNotFound, "not_found")
		expect(t, webCall(h, "DELETE", "/web/albums/abc", cookies["staff"]), http.StatusBadRequest, "invalid_id")
	})
}

func TestWebLoginAndLogout(t *testing.T) {
	h := newRouter(t)
	webFixtures(t, h)
	register(t, h, "alice@example.com")
	var session *http.Cookie

	t.Run("the login page is a form posting e-mail and password", func(t *testing.T) {
		body := html(t, webCall(h, "GET", "/login", nil), http.StatusOK)
		contains(t, "login", body, "<form", `method="post"`, `action="/login"`, `name="email"`, `name="password"`)
	})
	t.Run("a valid login sets an HttpOnly SameSite cookie and redirects to the catalogue", func(t *testing.T) {
		r := formPost(h, "/login", url.Values{"email": {"alice@example.com"}, "password": {testPassword}}, nil)
		if r.Code != http.StatusSeeOther || r.Header.Get("Location") != "/catalog" {
			t.Fatalf("expected 303 to /catalog, got %d %q %s", r.Code, r.Header.Get("Location"), r.Body)
		}
		c := setCookie(t, r, webCookie)
		if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 86400 || c.Secure {
			t.Errorf("cookie attributes: %+v", c)
		}
		if strings.Count(c.Value, ".") != 2 {
			t.Errorf("the cookie must carry a compact JWT, got %q", c.Value)
		}
		session = &http.Cookie{Name: c.Name, Value: c.Value}
	})
	t.Run("the cookie identifies the account on the catalogue page", func(t *testing.T) {
		if session == nil {
			t.Fatal("no session cookie from the login subtest")
		}
		body := html(t, webCall(h, "GET", "/catalog", session), http.StatusOK)
		contains(t, "catalog as alice", body, "alice@example.com", "member", `action="/logout"`)
		lacks(t, "catalog as alice", body, "hx-delete", `href="/login"`)
	})
	t.Run("an unknown e-mail and a wrong password are indistinguishable", func(t *testing.T) {
		wrong := formPost(h, "/login", url.Values{"email": {"alice@example.com"}, "password": {"not-the-password"}}, nil)
		unknown := formPost(h, "/login", url.Values{"email": {"nobody@example.com"}, "password": {testPassword}}, nil)
		for _, r := range []resp{wrong, unknown} {
			contains(t, "failed login", html(t, r, http.StatusUnauthorized), "invalid credentials", "<form")
			if r.Header.Get("Set-Cookie") != "" {
				t.Errorf("a failed login must not set a cookie: %v", r.Header)
			}
		}
		if !bytes.Equal(wrong.Body, unknown.Body) {
			t.Error("the two failed logins must have byte-identical bodies")
		}
	})
	t.Run("logout expires the cookie and redirects", func(t *testing.T) {
		if session == nil {
			t.Fatal("no session cookie from the login subtest")
		}
		r := formPost(h, "/logout", nil, session)
		if r.Code != http.StatusSeeOther || r.Header.Get("Location") != "/catalog" {
			t.Fatalf("expected 303 to /catalog, got %d %q", r.Code, r.Header.Get("Location"))
		}
		c := setCookie(t, r, webCookie)
		expired := c.MaxAge < 0 || (!c.Expires.IsZero() && c.Expires.Before(time.Now()))
		if c.Value != "" || !expired {
			t.Errorf("logout must expire the cookie, got %+v", c)
		}
	})
}

func TestWebStaticAssets(t *testing.T) {
	h := newRouter(t)
	for _, tc := range []struct{ path, contentType string }{
		{"/static/app.css", "text/css"},
		{"/static/htmx.min.js", "javascript"},
	} {
		r := call(h, "GET", tc.path, "")
		if r.Code != http.StatusOK || !strings.Contains(r.Header.Get("Content-Type"), tc.contentType) || len(r.Body) == 0 {
			t.Errorf("%s: expected 200 %s with a body, got %d %q (%d bytes)", tc.path, tc.contentType, r.Code, r.Header.Get("Content-Type"), len(r.Body))
		}
	}
	if r := call(h, "GET", "/static/missing.css", ""); r.Code != http.StatusNotFound {
		t.Errorf("missing asset: expected 404, got %s", r)
	}
	if r := call(h, "GET", "/static/", ""); r.Code == http.StatusOK {
		t.Errorf("a directory listing must not be served, got %s", r)
	}
}
