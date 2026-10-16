// Package web carries the files of the server-rendered UI, embedded into the binary at
// build time: the html/template files under templates/ and the static assets (the
// stylesheet and htmx) under static/. It exposes them as fs.FS values and holds no HTTP
// code; the handlers live in internal/handler/web.
package web
