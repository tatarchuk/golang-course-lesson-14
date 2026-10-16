package handler

import (
	"errors"
	"net/http"

	"homework/internal/auth"
	"homework/internal/model"
	"homework/internal/repository"
)

// AuthHandler serves registration, login and the caller's own account.
type AuthHandler struct {
	users  repository.UserRepository
	secret string
}

func NewAuthHandler(users repository.UserRepository, secret string) *AuthHandler {
	return &AuthHandler{users: users, secret: secret}
}

// TokenResponse is the body of a successful login.
type TokenResponse struct {
	Token string `json:"token"`
}

// msgInvalidCredentials is the one message every login failure gets, so the endpoint
// cannot be used to tell registered e-mails from unknown ones.
const msgInvalidCredentials = "invalid credentials"

// Register handles POST /api/v1/auth/register.
//
// @Summary  Register an account (role member)
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    credentials  body      model.RegisterInput  true  "E-mail and password (8-72 bytes)"
// @Success  201          {object}  model.User
// @Failure  422          {object}  ErrorResponse  "Blank fields, bad JSON or an e-mail that is already registered"
// @Router   /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var in model.RegisterInput
	if !readInput(w, r, &in) {
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeRepoError(w, err, "User")
		return
	}
	user, err := h.users.Create(r.Context(), model.NormalizeEmail(in.Email), hash)
	if errors.Is(err, repository.ErrConflict) {
		WriteError(w, http.StatusUnprocessableEntity, CodeValidation, "email already registered")
		return
	}
	if err != nil {
		writeRepoError(w, err, "User")
		return
	}
	WriteJSON(w, http.StatusCreated, user)
}

// Login handles POST /api/v1/auth/login. Every failure takes the same path through
// Authenticate: the password is always compared with bcrypt (against DummyHash when
// the account is unknown or has no password yet) and the answer is the same 401 body,
// so neither the response nor its timing reveals which e-mails are registered. The web
// UI's login form (internal/handler/web) shares that check.
//
// @Summary  Log in and receive a 24-hour bearer token
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    credentials  body      model.LoginInput  true  "E-mail and password"
// @Success  200          {object}  TokenResponse
// @Failure  401          {object}  ErrorResponse  "invalid credentials"
// @Failure  422          {object}  ErrorResponse
// @Router   /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var in model.LoginInput
	if !readInput(w, r, &in) {
		return
	}
	user, err := Authenticate(r.Context(), h.users, in.Email, in.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		WriteError(w, http.StatusUnauthorized, CodeUnauthenticated, msgInvalidCredentials)
		return
	}
	if err != nil {
		writeRepoError(w, err, "User")
		return
	}
	token, err := auth.IssueToken(h.secret, user.ID, user.Role, auth.TokenTTL)
	if err != nil {
		writeRepoError(w, err, "User")
		return
	}
	WriteJSON(w, http.StatusOK, TokenResponse{Token: token})
}

// Me handles GET /api/v1/me.
//
// @Summary   The account behind the bearer token
// @Tags      auth
// @Produce   json
// @Security  BearerAuth
// @Success   200  {object}  model.User
// @Failure   401  {object}  ErrorResponse  "No valid token, or the account no longer exists"
// @Router    /me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
		return
	}
	user, err := h.users.Get(r.Context(), claims.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		// A valid token for a deleted account is no longer a credential.
		WriteError(w, http.StatusUnauthorized, CodeUnauthenticated, "the account behind this token no longer exists")
		return
	}
	if err != nil {
		writeRepoError(w, err, "User")
		return
	}
	WriteJSON(w, http.StatusOK, user)
}
