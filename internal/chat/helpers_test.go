package chat_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"homework/internal/auth"
	"homework/internal/chat"
	"homework/internal/model"
	"homework/internal/repository"
)

// accounts are the users the fake store knows: one per role.
var accounts = map[int64]model.User{
	1: {ID: 1, Email: "alice@albums.local", Role: model.UserRoleMember},
	2: {ID: 2, Email: "bob@albums.local", Role: model.UserRoleStaff},
	3: {ID: 3, Email: "carol@albums.local", Role: model.UserRoleAdmin},
}

type fakeUsers struct{}

func (fakeUsers) Get(_ context.Context, id int64) (model.User, error) {
	u, ok := accounts[id]
	if !ok {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

// env is a running hub with the chat handler behind a test middleware that does what
// CookieAuth does in the app: the claims of the account named in X-Test-Account go into
// the request context (X-Test-Role overrides the role, for an account that is gone).
type env struct {
	t      *testing.T
	hub    *chat.Hub
	srv    *httptest.Server
	origin string
}

func newEnv(t *testing.T, tune ...func(*chat.Options)) *env {
	t.Helper()
	o := chat.DefaultOptions()
	for _, f := range tune {
		f(&o)
	}
	hub := chat.NewHub(o)
	go hub.Run()
	h := chat.NewHandler(hub, fakeUsers{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, err := strconv.ParseInt(r.Header.Get("X-Test-Account"), 10, 64); err == nil {
			role := r.Header.Get("X-Test-Role")
			if role == "" {
				role = accounts[id].Role
			}
			r = r.WithContext(auth.WithUser(r.Context(), auth.Claims{UserID: id, Role: role}))
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(func() {
		hub.Shutdown()
		srv.CloseClientConnections()
		srv.Close()
	})
	return &env{t: t, hub: hub, srv: srv, origin: srv.URL}
}

// dial opens /ws/chat?room=room as the account (0: no claims) with the given extra headers;
// the server's own URL is the Origin unless the headers name another.
func (e *env) dial(account int64, room string, extra http.Header) (*websocket.Conn, *http.Response, error) {
	hdr := http.Header{"Origin": {e.origin}}
	for k, v := range extra {
		hdr[k] = v
	}
	if account != 0 {
		hdr.Set("X-Test-Account", strconv.FormatInt(account, 10))
	}
	u := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/ws/chat?room=" + room
	return websocket.DefaultDialer.Dial(u, hdr)
}

func (e *env) mustDial(account int64, room string) *websocket.Conn {
	e.t.Helper()
	c, _, err := e.dial(account, room, nil)
	if err != nil {
		e.t.Fatalf("dial %s as %d: %v", room, account, err)
	}
	e.t.Cleanup(func() { _ = c.Close() })
	return c
}

// waitClients polls until the hub reports n clients.
func (e *env) waitClients(n int, d time.Duration) {
	e.t.Helper()
	deadline := time.Now().Add(d)
	got := e.hub.Stats().Clients
	for got != n && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		got = e.hub.Stats().Clients
	}
	if got != n {
		e.t.Fatalf("hub has %d clients after %v, want %d", got, d, n)
	}
}

func send(t *testing.T, c *websocket.Conn, text string) {
	t.Helper()
	if err := c.WriteMessage(websocket.TextMessage, []byte(text)); err != nil {
		t.Fatal(err)
	}
}

// next reads frames until one of type typ arrives within d.
func next(t *testing.T, c *websocket.Conn, typ string, d time.Duration) chat.Message {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(d))
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for a %s frame: %v", typ, err)
		}
		var m chat.Message
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("frame %s: %v", data, err)
		}
		if m.Type == typ {
			return m
		}
	}
}

// noMore fails when a frame of type typ arrives within d. It ends the connection's reads:
// gorilla treats a read timeout as final.
func noMore(t *testing.T, c *websocket.Conn, typ string, d time.Duration) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(d))
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			var ne net.Error
			if !errors.As(err, &ne) || !ne.Timeout() {
				t.Errorf("read: %v, want a timeout", err)
			}
			return
		}
		var m chat.Message
		if json.Unmarshal(data, &m) == nil && m.Type == typ {
			t.Errorf("unexpected %s frame: %s", typ, data)
			return
		}
	}
}

// closeCode reads until the connection ends and returns the code of the server's close frame.
func closeCode(t *testing.T, c *websocket.Conn, d time.Duration) int {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(d))
	for {
		_, _, err := c.ReadMessage()
		if err == nil {
			continue
		}
		var ce *websocket.CloseError
		if errors.As(err, &ce) {
			return ce.Code
		}
		t.Fatalf("connection ended without a close frame: %v", err)
		return 0
	}
}

// goroutinesBackTo waits until the process runs at most baseline+2 goroutines.
func goroutinesBackTo(t *testing.T, baseline int, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	n := runtime.NumGoroutine()
	for n > baseline+2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		n = runtime.NumGoroutine()
	}
	if n > baseline+2 {
		t.Fatalf("%d goroutines %v after the clients left, baseline %d: about %d leaked", n, d, baseline, n-baseline)
	}
}
