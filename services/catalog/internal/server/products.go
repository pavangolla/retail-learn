package server

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// listProductsHandler serves GET /products?limit=20&after=<handle>.
func listProductsHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		limit := 0 // 0 means "use the default"
		if raw := q.Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				writeError(w, http.StatusBadRequest, "limit must be a positive integer")
				return
			}
			limit = n // the service clamps it to the maximum
		}

		page, err := svc.List(r.Context(), limit, q.Get("after"))
		if err != nil {
			log.Printf("list products: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

// getProductHandler serves GET /products/{handle}.
func getProductHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := svc.Get(r.Context(), r.PathValue("handle"))
		switch {
		case errors.Is(err, catalog.ErrNotFound):
			writeError(w, http.StatusNotFound, "product not found")
		case err != nil:
			log.Printf("get product: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		default:
			writeJSON(w, http.StatusOK, p)
		}
	}
}
