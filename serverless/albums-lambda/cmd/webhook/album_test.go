package main

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

var (
	testNow = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	testCfg = Config{AllowedGenres: []string{"electronic", "jazz", "rock"}, MinReleaseYear: 1900}
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

// validAlbum is a submission without problems; tests change one field at a time.
func validAlbum() Album {
	return Album{Title: "Kind of Blue", Label: "Columbia", Artists: []string{"Miles Davis"}, Genre: "jazz"}
}

func TestNormalizeCleansUpSubmission(t *testing.T) {
	in := Album{
		Title:       "  Kind\u00a0of   Blue\t",
		Label:       " Columbia ",
		Artists:     []string{"Miles Davis", " John  Coltrane ", "miles  davis", "MILES DAVIS"},
		Genre:       " JAZZ ",
		ReleaseYear: intPtr(1959),
		Notes:       strPtr("  Recorded in 1959.  "),
	}
	got, problems := normalize(in, testCfg, testNow)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	want := Album{
		Title:       "Kind of Blue",
		Label:       "Columbia",
		Artists:     []string{"Miles Davis", "John Coltrane"},
		Genre:       "jazz",
		ReleaseYear: intPtr(1959),
		Notes:       strPtr("Recorded in 1959."),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestBlankNotesBecomeNull(t *testing.T) {
	in := validAlbum()
	in.Notes = strPtr("   ")
	got, problems := normalize(in, testCfg, testNow)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if got.Notes != nil {
		t.Fatalf("notes = %q, want nil", *got.Notes)
	}
}

func TestNormalizeReportsAllProblems(t *testing.T) {
	_, problems := normalize(Album{}, testCfg, testNow)
	want := []string{
		"title is required",
		"label is required",
		"artists must contain at least one name",
		"genre is required",
	}
	if !reflect.DeepEqual(problems, want) {
		t.Fatalf("problems = %q, want %q", problems, want)
	}
}

func TestBlankArtistNameIsAProblem(t *testing.T) {
	in := validAlbum()
	in.Artists = []string{"Miles Davis", "  "}
	_, problems := normalize(in, testCfg, testNow)
	if !slices.Contains(problems, "artists must not contain blank names") {
		t.Fatalf("problems = %q", problems)
	}
}

func TestUnknownGenre(t *testing.T) {
	in := validAlbum()
	in.Genre = "polka"
	_, problems := normalize(in, testCfg, testNow)
	if !slices.Contains(problems, "genre must be one of: electronic, jazz, rock") {
		t.Fatalf("problems = %q", problems)
	}
}

func TestReleaseYearBounds(t *testing.T) {
	const outside = "release_year must be between 1900 and 2027"
	tests := []struct {
		name string
		year *int
		ok   bool
	}{
		{"absent", nil, true},
		{"minimum", intPtr(1900), true},
		{"next year", intPtr(2027), true},
		{"before the minimum", intPtr(1899), false},
		{"two years ahead", intPtr(2028), false},
		{"negative", intPtr(-5), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validAlbum()
			in.ReleaseYear = tt.year
			_, problems := normalize(in, testCfg, testNow)
			if tt.ok && len(problems) != 0 {
				t.Fatalf("problems = %q, want none", problems)
			}
			if !tt.ok && !slices.Contains(problems, outside) {
				t.Fatalf("problems = %q, want %q", problems, outside)
			}
		})
	}
}
