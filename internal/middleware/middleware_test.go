package middleware

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// okHandler is a downstream handler that writes a 201 with a small JSON body.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
}

// captureLog redirects the standard logger into a buffer until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

// syncLog is captureLog for a handler that logs from the server's goroutine while the test
// reads: every access goes through a mutex.
type syncLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncLog) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncLog) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func captureSyncLog(t *testing.T) *syncLog {
	t.Helper()
	s := &syncLog{}
	log.SetOutput(s)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return s
}

// TestLoggingPassesHijackerThrough (Lesson 14): gorilla/websocket upgrades a connection
// through w.(http.Hijacker), so the writer Logging wraps around the response must still
// hijack, and the log line reports the 101 the client got.
func TestLoggingPassesHijackerThrough(t *testing.T) {
	logs := captureSyncLog(t)
	var upgrader websocket.Upgrader
	srv := httptest.NewServer(Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return // Upgrade has already answered with an HTTP error
		}
		_ = conn.Close()
	})))
	defer srv.Close()

	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("upgrade through Logging: %v (HTTP %d)", err, status)
	}
	_ = conn.Close()

	deadline := time.Now().Add(time.Second)
	for !strings.Contains(logs.String(), "GET /ws -> 101") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(logs.String(), "GET /ws -> 101") {
		t.Fatalf("log %q does not report the 101", logs.String())
	}
}

func TestRecoverTurnsPanicIntoJSON500(t *testing.T) {
	logs := captureLog(t)
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret: db password is hunter2")
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: want application/json, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"internal_server_error"`) {
		t.Errorf("body should carry the internal_server_error code, got %s", body)
	}
	if strings.Contains(body, "hunter2") {
		t.Errorf("body must not reveal the panic value, got %s", body)
	}
	if !strings.Contains(logs.String(), "hunter2") {
		t.Errorf("the panic value should be in the log, got %q", logs.String())
	}
}

func TestRecoverPassesNormalResponsesThrough(t *testing.T) {
	w := httptest.NewRecorder()

	Recover(okHandler()).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusCreated || w.Body.String() != `{"ok":true}` {
		t.Errorf("unexpected response: %d %s", w.Code, w.Body)
	}
}

func TestCORSAnswersPreflightWithoutCallingNext(t *testing.T) {
	called := false
	h := CORS(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/albums", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status: want 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("preflight must have no body, got %q", w.Body)
	}
	if called {
		t.Error("preflight must not reach the next handler")
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin: want *, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Allow-Methods must list POST, got %q", got)
	}
}

func TestCORSAddsHeaderToNormalResponses(t *testing.T) {
	w := httptest.NewRecorder()

	CORS(okHandler()).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusCreated {
		t.Errorf("next handler must run: want 201, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("missing Access-Control-Allow-Origin")
	}
}

func TestLoggingRecordsMethodPathAndStatus(t *testing.T) {
	logs := captureLog(t)

	w := httptest.NewRecorder()
	Logging(okHandler()).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/albums", nil))

	for _, want := range []string{"POST", "/api/v1/albums", "201"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log line %q should contain %q", logs.String(), want)
		}
	}
	if w.Code != http.StatusCreated {
		t.Errorf("status must pass through the wrapper, got %d", w.Code)
	}
}

func TestLoggingReports200WhenWriteHeaderIsNeverCalled(t *testing.T) {
	logs := captureLog(t)
	h := Logging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hi")) // implicit 200
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	if !strings.Contains(logs.String(), "200") {
		t.Errorf("log line %q should report 200", logs.String())
	}
}

// The chain used by NewRouter: CORS sets its headers before the handler runs,
// so even a 500 produced by Recover still carries them.
func TestChainKeepsCORSHeadersOnPanic(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	h := Logging(Recover(CORS(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("a 500 produced by Recover must still carry the CORS header")
	}
}
