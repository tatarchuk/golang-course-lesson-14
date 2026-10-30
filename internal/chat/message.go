// Package chat is the live chat of the albums web UI (Lesson 14). It follows the Lesson 14
// deck: a hub on channels with a single owner (slides 20-28), one reader and one writer
// per connection with ping/pong (slides 14, 15, 27), bounded queues that drop a slow
// client (slide 28), and the fixes the slide 42 review found in the slide code (the
// golang-course-lesson-14-examples repository). On top of the deck: rooms by role, the
// author from the session's account, the room's recent messages for a client that joins
// or reconnects, presence, and notices from the REST side.
package chat

import (
	"errors"
	"slices"
	"time"

	"homework/internal/model"
)

// The rooms. Every signed-in role may join general; staff is for staff and admin.
const (
	RoomGeneral = "general"
	RoomStaff   = "staff"
)

// The frame types the server sends.
const (
	TypeMessage  = "message"  // a chat message
	TypeNotice   = "notice"   // a catalogue change announced by the REST side
	TypePresence = "presence" // someone joined or left; Online is the room's new count
	TypeHistory  = "history"  // the room's recent messages, sent to a client that joins
	TypeError    = "error"    // a message of the client was refused; sent to that client only
)

// Message is every frame the server sends. A client sends only {"text": "..."}.
type Message struct {
	Type    string    `json:"type"`
	Room    string    `json:"room"`
	Author  string    `json:"author,omitempty"`
	Text    string    `json:"text,omitempty"`
	Online  int       `json:"online,omitempty"`
	At      time.Time `json:"at"`
	History []Message `json:"history,omitempty"`
}

// Options holds the connection settings.
type Options struct {
	PongWait        time.Duration // read deadline, renewed by every pong (slide 15)
	PingPeriod      time.Duration // how often the server pings; shorter than PongWait
	WriteWait       time.Duration // deadline of one write (slide 16)
	MaxMessageBytes int64         // largest frame a client may send; a bigger one closes with 1009
	MaxTextRunes    int           // longest chat message, in characters
	SendBuffer      int           // capacity of each client's queue (slide 28)
	HistorySize     int           // messages and notices replayed to a client that joins
}

// DefaultOptions are the values of the Lesson 14 examples: the deck's 60 s pong wait with
// pings at 9/10 of it, a 256-message queue (gorilla's chat example), the last 50 messages.
func DefaultOptions() Options {
	return Options{
		PongWait:        60 * time.Second,
		PingPeriod:      54 * time.Second,
		WriteWait:       10 * time.Second,
		MaxMessageBytes: 4096,
		MaxTextRunes:    1000,
		SendBuffer:      256,
		HistorySize:     50,
	}
}

// Validate checks the settings the hub and the pumps rely on.
func (o Options) Validate() error {
	switch {
	case o.SendBuffer <= 0:
		return errors.New("SendBuffer must be positive: with an unbuffered queue the hub drops every client")
	case o.PingPeriod <= 0 || o.PingPeriod >= o.PongWait:
		return errors.New("PingPeriod must be positive and shorter than PongWait")
	case o.WriteWait <= 0:
		return errors.New("WriteWait must be positive")
	case o.MaxMessageBytes <= 0 || o.MaxTextRunes <= 0:
		return errors.New("MaxMessageBytes and MaxTextRunes must be positive")
	case o.HistorySize < 0:
		return errors.New("HistorySize must not be negative")
	}
	return nil
}

// RoomsFor lists the rooms a role may join, general first; an unknown role gets none.
func RoomsFor(role string) []string {
	switch role {
	case model.UserRoleMember:
		return []string{RoomGeneral}
	case model.UserRoleStaff, model.UserRoleAdmin:
		return []string{RoomGeneral, RoomStaff}
	}
	return nil
}

// CanJoin reports whether a role may join room.
func CanJoin(role, room string) bool { return slices.Contains(RoomsFor(role), room) }

func knownRoom(room string) bool { return room == RoomGeneral || room == RoomStaff }
