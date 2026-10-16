package web

import (
	"errors"
	"net/http"

	"homework/internal/auth"
	"homework/internal/handler"
)

// maxFormBytes bounds the login form body: two short fields need far less, and the
// limit keeps an oversized body from reaching bcrypt (the API caps bodies at 1 MiB).
const maxFormBytes = 16 << 10

// LoginForm handles GET /login.
func (h *Handler) LoginForm(w http.ResponseWriter, r *http.Request) {
	who, err := h.visitor(r)
	if err != nil {
		serverError(w, err)
		return
	}
	h.views.renderPage(w, http.StatusOK, pageLogin, pageData{Title: "Log in", User: who.user, CanDelete: who.canDelete})
}

// Login handles POST /login, the HTML form. The check is the API's Authenticate, so an
// unknown e-mail, a wrong password and an account without a password all produce the
// same 401 page, byte for byte: the form never echoes the e-mail back. Success stores
// the token in the session cookie and sends the browser to the catalogue.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		h.views.renderPage(w, http.StatusBadRequest, pageLogin, pageData{Title: "Log in", Error: "the form could not be read"})
		return
	}
	user, err := handler.Authenticate(r.Context(), h.users, r.PostForm.Get("email"), r.PostForm.Get("password"))
	if errors.Is(err, handler.ErrInvalidCredentials) {
		h.views.renderPage(w, http.StatusUnauthorized, pageLogin, pageData{Title: "Log in", Error: "invalid credentials"})
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	token, err := auth.IssueToken(h.secret, user.ID, user.Role, auth.TokenTTL)
	if err != nil {
		serverError(w, err)
		return
	}
	http.SetCookie(w, h.sessionCookie(token, int(auth.TokenTTL.Seconds())))
	http.Redirect(w, r, "/catalog", http.StatusSeeOther)
}

// Logout handles POST /logout: it expires the cookie. The token itself stays valid until
// its exp, like every token of this stateless design; the browser simply no longer
// holds it.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, h.sessionCookie("", -1))
	http.Redirect(w, r, "/catalog", http.StatusSeeOther)
}

// sessionCookie builds the session cookie: HttpOnly keeps it away from scripts, Lax keeps
// it off cross-site POSTs and DELETEs, Secure (COOKIE_SECURE) restricts it to TLS, and
// the lifetime matches the token's.
func (h *Handler) sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cookieSecure,
	}
}
