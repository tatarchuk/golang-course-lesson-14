package web

import (
	"io/fs"
	"net/http"
	"strings"

	"homework/internal/handler"
)

// StaticFiles serves the embedded assets under /static/. It refuses directory listings
// and answers the API's JSON 404 for anything that is not a file, so the only successful
// responses are the assets themselves. Embedded files carry no modification time, so
// instead of validators the assets get a one-day Cache-Control: they only change with a
// new binary.
func StaticFiles(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))
	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" || strings.HasSuffix(name, "/") {
			handler.NotFound(w, r)
			return
		}
		if info, err := fs.Stat(fsys, name); err != nil || info.IsDir() {
			handler.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(w, r)
	}))
}
