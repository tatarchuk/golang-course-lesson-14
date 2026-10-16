package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFS embed.FS

// Static returns the static assets with the "static/" prefix stripped, so a request for
// /static/app.css maps to app.css once the route prefix is removed. The directory is part
// of the binary, so a failure here is a programming error.
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("web: static: " + err.Error())
	}
	return sub
}
