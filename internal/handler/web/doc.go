// Package web serves the server-rendered UI of Lesson 10: the catalogue page with its
// htmx live search and row deletion, the login form that stores the JWT in an HttpOnly
// cookie, and the embedded static files. The templates come from the root package web
// (embedded into the binary) and the data from the same repository interfaces the API
// handlers use, so tests run the handlers on fakes.
package web
