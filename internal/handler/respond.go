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

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
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
	WriteJSON(w, status, errorResponse{Error: errorDetail{Code: code, Message: message}})
}
