package app_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Lesson 9 acceptance tests for authentication: registration, login, token checks
// and the CORS preflight. They run against the real router and database, like the
// rest of this package.
//
// Error codes on top of the Theme 8 contract:
//
//	401 unauthenticated  wrong credentials; no, malformed, expired or badly signed token
//	403 forbidden        authenticated, but the role is not allowed (rbac_test.go)

// testJWTSecret is exported as JWT_SECRET before the router is built. The server signs
// and verifies tokens with it, and the tests can mint tokens of their own: expired
// ones, ones signed with another secret, ones carrying any role.
const testJWTSecret = "lesson9-test-secret-do-not-use-in-production"

// testPassword is the password of every account the tests register.
const testPassword = "Correct-Horse-Battery-1"

// newAuthRouter sets JWT_SECRET for the test and builds the router on a clean database.
func newAuthRouter(t *testing.T) http.Handler {
	t.Helper()
	t.Setenv("JWT_SECRET", testJWTSecret)
	return newRouter(t)
}

func credentials(email, password string) string {
	return fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
}

// register creates an account through the API and returns the user it answers with.
func register(t *testing.T, h http.Handler, email string) map[string]any {
	t.Helper()
	user := expect(t, call(h, "POST", "/api/v1/auth/register", credentials(email, testPassword)), http.StatusCreated, "")
	if _, ok := user["id"].(float64); !ok {
		t.Fatalf("register must answer with the created user, got %v", user)
	}
	return user
}

// login returns the token the API issues for the account.
func login(t *testing.T, h http.Handler, email string) string {
	t.Helper()
	body := expect(t, call(h, "POST", "/api/v1/auth/login", credentials(email, testPassword)), http.StatusOK, "")
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatalf("login must answer with a token, got %v", body)
	}
	return token
}

func claimsFor(userID int64, role string, ttl time.Duration) map[string]any {
	return map[string]any{"user_id": userID, "role": role, "exp": time.Now().Add(ttl).Unix()}
}

// mintToken signs a token the way the server is expected to, with testJWTSecret.
func mintToken(userID int64, role string, ttl time.Duration) string {
	return signJWT(testJWTSecret, claimsFor(userID, role, ttl))
}

// signJWT builds a compact HS256 JWT: base64url segments without padding, exactly
// what github.com/golang-jwt/jwt produces and accepts.
func signJWT(secret string, claims map[string]any) string {
	signingInput := b64(`{"alg":"HS256","typ":"JWT"}`) + "." + b64(mustJSON(claims))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// noneToken is the classic "alg": "none" attack: an unsigned token the server must reject.
func noneToken(claims map[string]any) string {
	return b64(`{"alg":"none","typ":"JWT"}`) + "." + b64(mustJSON(claims)) + "."
}

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// verifyJWT checks that token is an HS256 JWT signed with secret and returns its claims.
func verifyJWT(t *testing.T, token, secret string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("a JWT has three segments, got %q", token)
	}
	if header := jsonSegment(t, parts[0]); header["alg"] != "HS256" {
		t.Fatalf("the token must be signed with HS256, got alg %v", header["alg"])
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[2], "="))
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		t.Fatalf("the token signature does not verify with JWT_SECRET")
	}
	return jsonSegment(t, parts[1])
}

func jsonSegment(t *testing.T, segment string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(segment, "="))
	if err != nil {
		t.Fatalf("segment %q is not base64url: %v", segment, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("segment %q is not a JSON object: %v", segment, err)
	}
	return obj
}

func errorMessage(obj map[string]any) string {
	e, _ := obj["error"].(map[string]any)
	msg, _ := e["message"].(string)
	return msg
}

// assertNoSecrets fails when a success response carries password material.
func assertNoSecrets(t *testing.T, r resp) {
	t.Helper()
	body := strings.ToLower(string(r.Body))
	for _, needle := range []string{"password", "hash", "$2a$", "$2b$", "$2y$"} {
		if strings.Contains(body, needle) {
			t.Errorf("response must not expose password material (found %q): %s", needle, r)
		}
	}
}

