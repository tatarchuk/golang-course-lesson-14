package handler

import (
	"errors"
	"log"
	"net/http"

	"homework/internal/model"
	"homework/internal/repository"
)

const msgInvalidID = "id must be an integer between 0 and 4294967295"

// AlbumHandler serves /api/v1/albums. It depends on the repository interface only.
type AlbumHandler struct {
	repo repository.AlbumRepository
}

func NewAlbumHandler(repo repository.AlbumRepository) *AlbumHandler {
	return &AlbumHandler{repo: repo}
}

// Create handles POST /api/v1/albums.
//
// @Summary  Create an album
// @Tags     albums
// @Accept   json
// @Produce  json
// @Param    album  body      model.AlbumInput  true  "Album to create"
// @Success  201    {object}  model.Album
// @Failure  422    {object}  ErrorResponse
// @Router   /albums [post]
func (h *AlbumHandler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := h.readInput(w, r)
	if !ok {
		return // readInput already wrote the 422
	}
	album, err := h.repo.Create(in)
	if err != nil {
		h.storageError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, album)
}

// Get handles GET /api/v1/albums/{id}.
//
// @Summary  Get an album by id
// @Tags     albums
// @Produce  json
// @Param    id   path      int  true  "Album id (0..4294967295)"
// @Success  200  {object}  model.Album
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /albums/{id} [get]
func (h *AlbumHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		WriteError(w, http.StatusBadRequest, CodeInvalidID, msgInvalidID)
		return
	}
	album, err := h.repo.Get(id)
	if err != nil {
		h.storageError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, album)
}

// Update handles PUT /api/v1/albums/{id}. The check order is part of the contract:
// id (400) -> body (422) -> existence (404). The repository is touched only after the body
// is valid, so a rejected request changes nothing.
//
// @Summary  Replace an album (full update)
// @Tags     albums
// @Accept   json
// @Produce  json
// @Param    id     path      int               true  "Album id (0..4294967295)"
// @Param    album  body      model.AlbumInput  true  "Full replacement; omitted optional fields become null"
// @Success  200    {object}  model.Album
// @Failure  400    {object}  ErrorResponse
// @Failure  404    {object}  ErrorResponse
// @Failure  422    {object}  ErrorResponse
// @Router   /albums/{id} [put]
func (h *AlbumHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		WriteError(w, http.StatusBadRequest, CodeInvalidID, msgInvalidID)
		return
	}
	in, ok := h.readInput(w, r)
	if !ok {
		return
	}
	album, err := h.repo.Update(id, in)
	if err != nil {
		h.storageError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, album)
}

// Delete handles DELETE /api/v1/albums/{id}.
//
// @Summary  Delete an album
// @Tags     albums
// @Param    id   path  int  true  "Album id (0..4294967295)"
// @Success  204  "No Content"
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /albums/{id} [delete]
func (h *AlbumHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		WriteError(w, http.StatusBadRequest, CodeInvalidID, msgInvalidID)
		return
	}
	if err := h.repo.Delete(id); err != nil {
		h.storageError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204: no body, no Content-Type
}

const maxLimit = 100

// List handles GET /api/v1/albums?page=&limit=&genre=.
//
// @Summary  List albums
// @Tags     albums
// @Produce  json
// @Param    page   query     int     false  "Page number"                 minimum(1)  default(1)
// @Param    limit  query     int     false  "Items per page (max 100)"    minimum(1)  default(10)
// @Param    genre  query     string  false  "Exact, case-sensitive genre filter"
// @Success  200    {array}   model.Album
// @Failure  400    {object}  ErrorResponse
// @Router   /albums [get]
func (h *AlbumHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, okPage := parsePositiveInt(q, "page", 1)
	limit, okLimit := parsePositiveInt(q, "limit", 10)
	if !okPage || !okLimit {
		WriteError(w, http.StatusBadRequest, CodeInvalidPagination, "page and limit must be integers >= 1")
		return
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	albums, err := h.repo.List(repository.ListOptions{Genre: q.Get("genre"), Page: page, Limit: limit})
	if err != nil {
		h.storageError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, albums)
}

// readInput decodes and validates the body. On failure it writes the 422 and returns false.
func (h *AlbumHandler) readInput(w http.ResponseWriter, r *http.Request) (model.AlbumInput, bool) {
	var in model.AlbumInput
	if err := decodeJSON(w, r, &in); err != nil {
		WriteError(w, http.StatusUnprocessableEntity, CodeValidation, decodeMessage(err))
		return in, false
	}
	if err := in.Validate(); err != nil {
		WriteError(w, http.StatusUnprocessableEntity, CodeValidation, err.Error())
		return in, false
	}
	return in, true
}

// storageError maps repository errors to HTTP responses.
func (h *AlbumHandler) storageError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		WriteError(w, http.StatusNotFound, CodeNotFound, "Album not found")
		return
	}
	log.Printf("storage error: %v", err) // details go to the log, never to the client
	WriteError(w, http.StatusInternalServerError, CodeInternal, "internal server error")
}
