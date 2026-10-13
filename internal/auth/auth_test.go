package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("Correct-Horse-Battery-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$2a$10$") {
		t.Errorf("expected a cost-10 bcrypt hash, got %q", hash)
	}
	if !CheckPassword(hash, "Correct-Horse-Battery-1") {
		t.Error("the right password must match")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("a wrong password must not match")
	}
	if CheckPassword("", "anything") {
		t.Error("an account without a password must never match")
	}
	if CheckPassword(DummyHash, "") || CheckPassword(DummyHash, "anything") {
		t.Error("the dummy hash must never match")
	}
	if cost, err := bcrypt.Cost([]byte(DummyHash)); err != nil || cost != bcrypt.DefaultCost {
		t.Errorf("DummyHash must be a cost-%d bcrypt hash: cost %d, err %v", bcrypt.DefaultCost, cost, err)
	}
}

func TestIssueAndParseToken(t *testing.T) {
	token, err := IssueToken("s3cret", 42, "staff", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken("s3cret", token)
	if err != nil || claims != (Claims{UserID: 42, Role: "staff"}) {
		t.Fatalf("round trip: %+v %v", claims, err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("a compact JWT has three segments: %q", token)
	}
	if header := decodeSegment(t, parts[0]); header["alg"] != "HS256" {
		t.Errorf("alg must be HS256, got %v", header["alg"])
	}
	payload := decodeSegment(t, parts[1])
	if payload["user_id"] != float64(42) || payload["role"] != "staff" {
		t.Errorf("payload must carry user_id and role as plain JSON values: %v", payload)
	}
	exp, _ := payload["exp"].(float64)
	if ttl := time.Until(time.Unix(int64(exp), 0)); ttl < 59*time.Minute || ttl > 61*time.Minute {
		t.Errorf("exp must be about one hour ahead, got %s", ttl)
	}
}

func TestParseTokenRejectsBadTokens(t *testing.T) {
	good, _ := IssueToken("s3cret", 42, "staff", time.Hour)
	expired, _ := IssueToken("s3cret", 42, "staff", -time.Minute)
	other, _ := IssueToken("other", 42, "staff", time.Hour)
	parts := strings.Split(good, ".")
	seg := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	signed := func(claims jwt.MapClaims) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("s3cret"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	future := time.Now().Add(time.Hour).Unix()
	cases := map[string]string{
		"empty":        "",
		"garbage":      "not.a.jwt",
		"two segments": parts[0] + "." + parts[1],
		"expired":      expired,
		"other secret": other,
		"tampered":     parts[0] + "." + seg(`{"user_id":42,"role":"admin","exp":4102444800}`) + "." + parts[2],
		"alg none":     seg(`{"alg":"none","typ":"JWT"}`) + "." + parts[1] + ".",
		"no exp":       signed(jwt.MapClaims{"user_id": 42, "role": "staff"}),
		"user_id 0":    signed(jwt.MapClaims{"user_id": 0, "role": "staff", "exp": future}),
		"empty role":   signed(jwt.MapClaims{"user_id": 42, "role": "", "exp": future}),
	}
	for name, raw := range cases {
		if _, err := ParseToken("s3cret", raw); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: expected ErrInvalidToken, got %v", name, err)
		}
	}
}

// TestParseTokenAcceptsMintedTokens covers the shape the acceptance tests mint:
// user_id, role and exp only, no iat.
func TestParseTokenAcceptsMintedTokens(t *testing.T) {
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": 7, "role": "admin", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("s3cret"))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken("s3cret", raw)
	if err != nil || claims != (Claims{UserID: 7, Role: "admin"}) {
		t.Fatalf("minted token: %+v %v", claims, err)
	}
}

func TestUserFromContext(t *testing.T) {
	if _, ok := UserFromContext(context.Background()); ok {
		t.Error("an empty context has no user")
	}
	ctx := WithUser(context.Background(), Claims{UserID: 7, Role: "member"})
	if c, ok := UserFromContext(ctx); !ok || c != (Claims{UserID: 7, Role: "member"}) {
		t.Errorf("got %+v %v", c, ok)
	}
}

func decodeSegment(t *testing.T, segment string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		t.Fatalf("segment %q is not base64url: %v", segment, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("segment %q is not a JSON object: %v", segment, err)
	}
	return obj
}
