package main

import (
	"strings"
	"testing"
)

// TestValidateSecret pins security_audit.md finding 4: an empty, short or template secret
// must stop the server before anything is signed with it.
func TestValidateSecret(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"short": false,
		"change-me-openssl-rand-base64-48-before-first-run": false, // the .env.example placeholder
		strings.Repeat("x", 31):                             false,
		"lesson9-test-secret-do-not-use-in-production":      true,
		"Q2hhbmdlTWVQbGVhc2VOb3dSaWdodEF3YXlUaGFua1lvdQ==":  true,
	}
	for secret, ok := range cases {
		if err := validateSecret(secret); (err == nil) != ok {
			t.Errorf("validateSecret(%q): err=%v, want ok=%v", secret, err, ok)
		}
	}
}
