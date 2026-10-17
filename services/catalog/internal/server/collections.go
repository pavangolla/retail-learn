package server

import (
	"net/http"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

const collectionNotFound = "collection not found"

type collectionList struct {
	Items []catalog.Collection `json:"items"`
}

// listCollectionsHandler serves GET /collections.
func listCollectionsHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, err := svc.Collections(r.Context())
		if err != nil {
			writeServiceError(w, err, collectionNotFound)
			return
		}
		writeJSON(w, http.StatusOK, collectionList{Items: cs})
	}
}

// getCollectionHandler serves GET /collections/{handle}.
func getCollectionHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := svc.Collection(r.Context(), r.PathValue("handle"))
		if err != nil {
			writeServiceError(w, err, collectionNotFound)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

// collectionProductsHandler serves GET /collections/{handle}/products with the
// same parameters as GET /products.
func collectionProductsHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseListQuery(r.URL.Query())
		if err != nil {
			writeServiceError(w, err, collectionNotFound)
			return
		}
		page, err := svc.ListInCollection(r.Context(), r.PathValue("handle"), q)
		if err != nil {
			writeServiceError(w, err, collectionNotFound)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

// collectionFacetsHandler serves GET /collections/{handle}/facets.
func collectionFacetsHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := parseFilter(r.URL.Query())
		if err != nil {
			writeServiceError(w, err, collectionNotFound)
			return
		}
		facets, err := svc.CollectionFacets(r.Context(), r.PathValue("handle"), f)
		if err != nil {
			writeServiceError(w, err, collectionNotFound)
			return
		}
		writeJSON(w, http.StatusOK, facets)
	}
}
