package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"homework/internal/model"
)

// Lesson 14 acceptance tests: the chat behind the real router, over a real HTTP server
// (a WebSocket needs a connection to hijack) and the test database.

func chatServer(t *testing.T) (http.Handler, *httptest.Server) {
	t.Helper()
	h := newRouter(t)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return h, srv
}

// dialChat opens /ws/chat?room=room from the server's own page, with the session cookie
// when token is not empty, the way the browser does.
func dialChat(srv *httptest.Server, room, token string) (*websocket.Conn, *http.Response, error) {
	hdr := http.Header{"Origin": {srv.URL}}
	if token != "" {
		hdr.Set("Cookie", "albums_token="+token)
	}
	return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/chat?room="+room, hdr)
}

func TestChatRoutesWired(t *testing.T) {
	h, srv := chatServer(t)
	member := id(register(t, h, "member@albums.local"))
	memberToken := mintToken(member, model.UserRoleMember, time.Hour)

	for name, tc := range map[string]struct {
		room, token string
		status      int
	}{
		"no session":                 {"general", "", http.StatusUnauthorized},
		"a member in the staff room": {"staff", memberToken, http.StatusForbidden},
	} {
		c, resp, err := dialChat(srv, tc.room, tc.token)
		if err == nil {
			c.Close()
			t.Errorf("%s: upgraded, want %d", name, tc.status)
			continue
		}
		if resp == nil || resp.StatusCode != tc.status {
			t.Errorf("%s: %v, want %d", name, err, tc.status)
		}
	}
	c, _, err := dialChat(srv, "general", memberToken)
	if err != nil {
		t.Fatalf("a member in general: %v", err)
	}
	c.Close()

	expect(t, call(h, "GET", "/api/v1/chat/stats", ""), http.StatusUnauthorized, "unauthenticated")
	expect(t, authCall(h, "GET", "/api/v1/chat/stats", "", memberToken), http.StatusForbidden, "forbidden")
	stats := expect(t, authCall(h, "GET", "/api/v1/chat/stats", "", mintToken(member, model.UserRoleAdmin, time.Hour)), http.StatusOK, "")
	if _, ok := stats["clients"]; !ok {
		t.Fatalf("stats %v have no clients field", stats)
	}

	req := newRequest("GET", "/chat", "")
	req.AddCookie(&http.Cookie{Name: "albums_token", Value: memberToken})
	page := serve(h, req)
	if page.Code != http.StatusOK || !strings.Contains(page.Header.Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatalf("GET /chat: %d with CSP %q, want 200 under the web UI's policy", page.Code, page.Header.Get("Content-Security-Policy"))
	}
}

// TestAlbumCreateNotifiesChat: a REST write reaches the people in the chat at once
// (slide 41 for this app; slide 43 asks for about a second).
func TestAlbumCreateNotifiesChat(t *testing.T) {
	h, srv := chatServer(t)
	staff := id(register(t, h, "staff@albums.local"))
	c, _, err := dialChat(srv, "general", mintToken(staff, model.UserRoleStaff, time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	genre, artists := fixtures(t, h)

	expect(t, staffCall(h, "POST", "/api/v1/albums", albumBody("Blue Train", genre, artists[0])), http.StatusCreated, "")
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("no notice within 1s: %v", err)
		}
		var m map[string]any
		if json.Unmarshal(data, &m) == nil && m["type"] == "notice" {
			if text, _ := m["text"].(string); !strings.Contains(text, `Album "Blue Train"`) {
				t.Fatalf("notice %q does not name the album", text)
			}
			return
		}
	}
}
