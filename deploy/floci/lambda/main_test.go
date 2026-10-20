package main

import (
	"context"
	"testing"
)

func TestHandlerDefaultsToWorld(t *testing.T) {
	got, err := handler(context.Background(), Event{})
	if err != nil {
		t.Fatalf("handler(Event{}) returned error: %v", err)
	}
	want := Response{StatusCode: 200, Body: "hello, world from floci"}
	if got != want {
		t.Fatalf("handler(Event{}) = %+v, want %+v", got, want)
	}
}

func TestHandlerUsesName(t *testing.T) {
	got, err := handler(context.Background(), Event{Name: "albums"})
	if err != nil {
		t.Fatalf("handler(Event{Name: albums}) returned error: %v", err)
	}
	if want := "hello, albums from floci"; got.Body != want {
		t.Fatalf("body = %q, want %q", got.Body, want)
	}
}
