// Package middleware holds logging, panic recovery, CORS, bearer-token authentication
// (Auth) and the role check (RequireRole). Every middleware has the shape
// func(http.Handler) http.Handler and wraps the next handler explicitly.
package middleware
