package main

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Album is both the submitted data and the normalized result. The optional fields are pointers
// without omitempty, so an absent value is encoded as null and never dropped, as in the albums API.
type Album struct {
	Title       string   `json:"title"`
	Label       string   `json:"label"`
	Artists     []string `json:"artists"`
	Genre       string   `json:"genre"`
	ReleaseYear *int     `json:"release_year"`
	Notes       *string  `json:"notes"`
}

// normalize cleans up a submission and validates it against cfg. It reports every problem at once
// rather than the first one, so a client can fix the whole request in one go.
func normalize(in Album, cfg Config, now time.Time) (Album, []string) {
	var problems []string
	out := Album{
		Title:       collapse(in.Title),
		Label:       collapse(in.Label),
		Genre:       strings.ToLower(strings.TrimSpace(in.Genre)),
		ReleaseYear: in.ReleaseYear,
	}
	if out.Title == "" {
		problems = append(problems, "title is required")
	}
	if out.Label == "" {
		problems = append(problems, "label is required")
	}

	artists, artistsProblem := normalizeArtists(in.Artists)
	out.Artists = artists
	if artistsProblem != "" {
		problems = append(problems, artistsProblem)
	}

	switch {
	case out.Genre == "":
		problems = append(problems, "genre is required")
	case !slices.Contains(cfg.AllowedGenres, out.Genre):
		problems = append(problems, "genre must be one of: "+strings.Join(cfg.AllowedGenres, ", "))
	}

	// A release may be announced for next year, not later.
	maxYear := now.Year() + 1
	if y := in.ReleaseYear; y != nil && (*y < cfg.MinReleaseYear || *y > maxYear) {
		problems = append(problems, fmt.Sprintf("release_year must be between %d and %d", cfg.MinReleaseYear, maxYear))
	}

	if in.Notes != nil {
		if notes := strings.TrimSpace(*in.Notes); notes != "" {
			out.Notes = &notes
		}
	}
	return out, problems
}

// normalizeArtists collapses every name and drops repeats that differ only in case or spacing,
// keeping the first spelling. A blank name is a problem, reported once.
func normalizeArtists(names []string) ([]string, string) {
	if len(names) == 0 {
		return []string{}, "artists must contain at least one name"
	}
	out := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	problem := ""
	for _, name := range names {
		name = collapse(name)
		if name == "" {
			problem = "artists must not contain blank names"
			continue
		}
		if key := strings.ToLower(name); !seen[key] {
			seen[key] = true
			out = append(out, name)
		}
	}
	return out, problem
}

// collapse trims s and turns every run of white space (tabs, newlines and non-breaking spaces
// included) into a single space.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