// TestAuthRegisterAndLogin is the table-driven contract of POST /auth/register and
// POST /auth/login. The cases run in order on one database: the account registered
// in the first case is the one the later cases log in with.
func TestAuthRegisterAndLogin(t *testing.T) {
	h := newAuthRouter(t)
	var userID float64 // set by the first case, checked against the token claims later

	tests := []struct {
		name   string
		path   string
		body   string
		status int
		code   string
		check  func(t *testing.T, r resp, obj map[string]any)
	}{
		{
			name:   "register creates a member and answers with the user, not the hash",
			path:   "/api/v1/auth/register",
			body:   credentials("reader@albums.local", testPassword),
			status: http.StatusCreated,
			check: func(t *testing.T, r resp, obj map[string]any) {
				if obj["email"] != "reader@albums.local" || obj["role"] != "member" {
					t.Errorf("registered user: %s", r)
				}
				userID, _ = obj["id"].(float64)
				if userID == 0 {
					t.Errorf("registered user must have an id: %s", r)
				}
				assertNoSecrets(t, r)
			},
		},
		{
			name:   "register rejects a duplicate email with 422",
			path:   "/api/v1/auth/register",
			body:   credentials("reader@albums.local", "Another-Pass-2"),
			status: http.StatusUnprocessableEntity,
			code:   "validation_error",
			check: func(t *testing.T, r resp, obj map[string]any) {
				if errorMessage(obj) != "email already registered" {
					t.Errorf("duplicate email: %s", r)
				}
			},
		},
		{
			name:   "register rejects a missing password",
			path:   "/api/v1/auth/register",
			body:   `{"email":"nopass@albums.local"}`,
			status: http.StatusUnprocessableEntity,
			code:   "validation_error",
		},
		{
			name:   "register rejects a blank email",
			path:   "/api/v1/auth/register",
			body:   credentials("   ", testPassword),
			status: http.StatusUnprocessableEntity,
			code:   "validation_error",
		},
		{
			name:   "register rejects a body that is not JSON",
			path:   "/api/v1/auth/register",
			body:   `{not json`,
			status: http.StatusUnprocessableEntity,
			code:   "validation_error",
		},
		{
			name:   "login issues an HS256 token with user_id, role and a 24h exp",
			path:   "/api/v1/auth/login",
			body:   credentials("reader@albums.local", testPassword),
			status: http.StatusOK,
			check: func(t *testing.T, r resp, obj map[string]any) {
				token, _ := obj["token"].(string)
				if token == "" {
					t.Fatalf("login must answer with a token: %s", r)
				}
				claims := verifyJWT(t, token, testJWTSecret)
				if claims["user_id"] != userID || claims["role"] != "member" {
					t.Errorf("claims must carry user_id %v and role member: %v", userID, claims)
				}
				exp, _ := claims["exp"].(float64)
				if ttl := time.Until(time.Unix(int64(exp), 0)); ttl < 23*time.Hour || ttl > 25*time.Hour {
					t.Errorf("exp must be about 24h ahead, got %s (claims %v)", ttl, claims)
				}
			},
		},
		{
			name:   "login with the wrong password is a generic 401",
			path:   "/api/v1/auth/login",
			body:   credentials("reader@albums.local", "wrong-password"),
			status: http.StatusUnauthorized,
			code:   "unauthenticated",
			check: func(t *testing.T, r resp, obj map[string]any) {
				if errorMessage(obj) != "invalid credentials" {
					t.Errorf("wrong password: %s", r)
				}
			},
		},
		{
			name:   "login with an unknown email is the same generic 401",
			path:   "/api/v1/auth/login",
			body:   credentials("nobody@albums.local", testPassword),
			status: http.StatusUnauthorized,
			code:   "unauthenticated",
			check: func(t *testing.T, r resp, obj map[string]any) {
				if errorMessage(obj) != "invalid credentials" {
					t.Errorf("unknown email: %s", r)
				}
			},
		},
		{
			name:   "login rejects a missing password as invalid input",
			path:   "/api/v1/auth/login",
			body:   `{"email":"reader@albums.local"}`,
			status: http.StatusUnprocessableEntity,
			code:   "validation_error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := call(h, "POST", tt.path, tt.body)
			obj := expect(t, r, tt.status, tt.code)
			if tt.check != nil {
				tt.check(t, r, obj)
			}
		})
	}
}

