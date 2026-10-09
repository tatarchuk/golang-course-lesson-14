package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestArtistInputValidateRequiresName(t *testing.T) {
	err := ArtistInput{Name: " "}.Validate()
	if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected ErrValidation naming name, got %v", err)
	}
	if err := (ArtistInput{Name: "Miles Davis"}).Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestArtistBioMarshalsAsNullWhenUnset(t *testing.T) {
	b, err := json.Marshal(Artist{ID: 1, Name: "Daft Punk"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"bio":null`) {
		t.Fatalf("expected bio:null in %s", b)
	}
}
