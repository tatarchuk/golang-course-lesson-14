package handler

import (
	"net/http"

	"homework/internal/model"
	"homework/internal/repository"
)

// GenreHandler serves /api/v1/genres.
type GenreHandler struct {
	genres repository.GenreRepository
}

func NewGenreHandler(genres repository.GenreRepository) *GenreHandler {
	return &GenreHandler{genres: genres}
}

// Create handles POST /api/v1/genres.
//
// @Summary  Create a genre
// @Tags     genres
// @Accept   json
// @Produce  json
// @Param    genre  body      model.GenreInput  true  "Genre to create"
// @Success  201    {object}  model.Genre
// @Failure  409    {object}  ErrorResponse  "A genre with this name exists"
// @Failure  422    {object}  ErrorResponse
// @Router   /genres [post]
func (h *GenreHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in model.GenreInput
	if !readInput(w, r, &in) {
		return
	}
	genre, err := h.genres.Create(r.Context(), in)
	if err != nil {
		writeRepoError(w, err, "Genre")
		return
	}
	WriteJSON(w, http.StatusCreated, genre)
}

// Get handles GET /api/v1/genres/{id}.
//
// @Summary  Get a genre by id
// @Tags     genres
// @Produce  json
// @Param    id   path      int  true  "Genre id"
// @Success  200  {object}  model.Genre
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /genres/{id} [get]
func (h *GenreHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	genre, err := h.genres.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err, "Genre")
		return
	}
	WriteJSON(w, http.StatusOK, genre)
}

// List handles GET /api/v1/genres.
//
// @Summary  List genres
// @Tags     genres
// @Produce  json
// @Success  200  {array}  model.Genre
// @Router   /genres [get]
func (h *GenreHandler) List(w http.ResponseWriter, r *http.Request) {
	genres, err := h.genres.List(r.Context())
	if err != nil {
		writeRepoError(w, err, "Genre")
		return
	}
	WriteJSON(w, http.StatusOK, genres)
}
