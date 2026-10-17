package server

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// listProductsHandler serves
//
//	GET /products?limit=&after=&sort=&type=&tag=&minPrice=&maxPrice=&q=
//
// Prices are integer cents. `tag` may repeat (all tags must match). With `q`
// the response is a relevance-ranked search: no `sort`, no `after`, no paging.
func listProductsHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseListQuery(r.URL.Query())
		if err != nil {
			writeServiceError(w, err, "")
			return
		}
		page, err := svc.List(r.Context(), q)
		if err != nil {
			writeServiceError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

// productFacetsHandler serves GET /products/facets?type=&tag=&minPrice=&maxPrice=
func productFacetsHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := parseFilter(r.URL.Query())
		if err != nil {
			writeServiceError(w, err, "")
			return
		}
		facets, err := svc.Facets(r.Context(), f)
		if err != nil {
			writeServiceError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, facets)
	}
}

// getProductHandler serves GET /products/{handle}.
func getProductHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := svc.Get(r.Context(), r.PathValue("handle"))
		if err != nil {
			writeServiceError(w, err, "product not found")
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

// parseListQuery turns URL parameters into a catalog.ListQuery. Malformed
// values come back as *catalog.InvalidError, which becomes a 400.
func parseListQuery(v url.Values) (catalog.ListQuery, error) {
	var q catalog.ListQuery

	if raw := v.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return q, &catalog.InvalidError{Msg: "limit must be a positive integer"}
		}
		q.Limit = n // the service clamps it to the maximum
	}

	sort, err := catalog.ParseSort(v.Get("sort"))
	if err != nil {
		return q, err
	}
	q.Sort = sort

	q.Cursor = v.Get("after")
	q.Search = v.Get("q")

	q.Filter, err = parseFilter(v)
	return q, err
}

// parseFilter reads the filter parameters shared by listings and facets.
func parseFilter(v url.Values) (catalog.Filter, error) {
	f := catalog.Filter{
		ProductType: v.Get("type"),
		Tags:        v["tag"],
	}
	var err error
	if f.MinPriceCents, err = parseCents("minPrice", v.Get("minPrice")); err != nil {
		return f, err
	}
	if f.MaxPriceCents, err = parseCents("maxPrice", v.Get("maxPrice")); err != nil {
		return f, err
	}
	return f, nil
}

func parseCents(name, raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0, &catalog.InvalidError{Msg: name + " must be a non-negative whole number of cents"}
	}
	return n, nil
}

// writeServiceError maps a service error to an HTTP response:
// bad input -> 400, not found -> 404 (with notFoundMsg), anything else -> 500.
func writeServiceError(w http.ResponseWriter, err error, notFoundMsg string) {
	var invalid *catalog.InvalidError
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, invalid.Msg)
	case errors.Is(err, catalog.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundMsg)
	default:
		log.Printf("internal error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
