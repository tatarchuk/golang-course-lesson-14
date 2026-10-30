package middleware

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

// statusWriter remembers the status code so it can be logged after the handler returns.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Hijack hands the connection over to a WebSocket upgrade (Lesson 14): gorilla/websocket
// takes it with w.(http.Hijacker), so without this method every upgrade behind Logging
// failed with a 500. The upgrader writes the 101 itself, so it is recorded here.
func (s *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("middleware: %T cannot be hijacked", s.ResponseWriter)
	}
	conn, rw, err := h.Hijack()
	if err == nil {
		s.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

// Unwrap lets http.NewResponseController reach the writer underneath (Flush, deadlines).
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Logging writes one line per request: method, path, status, duration. For a WebSocket the
// status is 101 and the duration is the life of the connection.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK} // 200 if WriteHeader is never called
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}
