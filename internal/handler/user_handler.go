package handler

import (
	"net/http"

	"homework/internal/model"
	"homework/internal/repository"
)

// UserHandler serves the administrator's account routes under /api/v1/users.
type UserHandler struct {
	users repository.UserRepository
}

func NewUserHandler(users repository.UserRepository) *UserHandler {
	return &UserHandler{users: users}
}

// List handles GET /api/v1/users.
//
// @Summary   List every account (admin)
// @Tags      users
// @Produce   json
// @Security  BearerAuth
// @Success   200  {array}   model.User
// @Failure   401  {object}  ErrorResponse
// @Failure   403  {object}  ErrorResponse
// @Router    /users [get]
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		writeRepoError(w, err, "User")
		return
	}
	WriteJSON(w, http.StatusOK, users)
}

// UpdateRole handles PUT /api/v1/users/{id}/role. Checks run in the order id (400),
// body (422), existence (404).
//
// @Summary   Change the role of an account (admin)
// @Tags      users
// @Accept    json
// @Produce   json
// @Security  BearerAuth
// @Param     id    path      int              true  "User id"
// @Param     role  body      model.RoleInput  true  "New role: member, staff or admin"
// @Success   200   {object}  model.User
// @Failure   400   {object}  ErrorResponse
// @Failure   401   {object}  ErrorResponse
// @Failure   403   {object}  ErrorResponse
// @Failure   404   {object}  ErrorResponse
// @Failure   422   {object}  ErrorResponse
// @Router    /users/{id}/role [put]
func (h *UserHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in model.RoleInput
	if !readInput(w, r, &in) {
		return
	}
	user, err := h.users.UpdateRole(r.Context(), id, in.Role)
	if err != nil {
		writeRepoError(w, err, "User")
		return
	}
	WriteJSON(w, http.StatusOK, user)
}
