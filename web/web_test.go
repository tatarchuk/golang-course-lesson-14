package web

import (
	"bytes"
	"html/template"
	"io/fs"
	"strings"
	"testing"
)

// pageData mirrors the data contract of the templates (see prompts.txt, prompt 2.1); the
// real type lives in internal/handler/web, this copy keeps the package dependency-free.
type pageData struct {
	Title     string
	User      *struct{ Email, Role string }
	CanDelete bool
	Query     string
	Albums    []albumRow
	Error     string
}

type albumRow struct {
	ID                    int64
	Title, Artists, Genre string
	ReleaseYear           *int
	Label                 string
	Available, Total      int
	CanDelete             bool
}

// parsePages parses the embedded templates the way the handlers do: the layout and
// the partials once, then one clone per page.
func parsePages(t *testing.T) (base *template.Template, pages map[string]*template.Template) {
	t.Helper()
	base, err := template.ParseFS(Templates(), "layout.html", "partials/*.html")
	if err != nil {
		t.Fatalf("parse layout and partials: %v", err)
	}
	pages = map[string]*template.Template{}
	for _, name := range []string{"catalog.html", "login.html"} {
		clone, err := base.Clone()
		if err != nil {
			t.Fatal(err)
		}
		if pages[name], err = clone.ParseFS(Templates(), name); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}
	return base, pages
}

func TestTemplatesEmbedTheLayoutPagesAndPartials(t *testing.T) {
	base, pages := parsePages(t)
	for _, name := range []string{"layout.html", "album_row", "album_rows"} {
		if base.Lookup(name) == nil {
			t.Errorf("the base set must define %q", name)
		}
	}
	for name, page := range pages {
		for _, block := range []string{"title", "content"} {
			if page.Lookup(block) == nil {
				t.Errorf("%s must define the %q block", name, block)
			}
		}
	}
}

func TestCatalogPageRendersRowsAndEscapes(t *testing.T) {
	_, pages := parsePages(t)
	year := 1959
	data := pageData{
		Title:     "Catalogue",
		CanDelete: true,
		Query:     `a "quoted" <query>`,
		Albums: []albumRow{
			{ID: 1, Title: "Kind of Blue", Artists: "Miles Davis", Genre: "jazz", ReleaseYear: &year, Label: "Columbia", Available: 2, Total: 3, CanDelete: true},
			{ID: 2, Title: "<b>Bold</b>", Artists: "Radiohead", Genre: "rock", Label: "Parlophone", CanDelete: true},
		},
	}
	var out bytes.Buffer
	if err := pages["catalog.html"].ExecuteTemplate(&out, "layout.html", data); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{
		"<!doctype html>", `href="/static/app.css"`, `src="/static/htmx.min.js"`, `href="/login"`,
		`id="album-list"`, `name="q"`, `hx-get="/web/albums"`, `hx-trigger="keyup changed delay:300ms"`, `hx-target="#album-list"`,
		`id="album-1"`, "Kind of Blue", "Miles Davis", "1959", "2/3", "0/0",
		`hx-delete="/web/albums/1"`, `hx-target="closest tr"`, `hx-swap="outerHTML"`, `hx-confirm="Delete this album?"`,
		"&lt;b&gt;Bold&lt;/b&gt;", `value="a &#34;quoted&#34; &lt;query&gt;"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("catalog page: missing %q", want)
		}
	}
	if strings.Contains(body, "<b>Bold</b>") {
		t.Error("catalog page: the title must be escaped")
	}

	out.Reset()
	data.CanDelete = false
	data.Albums = nil
	if err := pages["catalog.html"].ExecuteTemplate(&out, "layout.html", data); err != nil {
		t.Fatal(err)
	}
	if body := out.String(); !strings.Contains(body, `class="empty"`) || strings.Contains(body, "hx-delete") {
		t.Errorf("empty catalogue for a visitor: expected one empty row and no delete button, got %s", body)
	}
}

func TestRowsPartialIsTableRowsOnly(t *testing.T) {
	base, _ := parsePages(t)
	var out bytes.Buffer
	if err := base.ExecuteTemplate(&out, "album_rows", pageData{Albums: []albumRow{{ID: 7, Title: "Kid A"}}}); err != nil {
		t.Fatal(err)
	}
	body := strings.TrimSpace(out.String())
	if !strings.HasPrefix(body, "<tr") || !strings.HasSuffix(body, "</tr>") || strings.Contains(body, "<table") {
		t.Errorf("album_rows must render table rows only, got %q", body)
	}
}

func TestLoginPageShowsTheErrorAndNeverTheEmail(t *testing.T) {
	_, pages := parsePages(t)
	var out bytes.Buffer
	if err := pages["login.html"].ExecuteTemplate(&out, "layout.html", pageData{Title: "Log in", Error: "invalid credentials"}); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{`action="/login"`, `method="post"`, `name="email"`, `name="password"`, "invalid credentials"} {
		if !strings.Contains(body, want) {
			t.Errorf("login page: missing %q", want)
		}
	}
	if strings.Contains(body, `value="`) {
		t.Error("login page: inputs must not echo submitted values")
	}
}

func TestStaticEmbedsTheStylesheetAndHtmx(t *testing.T) {
	for name, marker := range map[string]string{"app.css": ".topbar", "htmx.min.js": "htmx"} {
		data, err := fs.ReadFile(Static(), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(string(data), marker) {
			t.Errorf("%s: expected to contain %q", name, marker)
		}
	}
	if _, err := fs.ReadFile(Static(), "static/app.css"); err == nil {
		t.Error("the static/ prefix must be stripped by Static()")
	}
}
