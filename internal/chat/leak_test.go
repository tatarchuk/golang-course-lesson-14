package chat_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"homework/internal/chat"
)

// TestConnectDisconnectCyclesNoLeak is slide 43 and homework item 2: 100 clients join and
// leave in five rounds, half with a close frame and half abruptly, and the goroutine count
// returns to where it started.
func TestConnectDisconnectCyclesNoLeak(t *testing.T) {
	e := newEnv(t)
	baseline := runtime.NumGoroutine()
	for cycle := 1; cycle <= 5; cycle++ {
		conns := make([]*websocket.Conn, 20)
		for i := range conns {
			c, _, err := e.dial(int64(i%3)+1, "general", nil)
			if err != nil {
				t.Fatalf("cycle %d dial %d: %v", cycle, i, err)
			}
			conns[i] = c
		}
		e.waitClients(len(conns), 2*time.Second)
		for i, c := range conns {
			send(t, c, fmt.Sprintf(`{"text":"ping %d.%d"}`, cycle, i))
		}
		for i, c := range conns {
			for n := 0; n < len(conns); n++ {
				if m := next(t, c, chat.TypeMessage, 3*time.Second); m.Text == "" {
					t.Fatalf("cycle %d client %d: empty message %+v", cycle, i, m)
				}
			}
		}
		for i, c := range conns {
			if i%2 == 0 {
				_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"), time.Now().Add(time.Second))
				_ = c.Close()
			} else {
				_ = c.NetConn().Close()
			}
		}
		e.waitClients(0, 2*time.Second)
	}
	goroutinesBackTo(t, baseline, 3*time.Second)
}

// TestShutdownClosesClientsWithGoingAway: http.Server.Shutdown leaves hijacked connections
// alone, so the hub closes them itself (registered with RegisterOnShutdown in main): every
// client gets 1001 and every per-client goroutine ends.
func TestShutdownClosesClientsWithGoingAway(t *testing.T) {
	e := newEnv(t)
	baseline := runtime.NumGoroutine()
	conns := make([]*websocket.Conn, 10)
	for i := range conns {
		conns[i] = e.mustDial(int64(i%3)+1, "general")
	}
	e.waitClients(len(conns), 2*time.Second)

	e.hub.Shutdown()
	for i, c := range conns {
		if code := closeCode(t, c, time.Second); code != websocket.CloseGoingAway {
			t.Fatalf("client %d: close code %d, want 1001", i, code)
		}
	}
	goroutinesBackTo(t, baseline, 3*time.Second)
}
