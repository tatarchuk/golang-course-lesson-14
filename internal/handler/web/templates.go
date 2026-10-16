package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
)

// Page names, which are also the template file names under web/templates.
const (
	pageCatalog = "catalog.html"
	pageLogin   = "login.html"
)

// views holds the parsed template sets. The base set (layout and partials) renders the
// fragments; every page is a clone of the base with its own "title" and "content"
// blocks parsed on top, so pages override the layout's blocks without clobbering each
// other. Everything is html/template with contextual autoescaping; nothing is ever marked
// as safe HTML, so user and database text stays text.
type views struct {
	base  *template.Template
	pages map[string]*template.Template
}

// parseViews parses the embedded templates once, at startup.
func parseViews(fsys fs.FS) (*views, error) {
	base, err := template.ParseFS(fsys, "layout.html", "partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("web templates: %w", err)
	}
	v := &views{base: base, pages: map[string]*template.Template{}}
	for _, name := range []string{pageCatalog, pageLogin} {
		clone, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("web templates: clone for %s: %w", name, err)
		}
		if v.pages[name], err = clone.ParseFS(fsys, name); err != nil {
			return nil, fmt.Errorf("web templates: %w", err)
		}
	}
	return v, nil
}

// renderPage writes a whole page with the given status.
func (v *views) renderPage(w http.ResponseWriter, status int, page string, data pageData) {
	v.render(w, status, v.pages[page], "layout.html", data)
}

// renderRows writes the table rows of the catalogue: what htmx swaps into #album-list.
func (v *views) renderRows(w http.ResponseWriter, data pageData) {
	v.render(w, http.StatusOK, v.base, "album_rows", data)
}

// render executes the template into a buffer first, so a template error becomes a clean
// 500 instead of half a page sent with a 200 status.
func (v *views) render(w http.ResponseWriter, status int, t *template.Template, name string, data pageData) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		serverError(w, fmt.Errorf("render %s: %w", name, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
