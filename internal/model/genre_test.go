package model

import (
	"errors"
	"strings"
	"testing"
)

func TestGenreInputValidateRequiresName(t *testing.T) {
	for _, name := range []string{"", "   "} {
		err := GenreInput{Name: name}.Validate()
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "name") {
			t.Errorf("name=%q: expected ErrValidation naming name, got %v", name, err)
		}
	}
	if err := (GenreInput{Name: "jazz"}).Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
