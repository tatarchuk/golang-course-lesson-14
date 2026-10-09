package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func validAlbumInput() AlbumInput {
	year := 1959
	notes := "Recorded in 1959"
	return AlbumInput{
		Title:       "Kind of Blue",
		Label:       "Columbia",
		GenreID:     1,
		ArtistIDs:   []int64{1, 2},
		ReleaseYear: &year,
		Notes:       &notes,
	}
}

func TestAlbumInputValidateAcceptsCompleteInput(t *testing.T) {
	if err := validAlbumInput().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAlbumInputValidateAcceptsOmittedOptionalFields(t *testing.T) {
	in := validAlbumInput()
	in.ReleaseYear, in.Notes = nil, nil
	if err := in.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAlbumInputValidateRejectsMissingOrBlankRequiredFields(t *testing.T) {
	in := validAlbumInput()
	in.Title, in.Label = "", "   "
	err := in.Validate()
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	for _, field := range []string{"title", "label"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error %q should name %q", err, field)
		}
	}
}

func TestAlbumInputValidateRejectsNonPositiveGenreID(t *testing.T) {
	for _, id := range []int64{0, -1} {
		in := validAlbumInput()
		in.GenreID = id
		err := in.Validate()
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "genre_id") {
			t.Errorf("genre_id=%d: expected ErrValidation naming genre_id, got %v", id, err)
		}
	}
}

func TestAlbumInputValidateRejectsEmptyOrInvalidArtistIDs(t *testing.T) {
	cases := map[string][]int64{
		"nil":       nil,
		"empty":     {},
		"zero":      {0},
		"negative":  {1, -2},
		"duplicate": {1, 1},
	}
	for name, ids := range cases {
		in := validAlbumInput()
		in.ArtistIDs = ids
		err := in.Validate()
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "artist_ids") {
			t.Errorf("%s: expected ErrValidation naming artist_ids, got %v", name, err)
		}
	}
}

func TestAlbumMarshalsOptionalFieldsAsNullAndRelationsAsObjects(t *testing.T) {
	a := Album{
		ID:        1,
		Title:     "Kind of Blue",
		Label:     "Columbia",
		GenreID:   1,
		Genre:     Genre{ID: 1, Name: "jazz"},
		Artists:   []Artist{{ID: 1, Name: "Miles Davis"}},
		CreatedAt: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"release_year":null`, `"notes":null`, `"genre_id":1`,
		`"genre":{"id":1,"name":"jazz"`, `"artists":[{"id":1,"name":"Miles Davis","bio":null`,
		`"created_at":"2026-10-09T12:00:00Z"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON %s should contain %s", s, want)
		}
	}
}

func TestAlbumInputRejectsWrongJSONTypes(t *testing.T) {
	bodies := map[string]string{
		"release_year string": `{"title":"T","label":"L","genre_id":1,"artist_ids":[1],"release_year":"1999"}`,
		"release_year float":  `{"title":"T","label":"L","genre_id":1,"artist_ids":[1],"release_year":1999.5}`,
		"notes number":        `{"title":"T","label":"L","genre_id":1,"artist_ids":[1],"notes":42}`,
		"genre_id string":     `{"title":"T","label":"L","genre_id":"1","artist_ids":[1]}`,
		"artist_ids string":   `{"title":"T","label":"L","genre_id":1,"artist_ids":"1,2"}`,
		"title number":        `{"title":7,"label":"L","genre_id":1,"artist_ids":[1]}`,
	}
	for name, body := range bodies {
		var in AlbumInput
		if err := json.Unmarshal([]byte(body), &in); err == nil {
			t.Errorf("%s: expected a decoding error", name)
		}
	}
}

func TestAlbumInputUnmarshalsNullOptionalsAsNil(t *testing.T) {
	var in AlbumInput
	body := `{"title":"T","label":"L","genre_id":1,"artist_ids":[1],"release_year":null,"notes":null}`
	if err := json.Unmarshal([]byte(body), &in); err != nil {
		t.Fatal(err)
	}
	if in.ReleaseYear != nil || in.Notes != nil {
		t.Fatalf("expected nil optionals, got %v %v", in.ReleaseYear, in.Notes)
	}
}

func TestAlbumInputValidateRejectsReleaseYearOutOfRange(t *testing.T) {
	for _, year := range []int{-5, 10000, 99999999999} {
		in := validAlbumInput()
		in.ReleaseYear = &year
		err := in.Validate()
		if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "release_year") {
			t.Errorf("release_year=%d: expected ErrValidation naming release_year, got %v", year, err)
		}
	}
	for _, year := range []int{0, 1959, 9999} {
		in := validAlbumInput()
		in.ReleaseYear = &year
		if err := in.Validate(); err != nil {
			t.Errorf("release_year=%d: expected no error, got %v", year, err)
		}
	}
}
