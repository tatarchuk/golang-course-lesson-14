package chat_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"homework/internal/chat"
)

// TestThreeClientsEchoOnce is slides 40 and 43: three accounts in general, a message from
// one reaches all three, the sender included, exactly once each.
func TestThreeClientsEchoOnce(t *testing.T) {
	e := newEnv(t)
	alice := e.mustDial(1, "general")
	bob := e.mustDial(2, "general")
	carol := e.mustDial(3, "general")
	e.waitClients(3, 2*time.Second)

	send(t, alice, `{"text":"hello"}`)
	for name, c := range map[string]*websocket.Conn{"alice": alice, "bob": bob, "carol": carol} {
		m := next(t, c, chat.TypeMessage, time.Second)
		if m.Author != "alice@albums.local" || m.Text != "hello" || m.Room != chat.RoomGeneral || m.At.IsZero() {
			t.Errorf("%s got %+v, want hello from alice@albums.local in general with a time", name, m)
		}
		noMore(t, c, chat.TypeMessage, 200*time.Millisecond)
	}
}

// TestAuthorComesFromAccount is slide 40: the author is the account behind the session,
// and the room and the frame type are the server's; whatever else the client sends is
// ignored.
func TestAuthorComesFromAccount(t *testing.T) {
	e := newEnv(t)
	alice := e.mustDial(1, "general")
	bob := e.mustDial(2, "general")
	carol := e.mustDial(3, "staff")
	e.waitClients(3, 2*time.Second)

	send(t, alice, `{"author":"carol@albums.local","room":"staff","type":"notice","text":"x"}`)
	m := next(t, bob, chat.TypeMessage, time.Second)
	if m.Author != "alice@albums.local" || m.Room != chat.RoomGeneral || m.Text != "x" {
		t.Fatalf("bob got %+v, want x from alice@albums.local in general", m)
	}
	noMore(t, carol, chat.TypeMessage, 200*time.Millisecond)
}

func TestRoomAccess(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		name    string
		account int64
		room    string
		extra   http.Header
		status  int
	}{
		{"no session", 0, "general", nil, http.StatusUnauthorized},
		{"an account that no longer exists", 9, "general", http.Header{"X-Test-Role": {"member"}}, http.StatusUnauthorized},
		{"a member in the staff room", 1, "staff", nil, http.StatusForbidden},
		{"an unknown room", 1, "random", nil, http.StatusNotFound},
	}
	for _, tc := range cases {
		c, resp, err := e.dial(tc.account, tc.room, tc.extra)
		if err == nil {
			c.Close()
			t.Errorf("%s: upgraded, want %d", tc.name, tc.status)
			continue
		}
		if resp == nil || resp.StatusCode != tc.status {
			t.Errorf("%s: %v, want %d", tc.name, err, tc.status)
		}
	}
	e.mustDial(2, "staff") // staff may join the staff room
}

// TestForeignOriginRejected is slides 13 and 45: a page of another site cannot open the
// chat with the visitor's cookie (cross-site WebSocket hijacking).
func TestForeignOriginRejected(t *testing.T) {
	e := newEnv(t)
	c, resp, err := e.dial(1, "general", http.Header{"Origin": {"http://evil.example"}})
	if err == nil {
		c.Close()
		t.Fatal("upgraded with a foreign Origin, want 403")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("%v, want 403", err)
	}
}

// TestTextValidation: an empty, overlong or malformed message comes back as an error frame
// to its sender only; a frame over the read limit closes the connection with 1009.
func TestTextValidation(t *testing.T) {
	e := newEnv(t)
	alice := e.mustDial(1, "general")
	bob := e.mustDial(2, "general")
	e.waitClients(2, 2*time.Second)

	for input, want := range map[string]string{
		`{"text":"   "}`: "empty",
		`{"text":"` + strings.Repeat("ж", 1001) + `"}`: "1000",
		`not json`: "JSON",
	} {
		send(t, alice, input)
		if m := next(t, alice, chat.TypeError, time.Second); !strings.Contains(m.Text, want) {
			t.Errorf("error for %.20q is %q, want it to mention %q", input, m.Text, want)
		}
	}
	noMore(t, bob, chat.TypeMessage, 200*time.Millisecond)

	send(t, alice, strings.Repeat("x", 5000))
	if code := closeCode(t, alice, time.Second); code != websocket.CloseMessageTooBig {
		t.Fatalf("close code %d, want 1009", code)
	}
}

// TestAbruptDisconnectUnregisters is slide 43: a client that vanishes without a close frame
// is unregistered.
func TestAbruptDisconnectUnregisters(t *testing.T) {
	e := newEnv(t)
	gone := e.mustDial(1, "general")
	e.mustDial(2, "general")
	e.mustDial(3, "general")
	e.waitClients(3, 2*time.Second)

	_ = gone.NetConn().Close() // a crashed tab or a lost network
	e.waitClients(2, time.Second)
}

// TestDeadPeerTimesOut is slides 15 and 43: a client that stops answering pings is closed
// by the read deadline.
func TestDeadPeerTimesOut(t *testing.T) {
	e := newEnv(t, func(o *chat.Options) {
		o.PongWait = 300 * time.Millisecond
		o.PingPeriod = 100 * time.Millisecond
	})
	e.mustDial(1, "general") // never reads, so it never answers a ping
	e.waitClients(1, 2*time.Second)
	e.waitClients(0, time.Second)
}
