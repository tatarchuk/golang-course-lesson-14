package chat

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// These tests talk to the hub without sockets: a Client is only its room, its name and
// its send queue, which is all the hub touches.

func testOptions() Options {
	return Options{PongWait: time.Minute, PingPeriod: 54 * time.Second, WriteWait: 10 * time.Second,
		MaxMessageBytes: 4096, MaxTextRunes: 1000, SendBuffer: 256, HistorySize: 50}
}

func startHub(t *testing.T, o Options) *Hub {
	t.Helper()
	h := NewHub(o)
	go h.Run()
	t.Cleanup(h.Shutdown)
	return h
}

func newTestClient(room, user string, buffer int) *Client {
	return &Client{send: make(chan []byte, buffer), room: room, user: user}
}

// within runs f and reports a test error when f has not returned after d.
func within(t *testing.T, d time.Duration, what string, f func()) bool {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		t.Errorf("%s did not return within %v", what, d)
		return false
	}
}

// queued returns every frame waiting in c's queue once the hub has finished its current
// work (a Stats round trip proves that).
func queued(t *testing.T, h *Hub, c *Client) []Message {
	t.Helper()
	h.Stats()
	var out []Message
	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				return out
			}
			var m Message
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("frame %s: %v", data, err)
			}
			out = append(out, m)
		default:
			return out
		}
	}
}

func ofType(frames []Message, typ string) []Message {
	var out []Message
	for _, m := range frames {
		if m.Type == typ {
			out = append(out, m)
		}
	}
	return out
}

func TestRoomsAreIsolated(t *testing.T) {
	h := startHub(t, testOptions())
	general := newTestClient(RoomGeneral, "member@albums.local", 16)
	staff := newTestClient(RoomStaff, "staff@albums.local", 16)
	h.registerClient(general)
	h.registerClient(staff)
	queued(t, h, general)
	queued(t, h, staff)

	h.Publish(RoomGeneral, Message{Type: TypeMessage, Room: RoomGeneral, Author: "member@albums.local", Text: "hi"})
	if got := ofType(queued(t, h, general), TypeMessage); len(got) != 1 || got[0].Text != "hi" {
		t.Fatalf("general got %+v, want one message hi", got)
	}
	if got := queued(t, h, staff); len(got) != 0 {
		t.Fatalf("staff got %+v, want nothing", got)
	}
}

// TestHistoryReplayedOnJoin: a client that joins (or reconnects) gets the room's last
// HistorySize messages first, oldest first (slide 36: what a reconnecting client needs).
func TestHistoryReplayedOnJoin(t *testing.T) {
	h := startHub(t, testOptions())
	early := newTestClient(RoomGeneral, "early@albums.local", 128)
	h.registerClient(early)
	for i := 1; i <= 51; i++ {
		h.Publish(RoomGeneral, Message{Type: TypeMessage, Room: RoomGeneral, Author: "a@albums.local", Text: fmt.Sprintf("m%d", i)})
	}
	// Publish only queues; once early has all 51 live, the hub has handled them.
	seen := 0
	for deadline := time.Now().Add(time.Second); seen < 51 && time.Now().Before(deadline); {
		seen += len(ofType(queued(t, h, early), TypeMessage))
	}
	if seen != 51 {
		t.Fatalf("early client got %d messages, want 51", seen)
	}
	c := newTestClient(RoomGeneral, "late@albums.local", 16)
	h.registerClient(c)
	frames := queued(t, h, c)
	if len(frames) == 0 || frames[0].Type != TypeHistory {
		t.Fatalf("first frame %+v, want the history", frames)
	}
	hist := frames[0].History
	if len(hist) != 50 || hist[0].Text != "m2" || hist[49].Text != "m51" {
		t.Fatalf("history has %d messages from %q to %q, want 50 from m2 to m51", len(hist), first(hist), last(hist))
	}
}

func first(ms []Message) string {
	if len(ms) == 0 {
		return ""
	}
	return ms[0].Text
}

func last(ms []Message) string {
	if len(ms) == 0 {
		return ""
	}
	return ms[len(ms)-1].Text
}

// TestPresenceCountsOnline: joins and leaves reach the room with the number online.
func TestPresenceCountsOnline(t *testing.T) {
	h := startHub(t, testOptions())
	alice := newTestClient(RoomGeneral, "alice@albums.local", 16)
	bob := newTestClient(RoomGeneral, "bob@albums.local", 16)
	h.registerClient(alice)
	if p := ofType(queued(t, h, alice), TypePresence); len(p) != 1 || p[0].Online != 1 || p[0].Text != "alice@albums.local joined" {
		t.Fatalf("alice saw %+v, want her own join with 1 online", p)
	}
	h.registerClient(bob)
	if p := ofType(queued(t, h, alice), TypePresence); len(p) != 1 || p[0].Online != 2 || p[0].Text != "bob@albums.local joined" {
		t.Fatalf("alice saw %+v, want bob's join with 2 online", p)
	}
	queued(t, h, bob)
	h.unregisterClient(bob)
	if p := ofType(queued(t, h, alice), TypePresence); len(p) != 1 || p[0].Online != 1 || p[0].Text != "bob@albums.local left" {
		t.Fatalf("alice saw %+v, want bob leaving with 1 online", p)
	}
}

