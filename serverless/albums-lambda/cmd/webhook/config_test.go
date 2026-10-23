package main

import (
	"reflect"
	"testing"
)

// fakeEnv stands in for os.Getenv.
func fakeEnv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	got, err := loadConfig(fakeEnv(nil))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	want := Config{AllowedGenres: []string{"electronic", "jazz", "rock"}, MinReleaseYear: 1900}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	got, err := loadConfig(fakeEnv(map[string]string{
		"ALLOWED_GENRES":   " Jazz, blues ,,jazz",
		"MIN_RELEASE_YEAR": "1950",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	want := Config{AllowedGenres: []string{"blues", "jazz"}, MinReleaseYear: 1950}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLoadConfigRejectsBadValues(t *testing.T) {
	tests := map[string]map[string]string{
		"year is not a number":  {"MIN_RELEASE_YEAR": "abc"},
		"no genres in the list": {"ALLOWED_GENRES": " , "},
	}
	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := loadConfig(fakeEnv(env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestMustLoadConfigPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an invalid MIN_RELEASE_YEAR")
		}
	}()
	mustLoadConfig(fakeEnv(map[string]string{"MIN_RELEASE_YEAR": "abc"}))
}

// The configuration is read once, at package level, when a Lambda sandbox starts (slide 23). The
// test environment sets neither variable, so the package value holds the defaults.
func TestPackageLevelConfig(t *testing.T) {
	want := Config{AllowedGenres: []string{"electronic", "jazz", "rock"}, MinReleaseYear: 1900}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("package cfg = %+v, want %+v", cfg, want)
	}
}
