package web

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

// Templates returns the template files with the "templates/" prefix stripped, so
// "layout.html" and "partials/album_row.html" are the names html/template.ParseFS sees.
// The directory is part of the binary, so a failure here is a programming error.
func Templates() fs.FS {
	sub, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		panic("web: templates: " + err.Error())
	}
	return sub
}
