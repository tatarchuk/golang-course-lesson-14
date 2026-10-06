package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

// Error codes from the contract.
const (
	CodeInvalidID         = "invalid_id"
	CodeInvalidPagination = "invalid_pagination"
	CodeValidation        = "validation_error"
	CodeNotFound          = "not_found"
	CodeInternal          = "internal_server_error"
)

// ErrorResponse is the envelope of every error reply: {"error":{"code":"...","message":"..."}}.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries the machine-readable code and a human-readable message.
type ErrorDetail struct {
	Code    string `json:"code" example:"not_found"`
	Message string `json:"message" example:"Album not found"`
}

// WriteJSON sends v as JSON. Headers must be set BEFORE WriteHeader, or they are ignored.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

// WriteError sends the contract's envelope: {"error":{"code":"...","message":"..."}}.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Error: ErrorDetail{Code: code, Message: message}})
}
