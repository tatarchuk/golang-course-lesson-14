package handler

import (
	"errors"
	"log"
	"net/http"

	"homework/internal/model"
	"homework/internal/repository"
)

// Error codes of the API contract.
const (
	CodeInvalidID         = "invalid_id"
	CodeInvalidPagination = "invalid_pagination"
	CodeValidation        = "validation_error"
	CodeInvalidReference  = "invalid_reference"
	CodeConflict          = "conflict"
	CodeNotFound          = "not_found"
	CodeInternal          = "internal_server_error"
	CodeUnauthenticated   = "unauthenticated"
	CodeForbidden         = "forbidden"
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

// WriteError sends the contract's envelope with the given status.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Error: ErrorDetail{Code: code, Message: message}})
}

// writeRepoError maps a repository or validation error to the HTTP response.
// resource names the entity in messages ("Album", "Genre", ...). Unknown errors are
// logged in full and answered with a generic 500: internal details never reach the client.
func writeRepoError(w http.ResponseWriter, err error, resource string) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		WriteError(w, http.StatusNotFound, CodeNotFound, resource+" not found")
	case errors.Is(err, repository.ErrConflict):
		WriteError(w, http.StatusConflict, CodeConflict, resource+" already exists")
	case errors.Is(err, repository.ErrInvalidReference):
		WriteError(w, http.StatusUnprocessableEntity, CodeInvalidReference, err.Error())
	case errors.Is(err, repository.ErrInvalidInput):
		WriteError(w, http.StatusUnprocessableEntity, CodeValidation, resource+" has a value the database rejects")
	case errors.Is(err, model.ErrValidation):
		WriteError(w, http.StatusUnprocessableEntity, CodeValidation, err.Error())
	default:
		log.Printf("storage error: %v", err)
		WriteError(w, http.StatusInternalServerError, CodeInternal, "internal server error")
	}
}
