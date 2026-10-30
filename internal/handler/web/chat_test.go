package web

import (
	"net/http"
	"strings"
	"testing"

	"homework/internal/model"
)

// TestChatPageRedirectsAnonymous: the chat needs an account (its messages carry the
// author's e-mail), so an anonymous visitor goes to the login form.
func TestChatPageRedirectsAnonymous(t *testing.T) {
	fx := newFixture(t)
	rec := do(fx.server(), request("GET", "/chat", nil, nil, nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("anonymous GET /chat: %d to %q, want 303 to /login", rec.Code, rec.Header().Get("Location"))
	}
}

// TestChatPageRoomsByRole: the page offers the rooms the role may join and opens the one
// asked for only when the role may join it.
func TestChatPageRoomsByRole(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		id       int64
		role     string
		contains []string
		lacks    []string
	}{
		{"a member sees general only", "/chat", 1, model.UserRoleMember,
			[]string{`data-room="general"`, `href="/chat?room=general"`, "member@albums.local (member)"},
			[]string{`href="/chat?room=staff"`}},
		{"staff see both rooms", "/chat?room=staff", 2, model.UserRoleStaff,
			[]string{`data-room="staff"`, `href="/chat?room=general"`, `href="/chat?room=staff"`}, nil},
		{"a member who asks for the staff room gets general", "/chat?room=staff", 1, model.UserRoleMember,
			[]string{`data-room="general"`}, []string{`data-room="staff"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t)
			body := htmlBody(t, do(fx.server(), request("GET", tc.target, nil, sessionFor(t, tc.id, tc.role), nil)), http.StatusOK)
			contains(t, body, tc.contains...)
			lacks(t, body, tc.lacks...)
		})
	}
}

// TestChatPageScriptOnly: the page runs /static/chat.js and no inline script, so the
// Content-Security-Policy "default-src 'self'" of the web UI keeps working; the layout
// links the chat for a signed-in visitor only.
func TestChatPageScriptOnly(t *testing.T) {
	fx := newFixture(t)
	body := htmlBody(t, do(fx.server(), request("GET", "/chat", nil, sessionFor(t, 1, model.UserRoleMember), nil)), http.StatusOK)
	contains(t, body, `<script src="/static/chat.js" defer></script>`, `href="/chat"`)
	if all, external := strings.Count(body, "<script"), strings.Count(body, "<script src="); all != external {
		t.Fatalf("%d <script> tags, %d with src: inline script found", all, external)
	}
	catalog := htmlBody(t, do(fx.server(), request("GET", "/catalog", nil, nil, nil)), http.StatusOK)
	lacks(t, catalog, `href="/chat"`)
}
