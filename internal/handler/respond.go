package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON sends v as JSON. Headers must be set BEFORE WriteHeader, or they are ignored.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}
