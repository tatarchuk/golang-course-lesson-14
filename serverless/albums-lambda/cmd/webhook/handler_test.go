package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// validBody is the body of events/valid.json.
const validBody = `{"event":"album.submitted","data":{"title":"  Kind   of Blue ","label":"Columbia",` +
	`"artists":["Miles Davis"," John  Coltrane ","miles davis"],"genre":"Jazz",` +
	`"release_year":1959,"notes":"  Recorded in 1959.  "}}`

// call runs handle with the test configuration and clock and decodes the JSON response body.
func call(t *testing.T, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, map[string]any) {
	t.Helper()
	resp, err := handle(context.Background(), req, testCfg, testNow)
	if err != nil {
		t.Fatalf("handle returned a Go error, which API Gateway turns into a 502: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatalf("response body is not JSON: %v (%q)", err, resp.Body)
	}
	return resp, body
}

// jsonObject decodes a literal the way the response body is decoded, so numbers compare as float64.
func jsonObject(t *testing.T, s string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad literal %q: %v", s, err)
	}
	return v
}

func TestValidSubmissionIsNormalized(t *testing.T) {
	resp, body := call(t, events.APIGatewayProxyRequest{Body: validBody})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, resp.Body)
	}
	if body["status"] != "processed" {
		t.Errorf("status field = %v, want processed", body["status"])
	}
	want := jsonObject(t, `{"title":"Kind of Blue","label":"Columbia","artists":["Miles Davis","John Coltrane"],`+
		`"genre":"jazz","release_year":1959,"notes":"Recorded in 1959."}`)
	if !reflect.DeepEqual(body["album"], want) {
		t.Errorf("album = %v\nwant    %v", body["album"], want)
	}
	if got := resp.Headers["Content-Type"]; got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := resp.Headers["X-Request-Id"]; got != "local" {
		t.Errorf("X-Request-Id = %q, want local (no Lambda context, no API Gateway request id)", got)
	}
}

// Optional fields are null, never omitted, as in the albums API.
func TestAbsentOptionalFieldsAreNull(t *testing.T) {
	_, body := call(t, events.APIGatewayProxyRequest{
		Body: `{"event":"album.submitted","data":{"title":"Kid A","label":"Parlophone","artists":["Radiohead"],"genre":"rock"}}`,
	})
	album, _ := body["album"].(map[string]any)
	for _, key := range []string{"release_year", "notes"} {
		v, present := album[key]
		if !present || v != nil {
			t.Errorf("album[%q] = %v (present %v), want null", key, v, present)
		}
	}
}

func TestBase64Body(t *testing.T) {
	resp, _ := call(t, events.APIGatewayProxyRequest{
		Body:            base64.StdEncoding.EncodeToString([]byte(validBody)),
		IsBase64Encoded: true,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, resp.Body)
	}
}

func TestClientErrorsAre400Not502(t *testing.T) {
	unknownGenre := strings.Replace(validBody, `"genre":"Jazz"`, `"genre":"polka"`, 1)
	tooOld := strings.Replace(validBody, `"release_year":1959`, `"release_year":1899`, 1)
	tests := []struct {
		name      string
		body      string
		base64    bool
		wantError string
		extraKey  string // a key that must accompany the error, with extraValue
		extraVal  string
		details   int // number of validation problems; 0 means not checked
	}{
		{name: "malformed json", body: `{"event":`, wantError: "invalid json"},
		{name: "empty body", body: ``, wantError: "request body is empty"},
		{name: "whitespace body", body: " \n\t ", wantError: "request body is empty"},
		{name: "bad base64", body: "%%%", base64: true, wantError: "body is not valid base64"},
		{name: "unsupported event", body: `{"event":"album.deleted","data":{}}`,
			wantError: "unsupported event", extraKey: "event", extraVal: "album.deleted"},
		{name: "data is a string", body: `{"event":"album.submitted","data":"oops"}`, wantError: "data must be a JSON object"},
		{name: "data is null", body: `{"event":"album.submitted","data":null}`, wantError: "data must be a JSON object"},
		{name: "data is an array", body: `{"event":"album.submitted","data":[]}`, wantError: "data must be a JSON object"},
		{name: "data is missing", body: `{"event":"album.submitted"}`, wantError: "data must be a JSON object"},
		{name: "release_year is a string", body: `{"event":"album.submitted","data":{"release_year":"1959"}}`,
			wantError: "invalid field type", extraKey: "field", extraVal: "release_year"},
		{name: "artists is a string", body: `{"event":"album.submitted","data":{"artists":"Miles Davis"}}`,
			wantError: "invalid field type", extraKey: "field", extraVal: "artists"},
		{name: "missing fields", body: `{"event":"album.submitted","data":{}}`, wantError: "validation failed", details: 4},
		{name: "unknown genre", body: unknownGenre, wantError: "validation failed", details: 1},
		{name: "release year too old", body: tooOld, wantError: "validation failed", details: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := call(t, events.APIGatewayProxyRequest{Body: tt.body, IsBase64Encoded: tt.base64})
			if resp.StatusCode != 400 {
				t.Fatalf("status = %d, want 400 (%s)", resp.StatusCode, resp.Body)
			}
			if body["error"] != tt.wantError {
				t.Errorf("error = %v, want %q", body["error"], tt.wantError)
			}
			if tt.extraKey != "" && body[tt.extraKey] != tt.extraVal {
				t.Errorf("%s = %v, want %q", tt.extraKey, body[tt.extraKey], tt.extraVal)
			}
			if tt.details > 0 {
				details, _ := body["details"].([]any)
				if len(details) != tt.details {
					t.Errorf("details = %v, want %d problems", body["details"], tt.details)
				}
			}
		})
	}
}
