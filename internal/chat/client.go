package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

// Client is one connection as the hub sees it. room and user are set before the client is
// registered and never change afterwards, so the hub and the pumps read them without a
// lock; only the hub sends on send and closes it.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	room      string
	user      string // the account's e-mail: the author of everything this client sends
	closeCode int    // set by the hub before it closes send: the close frame writePump sends
	closeText string
	closed    chan struct{} // closed by writePump when it returns: its close frame is sent
}

func (c *Client) String() string { return fmt.Sprintf("user=%s room=%s", c.user, c.room) }

// readPump is the connection's only reader (slides 14 and 27). The read deadline is the
// keepalive of slide 15: every pong renews it, so a peer that stops answering pings fails
// the next read. Any read error ends the loop; the deferred unregister lets the hub close
// send, which ends writePump.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregisterClient(c)
		c.conn.Close()
	}()
	o := c.hub.opts
	c.conn.SetReadLimit(o.MaxMessageBytes)
	_ = c.conn.SetReadDeadline(time.Now().Add(o.PongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(o.PongWait))
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("chat: %s disconnected: %s", c, disconnectReason(err))
			return
		}
		text, problem := parseText(data, o.MaxTextRunes)
		if problem != "" {
			c.hub.sendError(c, problem)
			continue
		}
		c.hub.Publish(c.room, Message{Type: TypeMessage, Author: c.user, Text: text})
	}
}

// writePump is the connection's only writer (slides 14 and 44): the hub's frames and the
// pings share this goroutine, every write has a deadline and the first failed write ends
// it. When the hub closes send, writePump sends the close frame the hub chose and returns.
func (c *Client) writePump() {
	o := c.hub.opts
	ticker := time.NewTicker(o.PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		close(c.closed)
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(o.WriteWait))
			if !ok {
				if c.closeCode != 0 {
					_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(c.closeCode, c.closeText))
				}
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(o.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// parseText reads {"text": "..."}; every other field is ignored, because the author, the
// room and the type are the server's (slide 40). It returns the trimmed text, or why the
// message was refused.
func parseText(data []byte, maxRunes int) (string, string) {
	var in struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return "", `the message is not JSON: send {"text": "..."}`
	}
	text := strings.TrimSpace(in.Text)
	switch {
	case text == "":
		return "", "the message is empty"
	case utf8.RuneCountInString(text) > maxRunes:
		return "", fmt.Sprintf("the message is longer than %d characters", maxRunes)
	}
	return text, ""
}

// disconnectReason turns a read error into a log line (slide 16).
func disconnectReason(err error) string {
	var ce *websocket.CloseError
	var ne net.Error
	switch {
	case errors.As(err, &ce):
		return strings.TrimSpace(fmt.Sprintf("closed by the client (%d) %s", ce.Code, ce.Text))
	case errors.Is(err, websocket.ErrReadLimit):
		return "message over the read limit, closed with 1009"
	case errors.As(err, &ne) && ne.Timeout():
		return "no pong before the read deadline"
	default:
		return "connection lost: " + err.Error()
	}
}
