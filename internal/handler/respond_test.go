package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteErrorProducesTheContractEnvelope(t *testing.T) {
	w := httptest.NewRecorder()

	WriteError(w, http.StatusNotFound, CodeNotFound, "Album not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: want application/json, got %q", ct)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, w.Body)
	}
	if body.Error.Code != CodeNotFound || body.Error.Message != "Album not found" {
		t.Errorf("unexpected body: %s", w.Body)
	}
}

func TestWriteJSONEncodesAnEmptySliceAsArray(t *testing.T) {
	w := httptest.NewRecorder()

	WriteJSON(w, http.StatusOK, []int{})

	if got := strings.TrimSpace(w.Body.String()); got != "[]" {
		t.Errorf("want [], got %q", got)
	}
}
