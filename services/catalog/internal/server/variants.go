package server

import (
	"net/http"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// variantsHandler serves
//
//	GET /variants?sku=A&sku=B
//
// It is the read-only lookup other services use to price what they hold by SKU
// (the cart today, checkout later). Unknown SKUs are left out of the answer.
func variantsHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := svc.Variants(r.Context(), r.URL.Query()["sku"])
		if err != nil {
			writeServiceError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Items []catalog.VariantInfo `json:"items"`
		}{Items: items})
	}
}