func TestAuthLoginFailuresAreIndistinguishable(t *testing.T) {
	h := newAuthRouter(t)
	register(t, h, "reader@albums.local")

	wrongPassword := call(h, "POST", "/api/v1/auth/login", credentials("reader@albums.local", "wrong-password"))
	unknownEmail := call(h, "POST", "/api/v1/auth/login", credentials("nobody@albums.local", "wrong-password"))
	expect(t, wrongPassword, http.StatusUnauthorized, "unauthenticated")
	expect(t, unknownEmail, http.StatusUnauthorized, "unauthenticated")
	if string(wrongPassword.Body) != string(unknownEmail.Body) {
		t.Errorf("both failures must answer with the same body, or the API can be used to enumerate accounts:\n%s\n%s", wrongPassword, unknownEmail)
	}
}

// TestAuthTokenValidation sends every kind of bad credential to a protected route.
func TestAuthTokenValidation(t *testing.T) {
	h := newAuthRouter(t)
	uid := id(register(t, h, "reader@albums.local"))
	issued := login(t, h, "reader@albums.local")

	// The issued token with its payload swapped for an admin one: the signature no longer matches.
	parts := strings.Split(issued, ".")
	tampered := parts[0] + "." + b64(mustJSON(claimsFor(uid, "admin", time.Hour))) + "." + parts[2]

	tests := []struct {
		name          string
		authorization string
		status        int
	}{
		{"no Authorization header", "", http.StatusUnauthorized},
		{"a scheme other than Bearer", "Basic cmVhZGVyOnNlY3JldA==", http.StatusUnauthorized},
		{"Bearer without a token", "Bearer ", http.StatusUnauthorized},
		{"a token that is not a JWT", "Bearer not.a.jwt", http.StatusUnauthorized},
		{"an expired token", "Bearer " + mintToken(uid, "member", -15*time.Minute), http.StatusUnauthorized},
		{"a token signed with another secret", "Bearer " + signJWT("another-secret", claimsFor(uid, "member", time.Hour)), http.StatusUnauthorized},
		{"a token whose payload was changed after signing", "Bearer " + tampered, http.StatusUnauthorized},
		{"a token with alg none", "Bearer " + noneToken(claimsFor(uid, "admin", time.Hour)), http.StatusUnauthorized},
		{"the token issued by login", "Bearer " + issued, http.StatusOK},
		{"a token minted with the shared secret", "Bearer " + mintToken(uid, "member", time.Hour), http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest("GET", "/api/v1/me", "")
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			r := serve(h, req)
			code := ""
			if tt.status == http.StatusUnauthorized {
				code = "unauthenticated"
			}
			obj := expect(t, r, tt.status, code)
			if tt.status == http.StatusOK {
				if obj["id"] != float64(uid) || obj["email"] != "reader@albums.local" || obj["role"] != "member" {
					t.Errorf("GET /me must answer with the token's account: %s", r)
				}
				assertNoSecrets(t, r)
			}
		})
	}
}

func TestAuthCORSAllowsAuthorizationHeader(t *testing.T) {
	h := newAuthRouter(t)
	req := newRequest("OPTIONS", "/api/v1/me", "")
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	r := serve(h, req)
	allowed := strings.ToLower(r.Header.Get("Access-Control-Allow-Headers"))
	if r.Code != http.StatusNoContent || !strings.Contains(allowed, "authorization") {
		t.Errorf("preflight must allow the Authorization header, got %d with %q", r.Code, allowed)
	}
}
