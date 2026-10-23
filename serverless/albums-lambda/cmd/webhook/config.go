package main

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Config is the function's configuration, read from the environment once per Lambda sandbox.
type Config struct {
	AllowedGenres  []string // sorted, lower-case, unique
	MinReleaseYear int
}

// The default genres are the ones the albums database seeds (migrations/000002_seed_demo_data).
var defaultGenres = []string{"electronic", "jazz", "rock"}

const defaultMinReleaseYear = 1900

// loadConfig reads ALLOWED_GENRES (a comma-separated list) and MIN_RELEASE_YEAR; an unset variable
// keeps its default.
func loadConfig(getenv func(string) string) (Config, error) {
	c := Config{AllowedGenres: slices.Clone(defaultGenres), MinReleaseYear: defaultMinReleaseYear}
	if raw := getenv("ALLOWED_GENRES"); raw != "" {
		var genres []string
		for _, g := range strings.Split(raw, ",") {
			if g = strings.ToLower(strings.TrimSpace(g)); g != "" {
				genres = append(genres, g)
			}
		}
		if len(genres) == 0 {
			return Config{}, errors.New("ALLOWED_GENRES has no genres")
		}
		slices.Sort(genres)
		c.AllowedGenres = slices.Compact(genres)
	}
	if raw := getenv("MIN_RELEASE_YEAR"); raw != "" {
		year, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return Config{}, fmt.Errorf("MIN_RELEASE_YEAR must be an integer: %w", err)
		}
		c.MinReleaseYear = year
	}
	return c, nil
}

// mustLoadConfig turns a bad configuration into an init-phase failure: Lambda reports it with the
// message in the logs instead of serving requests with a guessed configuration.
func mustLoadConfig(getenv func(string) string) Config {
	c, err := loadConfig(getenv)
	if err != nil {
		panic(err)
	}
	return c
}
