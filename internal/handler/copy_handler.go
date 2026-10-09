package handler

import (
	"errors"
	"net/http"

	"homework/internal/model"
	"homework/internal/repository"
)

// CopyHandler serves the copies of one album under /api/v1/albums/{id}.
type CopyHandler struct {
	copies repository.CopyRepository
}

func NewCopyHandler(copies repository.CopyRepository) *CopyHandler {
	return &CopyHandler{copies: copies}
}

// Create handles POST /api/v1/albums/{id}/copies.
//
// @Summary  Add a physical copy to an album
// @Tags     copies
// @Accept   json
// @Produce  json
// @Param    id    path      int              true  "Album id"
// @Param    copy  body      model.CopyInput  true  "Copy to add; status defaults to available"
// @Success  201   {object}  model.Copy
// @Failure  400   {object}  ErrorResponse
// @Failure  404   {object}  ErrorResponse  "Album not found"
// @Failure  409   {object}  ErrorResponse  "Inventory number already used"
// @Failure  422   {object}  ErrorResponse
// @Router   /albums/{id}/copies [post]
func (h *CopyHandler) Create(w http.ResponseWriter, r *http.Request) {
	albumID, ok := pathID(w, r)
	if !ok {
		return
	}
	var in model.CopyInput
	if !readInput(w, r, &in) {
		return
	}
	copyRow, err := h.copies.Create(r.Context(), albumID, in)
	if err != nil {
		h.writeError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, copyRow)
}

// List handles GET /api/v1/albums/{id}/copies.
//
// @Summary  List the copies of an album
// @Tags     copies
// @Produce  json
// @Param    id   path      int  true  "Album id"
// @Success  200  {array}   model.Copy
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /albums/{id}/copies [get]
func (h *CopyHandler) List(w http.ResponseWriter, r *http.Request) {
	albumID, ok := pathID(w, r)
	if !ok {
		return
	}
	copies, err := h.copies.ListByAlbum(r.Context(), albumID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, copies)
}

// Availability handles GET /api/v1/albums/{id}/availability.
//
// @Summary  Count the copies of an album and how many are available
// @Tags     copies
// @Produce  json
// @Param    id   path      int  true  "Album id"
// @Success  200  {object}  model.Availability
// @Failure  400  {object}  ErrorResponse
// @Failure  404  {object}  ErrorResponse
// @Router   /albums/{id}/availability [get]
func (h *CopyHandler) Availability(w http.ResponseWriter, r *http.Request) {
	albumID, ok := pathID(w, r)
	if !ok {
		return
	}
	av, err := h.copies.Availability(r.Context(), albumID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, av)
}

// writeError: a missing row here is always the album (the repository checks it first);
// every other error concerns the copy itself.
func (h *CopyHandler) writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		WriteError(w, http.StatusNotFound, CodeNotFound, "Album not found")
		return
	}
	writeRepoError(w, err, "Copy")
}
