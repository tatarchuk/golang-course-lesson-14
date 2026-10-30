package chat

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"homework/internal/auth"
	"homework/internal/handler"
	"homework/internal/model"
	"homework/internal/repository"
)

// UserGetter is the part of repository.UserRepository the chat needs.
type UserGetter interface {
	Get(ctx context.Context, id int64) (model.User, error)
}

type wsHandler struct {
	hub      *Hub
	users    UserGetter
	upgrader websocket.Upgrader
}

// NewHandler serves GET /ws/chat?room=general|staff. In app.go CookieAuth and RequireRole
// run first, so the browser's session cookie authenticates the handshake and no token ever
// appears in a URL; the claims are checked here again. Before the upgrade it answers with
// the API's JSON errors: 401 without a session or for an account that no longer exists,
// 404 for an unknown room, 403 for a room the role may not join or a foreign Origin, 503
// while the server shuts down. The author of the connection's messages is the account's
// e-mail. The connection settings are the hub's.
func NewHandler(h *Hub, users UserGetter) http.Handler {
	return &wsHandler{hub: h, users: users, upgrader: websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     sameOrigin,
	}}
}

func (s *wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		handler.WriteError(w, http.StatusUnauthorized, handler.CodeUnauthenticated, "authentication required")
		return
	}
	room := r.URL.Query().Get("room")
	if room == "" {
		room = RoomGeneral
	}
	if !knownRoom(room) {
		handler.WriteError(w, http.StatusNotFound, handler.CodeNotFound, "no such chat room")
		return
	}
	if !CanJoin(claims.Role, room) {
		handler.WriteError(w, http.StatusForbidden, handler.CodeForbidden, "the "+room+" room is for staff and admin")
		return
	}
	account, err := s.users.Get(r.Context(), claims.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		handler.WriteError(w, http.StatusUnauthorized, handler.CodeUnauthenticated, "the account no longer exists")
		return
	}
	if err != nil {
		log.Printf("chat: account %d: %v", claims.UserID, err)
		handler.WriteError(w, http.StatusInternalServerError, handler.CodeInternal, "internal server error")
		return
	}
	if s.hub.stopped() {
		handler.WriteError(w, http.StatusServiceUnavailable, "unavailable", "the server is shutting down")
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("chat: upgrade refused: %v", err) // Upgrade has already answered (403 for a foreign Origin)
		return
	}
	c := &Client{hub: s.hub, conn: conn, send: make(chan []byte, s.hub.opts.SendBuffer), room: room, user: account.Email,
		closed: make(chan struct{})}
	if !s.hub.registerClient(c) { // the hub stopped after the check above
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"), time.Now().Add(s.hub.opts.WriteWait))
		conn.Close()
		return
	}
	go c.writePump()
	c.readPump()
}

// sameOrigin is the chat's CheckOrigin (slide 13): only a page of this site may open the
// chat, so another site cannot use the visitor's cookie for it (cross-site WebSocket
// hijacking). A handshake without an Origin header is refused too: browsers always send
// one, and cmd/chatload sends the page's Origin like a browser does.
func sameOrigin(r *http.Request) bool {
	u, err := url.Parse(r.Header.Get("Origin"))
	return err == nil && u.Host != "" && strings.EqualFold(u.Host, r.Host)
}

// StatsHandler serves the chat's statistics: the clients per room and the goroutines of the
// process, the health indicator of slide 28.
//
//	@Summary		Chat statistics
//	@Description	Clients per chat room and the goroutine count of the server (Lesson 14). Admin only.
//	@Tags			chat
//	@Produce		json
//	@Success		200	{object}	chat.Stats
//	@Failure		401	{object}	handler.ErrorResponse
//	@Failure		403	{object}	handler.ErrorResponse
//	@Security		BearerAuth
//	@Router			/chat/stats [get]
func StatsHandler(h *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		handler.WriteJSON(w, http.StatusOK, h.Stats())
	}
}