// TestStuckClientDropped is slides 28 and 44: a client whose queue is full is dropped with
// 1008 instead of stalling the room, and a late unregister of it changes nothing.
func TestStuckClientDropped(t *testing.T) {
	h := startHub(t, testOptions())
	healthy := newTestClient(RoomGeneral, "healthy@albums.local", 256)
	stuck := newTestClient(RoomGeneral, "stuck@albums.local", 2) // its history and its own join fill it
	h.registerClient(healthy)
	h.registerClient(stuck)

	for i := 1; i <= 10; i++ {
		if !within(t, 100*time.Millisecond, fmt.Sprintf("Publish #%d", i), func() {
			h.Publish(RoomGeneral, Message{Type: TypeMessage, Room: RoomGeneral, Author: "a@albums.local", Text: "tick"})
		}) {
			return
		}
	}
	if got := ofType(queued(t, h, healthy), TypeMessage); len(got) != 10 {
		t.Fatalf("healthy got %d messages, want 10", len(got))
	}
	var clients int
	if !within(t, 100*time.Millisecond, "Stats", func() { clients = h.Stats().Clients }) {
		return
	}
	if clients != 1 {
		t.Fatalf("clients = %d, want 1 (the stuck client dropped)", clients)
	}
	for range stuck.send { // the queued frames, then the close
	}
	if stuck.closeCode != 1008 {
		t.Fatalf("close code %d, want 1008", stuck.closeCode)
	}
	within(t, 100*time.Millisecond, "unregister of the dropped client", func() { h.unregisterClient(stuck) })
	if !within(t, 100*time.Millisecond, "Stats after the late unregister", func() { clients = h.Stats().Clients }) {
		return
	}
	if clients != 1 {
		t.Fatalf("clients = %d after the late unregister, want 1", clients)
	}
}

// TestShutdownIsIdempotentAndUnblocks: Shutdown closes every client with 1001 exactly once,
// a second call is harmless, and whatever comes after (a readPump's late unregister, a
// REST notice, a new connection) returns at once.
func TestShutdownIsIdempotentAndUnblocks(t *testing.T) {
	h := NewHub(testOptions())
	go h.Run()
	c := newTestClient(RoomGeneral, "member@albums.local", 16)
	h.registerClient(c)

	h.Shutdown()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("second Shutdown panicked: %v", r)
			}
		}()
		h.Shutdown()
	}()
	for range c.send {
	}
	if c.closeCode != 1001 {
		t.Errorf("close code %d, want 1001", c.closeCode)
	}
	within(t, 100*time.Millisecond, "unregisterClient after Shutdown", func() { h.unregisterClient(c) })
	within(t, 100*time.Millisecond, "Publish after Shutdown", func() {
		h.Publish(RoomGeneral, Message{Type: TypeNotice, Room: RoomGeneral, Text: "late"})
	})
	var ok bool
	within(t, 100*time.Millisecond, "registerClient after Shutdown", func() { ok = h.registerClient(newTestClient(RoomGeneral, "x@albums.local", 1)) })
	if ok {
		t.Error("registerClient after Shutdown returned true")
	}
}

// TestShutdownWaitsForWriters: http.Server.Shutdown starts the RegisterOnShutdown hook in
// its own goroutine and main returns right after it, so Shutdown must not return before
// every connection's writer has sent its close frame, and gives up after WriteWait.
func TestShutdownWaitsForWriters(t *testing.T) {
	o := testOptions()
	o.WriteWait = 300 * time.Millisecond
	h := NewHub(o)
	go h.Run()
	slow := newTestClient(RoomGeneral, "slow@albums.local", 4)
	slow.closed = make(chan struct{}) // closed when its writer has sent the close frame
	stuck := newTestClient(RoomGeneral, "stuck@albums.local", 4)
	stuck.closed = make(chan struct{}) // its writer never finishes
	h.registerClient(slow)
	h.registerClient(stuck)

	returned := make(chan struct{})
	go func() {
		h.Shutdown()
		close(returned)
	}()
	select {
	case <-returned:
		t.Fatal("Shutdown returned before the writers sent their close frames")
	case <-time.After(100 * time.Millisecond):
	}
	close(slow.closed)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Shutdown still waits for a stuck writer after WriteWait")
	}
}

func TestOptionsValidate(t *testing.T) {
	if err := DefaultOptions().Validate(); err != nil {
		t.Fatalf("DefaultOptions: %v", err)
	}
	for name, mutate := range map[string]func(*Options){
		"unbuffered send":            func(o *Options) { o.SendBuffer = 0 },
		"no ping period":             func(o *Options) { o.PingPeriod = 0 },
		"ping period over pong wait": func(o *Options) { o.PingPeriod = o.PongWait },
		"no write deadline":          func(o *Options) { o.WriteWait = 0 },
		"no read limit":              func(o *Options) { o.MaxMessageBytes = 0 },
		"no text limit":              func(o *Options) { o.MaxTextRunes = 0 },
		"negative history":           func(o *Options) { o.HistorySize = -1 },
	} {
		o := DefaultOptions()
		mutate(&o)
		if err := o.Validate(); err == nil {
			t.Errorf("%s: Validate accepted %+v", name, o)
		}
	}
}

func TestRoomsFor(t *testing.T) {
	for role, want := range map[string]string{"member": "[general]", "staff": "[general staff]", "admin": "[general staff]", "guest": "[]"} {
		if got := fmt.Sprint(RoomsFor(role)); got != want {
			t.Errorf("RoomsFor(%q) = %s, want %s", role, got, want)
		}
	}
	if CanJoin("member", RoomStaff) || !CanJoin("staff", RoomStaff) || CanJoin("admin", "random") {
		t.Error("CanJoin disagrees with RoomsFor")
	}
}
