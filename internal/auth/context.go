package auth

import "context"

// ctxKey is unexported, so no other package can collide with or forge the stored claims.
type ctxKey struct{}

// WithUser stores the verified claims in the request context, the net/http analogue of
// c.Set in a Gin handler.
func WithUser(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// UserFromContext returns the claims the Auth middleware stored, or false for an
// anonymous request.
func UserFromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}
