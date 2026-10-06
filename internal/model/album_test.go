package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func validInput() AlbumInput {
	return AlbumInput{Title: "Kind of Blue", Artist: "Miles Davis", Label: "Columbia", Genre: "jazz"}
}

func TestValidateAcceptsCompleteInput(t *testing.T) {
	if err := validInput().Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsMissingOrBlankRequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		change func(in *AlbumInput)
		field  string // the field the message must name
	}{
		{"empty title", func(in *AlbumInput) { in.Title = "" }, "title"},
		{"blank artist", func(in *AlbumInput) { in.Artist = "   " }, "artist"},
		{"empty label", func(in *AlbumInput) { in.Label = "" }, "label"},
		{"tab genre", func(in *AlbumInput) { in.Genre = "\t" }, "genre"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validInput()
			tc.change(&in)

			err := in.Validate()

			if !errors.Is(err, ErrValidation) {
				t.Fatalf("want ErrValidation, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("message %q does not name field %q", err, tc.field)
			}
		})
	}
}

func TestValidateListsEveryMissingFieldInDeclarationOrder(t *testing.T) {
	err := AlbumInput{}.Validate()
	if err == nil {
		t.Fatal("expected an error for an empty input")
	}
	const want = "title, artist, label, genre"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("message %q should list %q", err, want)
	}
}

// Optional fields are pointers so that an unset field is encoded as null, never omitted.
func TestOptionalFieldsMarshalAsNullWhenUnset(t *testing.T) {
	b, err := json.Marshal(Album{ID: 1, Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"release_year":null`, `"notes":null`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON %s should contain %s", b, want)
		}
	}
}

func TestOptionalFieldsUnmarshalNullAsNil(t *testing.T) {
	var in AlbumInput
	if err := json.Unmarshal([]byte(`{"title":"t","release_year":null,"notes":"n"}`), &in); err != nil {
		t.Fatal(err)
	}
	if in.ReleaseYear != nil {
		t.Errorf("release_year: want nil, got %d", *in.ReleaseYear)
	}
	if in.Notes == nil || *in.Notes != "n" {
		t.Errorf("notes: want %q, got %v", "n", in.Notes)
	}
}

// Type errors are the decoder's job, not Validate's: these bodies must fail to decode.
func TestInputRejectsWrongJSONTypes(t *testing.T) {
	bodies := map[string]string{
		"string for int":   `{"release_year":"1999"}`,
		"fraction for int": `{"release_year":1999.5}`,
		"number for text":  `{"notes":42}`,
		"number for title": `{"title":1}`,
		"array body":       `[1,2,3]`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			var in AlbumInput
			if err := json.Unmarshal([]byte(body), &in); err == nil {
				t.Errorf("Unmarshal(%s) should fail", body)
			}
		})
	}
}
