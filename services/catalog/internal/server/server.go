// Package server wires the HTTP routes for the catalog service.
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// Pinger reports whether a dependency is reachable.
// Handlers depend on this small interface so tests can fake it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps is everything the HTTP layer needs. main.go builds it; tests build
// it from fakes and the in-memory repository.
type Deps struct {
	Mongo   Pinger
	Catalog *catalog.Service
}

// New returns the service's HTTP handler with middleware applied.
func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(d.Mongo))
	// "/products/facets" is a literal, so it wins over "/products/{handle}".
	// That makes "facets" a reserved product handle.
	mux.HandleFunc("GET /products", listProductsHandler(d.Catalog))
	mux.HandleFunc("GET /products/facets", productFacetsHandler(d.Catalog))
	mux.HandleFunc("GET /products/{handle}", getProductHandler(d.Catalog))
	mux.HandleFunc("GET /collections", listCollectionsHandler(d.Catalog))
	mux.HandleFunc("GET /collections/{handle}", getCollectionHandler(d.Catalog))
	mux.HandleFunc("GET /collections/{handle}/products", collectionProductsHandler(d.Catalog))
	mux.HandleFunc("GET /collections/{handle}/facets", collectionFacetsHandler(d.Catalog))

	// Order matters: logging wraps recovery so a panic still gets logged as a 500.
	return logRequests(recoverPanics(mux))
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
		writeJSON(w, code, resp)
	}
}

// writeJSON sends v as JSON with the given status code.
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

// writeError sends the standard error body: {"error": "message"}.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, errorResponse{Error: msg})
}
