package handler

import (
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"testing"

	"homework/internal/model"
)

func TestParseID(t *testing.T) {
	cases := []struct {
		in     string
		wantID int64
		wantOK bool
	}{
		{"0", 0, true},
		{"1", 1, true},
		{"4294967295", 4294967295, true},
		{"4294967296", 0, false},
		{"abc", 0, false},
		{"-1", 0, false},
		{"1.5", 0, false},
		{"", 0, false},
		{" 1", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			id, ok := parseID(tc.in)
			if ok != tc.wantOK || id != tc.wantID {
				t.Errorf("parseID(%q) = %d, %v; want %d, %v", tc.in, id, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}

func TestParsePositiveInt(t *testing.T) {
	cases := []struct {
		name   string
		query  string
		want   int
		wantOK bool
	}{
		{"absent uses the default", "", 10, true},
		{"empty uses the default", "limit=", 10, true},
		{"plain number", "limit=5", 5, true},
		{"zero", "limit=0", 0, false},
		{"negative", "limit=-5", 0, false},
		{"text", "limit=abc", 0, false},
		{"fraction", "limit=1.5", 0, false},
		{"too large is clamped, not rejected", "limit=99999999999999999999", math.MaxInt, true},
		{"too large negative", "limit=-99999999999999999999", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := url.ParseQuery(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := parsePositiveInt(q, "limit", 10)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("parsePositiveInt(%q) = %d, %v; want %d, %v", tc.query, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestDecodeMessageNamesTheFieldWithoutLeakingGoTypes(t *testing.T) {
	var in model.AlbumInput
	err := json.Unmarshal([]byte(`{"release_year":"1999"}`), &in)
	if err == nil {
		t.Fatal("expected a type error")
	}
	msg := decodeMessage(err)
	if !strings.Contains(msg, `"release_year"`) {
		t.Errorf("message %q should name the field", msg)
	}
	for _, leak := range []string{"Go struct", "model.", "int"} {
		if strings.Contains(msg, leak) {
			t.Errorf("message %q leaks internals (%q)", msg, leak)
		}
	}
}

func TestDecodeMessageIsGenericForNonObjects(t *testing.T) {
	for _, body := range []string{`{not json`, ``, `[1,2,3]`, `"text"`} {
		var in model.AlbumInput
		err := json.Unmarshal([]byte(body), &in)
		if err == nil {
			t.Fatalf("Unmarshal(%q) should fail", body)
		}
		if got := decodeMessage(err); got != "request body must be a valid JSON object" {
			t.Errorf("Unmarshal(%q): unexpected message %q", body, got)
		}
	}
}
