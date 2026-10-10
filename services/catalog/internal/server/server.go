// Package server wires the HTTP routes for the catalog service.
package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// Pinger reports whether a dependency is reachable.
// Handlers depend on this small interface so tests can fake it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// TCPPinger checks that something accepts connections on Addr.
// Phase 0 uses this for MongoDB. In Phase 1 it gets replaced by the
// real driver's Ping once the catalog actually stores data.
type TCPPinger struct {
	Addr string
}

// Ping dials Addr and closes the connection immediately.
func (p TCPPinger) Ping(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", p.Addr)
	if err != nil {
		return err
	}
	return conn.Close()
}

// New returns the service's HTTP handler.
func New(mongo Pinger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(mongo))
	return mux
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Mongo   string `json:"mongo"`
}

// healthHandler is a readiness check: it is only "ok" when the
// dependencies the service needs are reachable. A plain liveness
// check would just return 200 without looking at dependencies.
func healthHandler(mongo Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		resp := healthResponse{Status: "ok", Service: "catalog", Mongo: "up"}
		code := http.StatusOK
		if err := mongo.Ping(ctx); err != nil {
			resp.Status = "degraded"
			resp.Mongo = "down"
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
