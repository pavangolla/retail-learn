package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
)

// decodeBody reads a small JSON body strictly: unknown fields and trailing
// data are errors, and the size is capped so nobody can stream gigabytes at us.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON with the expected fields")
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain a single JSON object")
		return false
	}
	return true
}

// POST /carts -> 201 {"id": "..."}
func createCartHandler(svc *cart.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, struct {
			ID string `json:"id"`
		}{ID: svc.Create()})
	}
}

// GET /carts/{id}
func getCartHandler(svc *cart.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := svc.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

// DELETE /carts/{id} -> 204
func clearCartHandler(svc *cart.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Clear(r.Context(), r.PathValue("id")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type addItemRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// POST /carts/{id}/items   {"sku": "...", "quantity": 1}   + Idempotency-Key header
//
// "Add" is not naturally safe to repeat (two requests add twice), so the
// Idempotency-Key header is required. A retry with the same key returns the
// current cart with `Idempotent-Replayed: true` and changes nothing.
func addItemHandler(svc *cart.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addItemRequest
		if !decodeBody(w, r, &req) {
			return
		}
		v, replayed, err := svc.Add(r.Context(), r.PathValue("id"), req.SKU, req.Quantity, r.Header.Get("Idempotency-Key"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		if replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeJSON(w, http.StatusOK, v)
	}
}

type setItemRequest struct {
	Quantity int `json:"quantity"`
}

// PUT /carts/{id}/items/{sku}   {"quantity": 3}   (0 removes the line)
//
// PUT means "make it exactly this", so repeating it is harmless and needs no
// idempotency key.
func setItemHandler(svc *cart.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req setItemRequest
		if !decodeBody(w, r, &req) {
			return
		}
		v, err := svc.SetQuantity(r.Context(), r.PathValue("id"), r.PathValue("sku"), req.Quantity)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

// DELETE /carts/{id}/items/{sku}
func removeItemHandler(svc *cart.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := svc.Remove(r.Context(), r.PathValue("id"), r.PathValue("sku"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}
