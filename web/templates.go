package web

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

// Templates returns the template files with the "templates/" prefix stripped, so
// "layout.html" and "partials/album_row.html" are the names html/template.ParseFS sees.
func Templates() fs.FS {
	return mustSub(templatesFS, "templates")
}

// mustSub strips a directory prefix from an embedded filesystem. The directory is part
// of the binary, so a failure here is a programming error, not a runtime condition.
func mustSub(fsys embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("web: " + dir + ": " + err.Error())
	}
	return sub
}
