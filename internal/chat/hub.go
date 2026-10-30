package chat

import (
	"encoding/json"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Stats is what GET /api/v1/chat/stats reports: the health indicator of slide 28.
type Stats struct {
	Clients    int            `json:"clients"`
	Rooms      map[string]int `json:"rooms"`
	Goroutines int            `json:"goroutines"`
}

// direct is a frame for one client only (an error about its own message).
type direct struct {
	c *Client
	m Message
}

// broadcastBuffer lets Publish queue a message and move on while the hub is busy.
const broadcastBuffer = 256

// Hub is the chat's hub (slide 26). Run is the only goroutine that reads or writes the
// registry h.rooms and the history, the only one that sends on a client's queue and the
// only one that closes it, so the hub needs no lock (slide 25).
type Hub struct {
	opts       Options
	rooms      map[string]map[*Client]bool
	history    map[string][]Message
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
	direct     chan direct
	stats      chan chan Stats
	quit       chan struct{} // closed by Shutdown
	done       chan struct{} // closed when Run has returned
	stopOnce   sync.Once
}

// NewHub returns a hub; start it with go h.Run(). It panics on invalid options.
func NewHub(o Options) *Hub {
	if err := o.Validate(); err != nil {
		panic("chat: " + err.Error())
	}
	return &Hub{
		opts:       o,
		rooms:      map[string]map[*Client]bool{RoomGeneral: {}, RoomStaff: {}},
		history:    map[string][]Message{},
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Message, broadcastBuffer),
		direct:     make(chan direct, broadcastBuffer),
		stats:      make(chan chan Stats),
		quit:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Run is the hub's main loop. It never waits for a client: a client whose queue is full is
// dropped with 1008 (slides 28 and 44).
func (h *Hub) Run() {
	defer close(h.done)
	for {
		select {
		case c := <-h.register:
			h.join(c)
		case c := <-h.unregister:
			if h.drop(c, 0, "left") {
				h.publish(h.presence(c, "left"))
			}
		case m := <-h.broadcast:
			h.remember(m)
			h.publish(m)
		case d := <-h.direct:
			if h.rooms[d.c.room][d.c] {
				h.deliver(d.c, d.m)
			}
		case reply := <-h.stats:
			reply <- h.snapshot()
		case <-h.quit:
			for _, members := range h.rooms {
				for c := range members {
					h.drop(c, websocket.CloseGoingAway, "server shutdown")
				}
			}
			return
		}
	}
}

// join registers c, sends it the room's history and announces it to the room.
func (h *Hub) join(c *Client) {
	h.rooms[c.room][c] = true
	log.Printf("chat: %s joined (%d in the room)", c, len(h.rooms[c.room]))
	h.deliver(c, Message{Type: TypeHistory, Room: c.room, At: now(), History: h.history[c.room]})
	if h.rooms[c.room][c] { // still there: the history fitted into its queue
		h.publish(h.presence(c, "joined"))
	}
}

// drop removes c and closes its queue; writePump then sends a close frame with code (0:
// none, the client has gone already). Only a registered client is dropped, so a late
// unregister of a client the hub dropped before changes nothing: every queue is closed
// exactly once (slide 27).
func (h *Hub) drop(c *Client, code int, reason string) bool {
	if !h.rooms[c.room][c] {
		return false
	}
	delete(h.rooms[c.room], c)
	c.closeCode, c.closeText = code, reason
	close(c.send)
	log.Printf("chat: %s left: %s (%d in the room)", c, reason, len(h.rooms[c.room]))
	return true
}

// publish delivers m to every member of its room. Members whose queue is full are dropped
// and their departure announced in turn, until nobody is left to drop.
func (h *Hub) publish(m Message) {
	queue := []Message{m}
	for len(queue) > 0 {
		m, queue = queue[0], queue[1:]
		data, _ := json.Marshal(m)
		for c := range h.rooms[m.Room] {
			select {
			case c.send <- data:
			default:
				if h.drop(c, websocket.ClosePolicyViolation, "slow consumer: send queue full") {
					queue = append(queue, h.presence(c, "left"))
				}
			}
		}
	}
}

// deliver sends m to c alone, dropping c when its queue is full.
func (h *Hub) deliver(c *Client, m Message) {
	data, _ := json.Marshal(m)
	select {
	case c.send <- data:
	default:
		if h.drop(c, websocket.ClosePolicyViolation, "slow consumer: send queue full") {
			h.publish(h.presence(c, "left"))
		}
	}
}

// remember keeps the room's last HistorySize messages and notices for clients that join.
func (h *Hub) remember(m Message) {
	if m.Type != TypeMessage && m.Type != TypeNotice || h.opts.HistorySize == 0 {
		return
	}
	hist := append(h.history[m.Room], m)
	if len(hist) > h.opts.HistorySize {
		hist = append([]Message(nil), hist[len(hist)-h.opts.HistorySize:]...)
	}
	h.history[m.Room] = hist
}

func (h *Hub) presence(c *Client, verb string) Message {
	return Message{Type: TypePresence, Room: c.room, Author: c.user, Text: c.user + " " + verb,
		Online: len(h.rooms[c.room]), At: now()}
}

func (h *Hub) snapshot() Stats {
	s := Stats{Rooms: map[string]int{}, Goroutines: runtime.NumGoroutine()}
	for room, members := range h.rooms {
		s.Rooms[room] = len(members)
		s.Clients += len(members)
	}
	return s
}

// Publish sends m to room: a chat message from a client, or a notice from the REST side.
// It waits only while the queue of the hub is full and returns at once when the hub has
// stopped.
func (h *Hub) Publish(room string, m Message) {
	m.Room = room
	if m.At.IsZero() {
		m.At = now()
	}
	select {
	case h.broadcast <- m:
	case <-h.done:
	}
}

// Stats reports the clients per room and the process's goroutines.
func (h *Hub) Stats() Stats {
	reply := make(chan Stats, 1)
	select {
	case h.stats <- reply:
		return <-reply
	case <-h.done:
		return Stats{Rooms: map[string]int{}, Goroutines: runtime.NumGoroutine()}
	}
}

// Shutdown closes every client with 1001 (going away), ends Run and waits for it; a second
// call returns at once. http.Server.Shutdown does not touch hijacked connections, so main
// registers this with RegisterOnShutdown. Run must have been started.
func (h *Hub) Shutdown() {
	h.stopOnce.Do(func() { close(h.quit) })
	<-h.done
}

func (h *Hub) stopped() bool {
	select {
	case <-h.done:
		return true
	default:
		return false
	}
}

// registerClient hands c to the hub; false means the hub has stopped.
func (h *Hub) registerClient(c *Client) bool {
	select {
	case h.register <- c:
		return true
	case <-h.done:
		return false
	}
}

// unregisterClient tells the hub that c has gone; it returns at once when the hub has
// stopped.
func (h *Hub) unregisterClient(c *Client) {
	select {
	case h.unregister <- c:
	case <-h.done:
	}
}

// sendError tells c alone why its message was refused.
func (h *Hub) sendError(c *Client, text string) {
	select {
	case h.direct <- direct{c: c, m: Message{Type: TypeError, Room: c.room, Text: text, At: now()}}:
	case <-h.done:
	}
}

func now() time.Time { return time.Now().UTC() }
