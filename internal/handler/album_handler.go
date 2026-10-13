package handler

import (
	"net/http"

	"homework/internal/model"
	"homework/internal/repository"
)

const maxLimit = 100

// AlbumHandler serves /api/v1/albums. It depends on the repository interface only.
type AlbumHandler struct {
	albums repository.AlbumRepository
}

func NewAlbumHandler(albums repository.AlbumRepository) *AlbumHandler {
	return &AlbumHandler{albums: albums}
}

// Create handles POST /api/v1/albums.
//
// @Summary  Create an album
// @Tags     albums
// @Accept   json
// @Produce  json
// @Param    album  body      model.AlbumInput  true  "Album to create; genre_id and artist_ids must reference existing rows"
// @Success  201    {object}  model.Album
// @Failure  409    {object}  ErrorResponse  "An album with the same title and label exists"
// @Failure  422    {object}  ErrorResponse  "validation_error or invalid_reference"
// @Failure  401    {object}  ErrorResponse  "No valid bearer token"
// @Failure  403    {object}  ErrorResponse  "The role may not write to the catalogue"
// @Security BearerAuth
// @Router   /albums [post]
func (h *AlbumHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in model.AlbumInput
	if !readInput(w, r, &in) {
		return
	}
	album, err := h.albums.Create(r.Context(), in)
	if err != nil {
		writeRepoError(w, err, "Album")
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
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	album, err := h.albums.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err, "Album")
		return
	}
	WriteJSON(w, http.StatusOK, album)
}

// List handles GET /api/v1/albums?page=&limit=&genre_id=.
//
// @Summary  List albums
// @Tags     albums
// @Produce  json
// @Param    page      query     int  false  "Page number"               minimum(1)  default(1)
// @Param    limit     query     int  false  "Items per page (max 100)"  minimum(1)  default(10)
// @Param    genre_id  query     int  false  "Only albums of this genre"
// @Success  200       {array}   model.Album
// @Failure  400       {object}  ErrorResponse
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
	genreID, ok := parseOptionalID(q, "genre_id")
	if !ok {
		WriteError(w, http.StatusBadRequest, CodeValidation, "genre_id must be a positive integer")
		return
	}
	albums, err := h.albums.List(r.Context(), repository.ListOptions{GenreID: genreID, Page: page, Limit: limit})
	if err != nil {
		writeRepoError(w, err, "Album")
		return
	}
	WriteJSON(w, http.StatusOK, albums)
}

// Update handles PUT /api/v1/albums/{id}. The check order is part of the contract:
// id (400) -> body (422) -> existence (404). The repository is touched only after the
// body is valid, so a rejected request changes nothing.
//
// @Summary  Replace an album (full update)
// @Tags     albums
// @Accept   json
// @Produce  json
// @Param    id     path      int               true  "Album id (0..4294967295)"
// @Param    album  body      model.AlbumInput  true  "Full replacement; omitted optional fields become null, artist_ids replace the previous artists"
// @Success  200    {object}  model.Album
// @Failure  400    {object}  ErrorResponse
// @Failure  404    {object}  ErrorResponse
// @Failure  409    {object}  ErrorResponse
// @Failure  422    {object}  ErrorResponse
// @Failure  401    {object}  ErrorResponse  "No valid bearer token"
// @Failure  403    {object}  ErrorResponse  "The role may not write to the catalogue"
// @Security BearerAuth
// @Router   /albums/{id} [put]
func (h *AlbumHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in model.AlbumInput
	if !readInput(w, r, &in) {
		return
	}
	album, err := h.albums.Update(r.Context(), id, in)
	if err != nil {
		writeRepoError(w, err, "Album")
		return
	}
	WriteJSON(w, http.StatusOK, album)
}

// Delete handles DELETE /api/v1/albums/{id}. The database cascades to copies and artist links.
//
// @Summary  Delete an album
// @Tags     albums
// @Param    id   path  int  true  "Album id (0..4294967295)"
// @Success  204  "No Content"
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Failure  401  {object}  ErrorResponse  "No valid bearer token"
// @Failure  403  {object}  ErrorResponse  "The role may not write to the catalogue"
// @Security BearerAuth
// @Router   /albums/{id} [delete]
func (h *AlbumHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.albums.Delete(r.Context(), id); err != nil {
		writeRepoError(w, err, "Album")
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204: no body, no Content-Type
}
