package web

import (
	"io/fs"
	"log"
	"net/http"

	"homework/internal/handler"
	"homework/internal/repository"
)

// CookieName is the cookie that carries the web session's JWT: the same HS256 token the
// API issues, stored where scripts cannot read it (HttpOnly) instead of in a JSON body.
const CookieName = "albums_token"

// pageLimit is how many albums the catalogue page shows: the first matches by id. The
// search narrows the list; the UI has no paging.
const pageLimit = 50

// Handler serves the server-rendered UI: the catalogue page, the htmx fragments, the
// login form and the session cookie. Like the API handlers it depends on the repository
// interfaces only.
type Handler struct {
	albums       repository.AlbumRepository
	copies       repository.CopyRepository
	users        repository.UserRepository
	secret       string
	cookieSecure bool
	views        *views
}

// New parses the templates once and returns the handler. A template that does not parse
// is a build problem, so the caller is expected to stop the server on the error.
func New(repos repository.Repositories, templates fs.FS, secret string, cookieSecure bool) (*Handler, error) {
	v, err := parseViews(templates)
	if err != nil {
		return nil, err
	}
	return &Handler{
		albums:       repos.Albums,
		copies:       repos.Copies,
		users:        repos.Users,
		secret:       secret,
		cookieSecure: cookieSecure,
		views:        v,
	}, nil
}

// serverError logs the cause and answers the API's generic 500 envelope. The browser
// shows it as JSON, which is acceptable for a failure that should never happen.
func serverError(w http.ResponseWriter, err error) {
	log.Printf("web: %v", err)
	handler.WriteError(w, http.StatusInternalServerError, handler.CodeInternal, "internal server error")
}
