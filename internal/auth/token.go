package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenTTL is how long a login token stays valid.
const TokenTTL = 24 * time.Hour

// Claims is what a verified token says about the caller.
type Claims struct {
	UserID int64
	Role   string
}

// ErrInvalidToken covers every token the server rejects; the exact reason stays in the
// server, clients only learn that the token did not work.
var ErrInvalidToken = errors.New("invalid token")

// tokenClaims is the JWT payload: the two custom claims plus the registered exp and iat.
type tokenClaims struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// IssueToken signs an HS256 token for the user that expires after ttl.
func IssueToken(secret string, userID int64, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := tokenClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken verifies the signature (HS256 only, so an "alg": "none" token is rejected)
// and the expiry, which must be present, and returns the claims.
func ParseToken(secret, raw string) (Claims, error) {
	var claims tokenClaims
	keyFunc := func(*jwt.Token) (any, error) { return []byte(secret), nil }
	token, err := jwt.ParseWithClaims(raw, &claims, keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired())
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid || claims.UserID <= 0 || claims.Role == "" {
		return Claims{}, ErrInvalidToken
	}
	return Claims{UserID: claims.UserID, Role: claims.Role}, nil
}
