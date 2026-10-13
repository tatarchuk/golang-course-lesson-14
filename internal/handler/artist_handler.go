package handler

import (
	"net/http"

	"homework/internal/model"
	"homework/internal/repository"
)

// ArtistHandler serves /api/v1/artists.
type ArtistHandler struct {
	artists repository.ArtistRepository
}

func NewArtistHandler(artists repository.ArtistRepository) *ArtistHandler {
	return &ArtistHandler{artists: artists}
}

// Create handles POST /api/v1/artists.
//
// @Summary  Create an artist
// @Tags     artists
// @Accept   json
// @Produce  json
// @Param    artist  body      model.ArtistInput  true  "Artist to create"
// @Success  201     {object}  model.Artist
// @Failure  422     {object}  ErrorResponse
// @Failure  401     {object}  ErrorResponse  "No valid bearer token"
// @Failure  403     {object}  ErrorResponse  "The role may not write to the catalogue"
// @Security BearerAuth
// @Router   /artists [post]
func (h *ArtistHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in model.ArtistInput
	if !readInput(w, r, &in) {
		return
	}
	artist, err := h.artists.Create(r.Context(), in)
	if err != nil {
		writeRepoError(w, err, "Artist")
		return
	}
	WriteJSON(w, http.StatusCreated, artist)
}

// Get handles GET /api/v1/artists/{id}.
//
// @Summary  Get an artist by id
// @Tags     artists
// @Produce  json
// @Param    id   path      int  true  "Artist id"
// @Success  200  {object}  model.Artist
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /artists/{id} [get]
func (h *ArtistHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	artist, err := h.artists.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err, "Artist")
		return
	}
	WriteJSON(w, http.StatusOK, artist)
}

// List handles GET /api/v1/artists.
//
// @Summary  List artists
// @Tags     artists
// @Produce  json
// @Success  200  {array}  model.Artist
// @Router   /artists [get]
func (h *ArtistHandler) List(w http.ResponseWriter, r *http.Request) {
	artists, err := h.artists.List(r.Context())
	if err != nil {
		writeRepoError(w, err, "Artist")
		return
	}
	WriteJSON(w, http.StatusOK, artists)
}
