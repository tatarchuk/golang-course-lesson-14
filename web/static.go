package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFS embed.FS

// Static returns the static assets with the "static/" prefix stripped, so a request for
// /static/app.css maps to app.css once the route prefix is removed.
func Static() fs.FS {
	return mustSub(staticFS, "static")
}
