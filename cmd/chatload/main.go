// Command chatload exercises the live chat of the albums server from the terminal (Lesson
// 14, slide 43): it logs in through the API, opens -clients WebSockets to /ws/chat with the
// session cookie (the browser's way: no token in the URL), lets every client send one
// message and read the others, closes them (with a close frame, or with -abrupt without
// one), repeats that -cycles times and compares the server's goroutines before and after
// (GET /api/v1/chat/stats, admin only). It exits with 1 when a handshake is refused or the
// server keeps clients or goroutines.
//
//	go run ./cmd/chatload -email admin@albums.local -password "$ADMIN_PASSWORD" -clients 50 -cycles 10
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var (
	base     = flag.String("base", "http://127.0.0.1:8080", "server address")
	email    = flag.String("email", envOr("ADMIN_EMAIL", "admin@albums.local"), "account to log in with (default $ADMIN_EMAIL)")
	password = flag.String("password", os.Getenv("ADMIN_PASSWORD"), "its password (default $ADMIN_PASSWORD)")
	room     = flag.String("room", "general", "chat room: general or staff")
	clients  = flag.Int("clients", 20, "clients per cycle")
	cycles   = flag.Int("cycles", 5, "number of cycles")
	abrupt   = flag.Bool("abrupt", false, "drop the TCP connection instead of sending a close frame")
	settle   = flag.Duration("settle", 5*time.Second, "how long the server may take to clean up")
)

// plain serves the login and the stats; without keep-alive these calls leave no idle
// connection, and no server goroutine, behind.
var plain = &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}

type stats struct {
	Clients    int            `json:"clients"`
	Rooms      map[string]int `json:"rooms"`
	Goroutines int            `json:"goroutines"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	token, err := login()
	if err != nil {
		return err
	}
	before, err := serverStats(token)
	measured := err == nil
	if measured {
		log.Printf("before: clients=%d %v goroutines=%d", before.Clients, before.Rooms, before.Goroutines)
	} else {
		log.Printf("no server statistics (%v): the cycles run without the goroutine check", err)
	}
	how := "with a close frame (1000)"
	if *abrupt {
		how = "abruptly (TCP closed, no close frame)"
	}
	log.Printf("%d cycles of %d clients as %s in #%s, leaving %s", *cycles, *clients, *email, *room, how)

	for cycle := 1; cycle <= *cycles; cycle++ {
		if err := runCycle(cycle, token); err != nil {
			return err
		}
	}
	if !measured {
		return nil
	}

	deadline := time.Now().Add(*settle)
	after, err := serverStats(token)
	for err == nil && (after.Clients != before.Clients || after.Goroutines > before.Goroutines+2) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		after, err = serverStats(token)
	}
	if err != nil {
		return err
	}
	log.Printf("after:  clients=%d %v goroutines=%d (%+d against the start)",
		after.Clients, after.Rooms, after.Goroutines, after.Goroutines-before.Goroutines)
	if after.Clients != before.Clients || after.Goroutines > before.Goroutines+2 {
		return fmt.Errorf("the server did not return to its baseline within %v", *settle)
	}
	log.Print("OK: no client left behind, the goroutine count is back to the baseline")
	return nil
}

func runCycle(cycle int, token string) error {
	conns := make([]*websocket.Conn, *clients)
	for i := range conns {
		c, err := dial(token)
		if err != nil {
			return fmt.Errorf("cycle %d client %d: %w", cycle, i+1, err)
		}
		conns[i] = c
	}
	for i, c := range conns {
		msg, _ := json.Marshal(map[string]string{"text": fmt.Sprintf("load cycle %d client %d", cycle, i+1)})
		if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
			return fmt.Errorf("cycle %d client %d write: %w", cycle, i+1, err)
		}
	}
	received := 0
	readBy := time.Now().Add(5 * time.Second) // one deadline per cycle: a lost message costs 5 s once
	for _, c := range conns {
		_ = c.SetReadDeadline(readBy)
		for n := 0; n < len(conns); {
			_, data, err := c.ReadMessage()
			if err != nil {
				break
			}
			var m struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(data, &m) == nil && m.Type == "message" {
				n++
				received++
			}
		}
	}
	for _, c := range conns {
		if *abrupt {
			_ = c.NetConn().Close()
			continue
		}
		_ = c.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"), time.Now().Add(time.Second))
		_ = c.Close()
	}
	log.Printf("cycle %d/%d: %d connected, %d of %d messages received, all left",
		cycle, *cycles, len(conns), received, len(conns)*len(conns))
	return nil
}

func login() (string, error) {
	body, _ := json.Marshal(map[string]string{"email": *email, "password": *password})
	resp, err := plain.Post(*base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Token string `json:"token"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&out) != nil || out.Token == "" {
		return "", fmt.Errorf("login as %s: %s", *email, resp.Status)
	}
	return out.Token, nil
}

// dial opens the chat the browser's way: the page's Origin and the session cookie.
func dial(token string) (*websocket.Conn, error) {
	u := "ws" + strings.TrimPrefix(*base, "http") + "/ws/chat?room=" + url.QueryEscape(*room)
	hdr := http.Header{"Origin": {*base}, "Cookie": {"albums_token=" + token}}
	c, resp, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil && resp != nil {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("handshake refused: %s %s", resp.Status, bytes.TrimSpace(msg))
	}
	return c, err
}

func serverStats(token string) (stats, error) {
	var s stats
	req, _ := http.NewRequest(http.MethodGet, *base+"/api/v1/chat/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := plain.Do(req)
	if err != nil {
		return s, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return s, errors.New("GET /api/v1/chat/stats: " + resp.Status)
	}
	return s, json.NewDecoder(resp.Body).Decode(&s)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
