package server

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder remembers the status code a handler wrote, so the logging
// middleware can report it. http.ResponseWriter does not expose it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs one line per request: method, path, status, duration.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.RequestURI(), rec.status, time.Since(start).Round(time.Microsecond))
	})
}

// recoverPanics turns a panic in a handler into a 500 response instead of
// killing the connection (net/http would recover too, but would not send our
// JSON error body).
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, v)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
