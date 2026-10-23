package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// The events/ fixtures are the input of sam local invoke and aws lambda invoke, and they double as
// regression tests (slide 20). They go through handler, the entry point Lambda calls.
func TestEventFixtures(t *testing.T) {
	tests := []struct {
		file   string
		status int
	}{
		{"valid.json", 200},
		{"invalid.json", 400},
		{"missing_field.json", 400},
		{"unsupported_event.json", 400},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "events", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			var req events.APIGatewayProxyRequest
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatalf("not an API Gateway proxy event: %v", err)
			}
			resp, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned a Go error: %v", err)
			}
			if resp.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d (%s)", resp.StatusCode, tt.status, resp.Body)
			}
			if tt.file != "valid.json" {
				return
			}
			var body struct {
				Album struct {
					Genre string `json:"genre"`
				} `json:"album"`
			}
			if err := json.Unmarshal([]byte(resp.Body), &body); err != nil || body.Album.Genre != "jazz" {
				t.Fatalf("album genre = %q (%v), want jazz", body.Album.Genre, err)
			}
		})
	}
}
