package server

import (
	"log"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs one line per request: method, route, status, duration.
//
// It logs the route pattern ("GET /carts/{id}/items"), not the URL. A cart id
// is the cart's only credential, so it must never end up in log files.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := r.Pattern // filled in by the mux while routing; empty if nothing matched
		if route == "" {
			route = "(no route)"
		}
		log.Printf("%s -> %d (%s)", route, rec.status, time.Since(start).Round(time.Microsecond))
	})
}

// recoverPanics turns a panic in a handler into a JSON 500.
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

// noStore tells browsers and proxies never to cache a response. A cart is
// private and changes constantly, so a cached copy would be wrong or leaked.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
