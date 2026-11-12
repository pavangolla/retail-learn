// Package server wires the HTTP routes for the cart service.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
)

// Pinger reports whether a dependency is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps is everything the HTTP layer needs.
type Deps struct {
	Redis Pinger
	Cart  *cart.Service
}

// New returns the service's HTTP handler with middleware applied.
func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(d.Redis))

	mux.HandleFunc("POST /carts", createCartHandler(d.Cart))
	mux.HandleFunc("GET /carts/{id}", getCartHandler(d.Cart))
	mux.HandleFunc("DELETE /carts/{id}", clearCartHandler(d.Cart))
	mux.HandleFunc("POST /carts/{id}/items", addItemHandler(d.Cart))
	mux.HandleFunc("PUT /carts/{id}/items/{sku}", setItemHandler(d.Cart))
	mux.HandleFunc("DELETE /carts/{id}/items/{sku}", removeItemHandler(d.Cart))

	return logRequests(recoverPanics(noStore(mux)))
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Redis   string `json:"redis"`
}

func healthHandler(redis Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		resp := healthResponse{Status: "ok", Service: "cart", Redis: "up"}
		code := http.StatusOK
		if err := redis.Ping(ctx); err != nil {
			resp.Status, resp.Redis, code = "degraded", "down", http.StatusServiceUnavailable
		}
		writeJSON(w, code, resp)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorResponse{Error: msg})
}

// writeServiceError maps a domain error to an HTTP status.
func writeServiceError(w http.ResponseWriter, err error) {
	var invalid *cart.InvalidError
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, invalid.Msg)
	case errors.Is(err, cart.ErrQuantityLimit), errors.Is(err, cart.ErrTooManyLines):
		// The request was fine but conflicts with the cart's current state.
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, cart.ErrUnavailable):
		log.Printf("dependency unavailable: %v", err)
		writeError(w, http.StatusServiceUnavailable, "temporarily unavailable, please retry")
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
