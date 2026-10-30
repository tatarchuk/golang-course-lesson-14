package web

import (
	"net/http"

	"homework/internal/chat"
)

// Chat handles GET /chat (Lesson 14): the live chat for a signed-in visitor, in the room
// asked for when the role may join it, otherwise general. The page only loads
// /static/chat.js, which opens the WebSocket; the session cookie authenticates it.
// Anonymous visitors go to the login form: the chat names every author by the account's
// e-mail.
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	who, err := h.visitor(r)
	if err != nil {
		serverError(w, err)
		return
	}
	if who.user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	room := r.URL.Query().Get("room")
	if !chat.CanJoin(who.user.Role, room) {
		room = chat.RoomGeneral
	}
	h.views.renderPage(w, http.StatusOK, pageChat, pageData{
		Title: "Chat", User: who.user, CanDelete: who.canDelete,
		Rooms: chat.RoomsFor(who.user.Role), Room: room,
	})
}
