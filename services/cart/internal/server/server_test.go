package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

type fakeCatalog struct{ err error }

func (f fakeCatalog) Variants(_ context.Context, skus []string) ([]cart.VariantInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []cart.VariantInfo
	for _, s := range skus {
		switch s {
		case "SAREE-1":
			out = append(out, cart.VariantInfo{SKU: s, ProductHandle: "saree", ProductTitle: "Silk Saree", Title: "Red / S", PriceCents: 20000, Currency: "USD"})
		case "KURTA-1":
			out = append(out, cart.VariantInfo{SKU: s, ProductHandle: "kurta", ProductTitle: "Cotton Kurta", Title: "Blue / M", PriceCents: 4500, CompareAtCents: 5999, Currency: "USD"})
		}
	}
	return out, nil
}

func newTestServer(redisErr, catalogErr error) http.Handler {
	return New(Deps{
		Redis: fakePinger{err: redisErr},
		Cart:  cart.NewService(cart.NewMemoryStore(), fakeCatalog{err: catalogErr}),
	})
}

// call sends a request and returns the recorder.
func call(h http.Handler, method, target string, body any, headers ...string) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, target, rd)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// expect asserts the status and decodes the JSON body into v (if given).
func expect(t *testing.T, rec *httptest.ResponseRecorder, wantCode int, v any) {
	t.Helper()
	if rec.Code != wantCode {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, wantCode, rec.Body)
	}
	if v != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("bad JSON: %v\n%s", err, rec.Body)
		}
	}
}

func newCart(t *testing.T, h http.Handler) string {
	t.Helper()
	var res struct{ ID string }
	expect(t, call(h, "POST", "/carts", nil), http.StatusCreated, &res)
	if !cart.ValidID(res.ID) {
		t.Fatalf("id %q is not a valid cart id", res.ID)
	}
	return res.ID
}

func TestHealth(t *testing.T) {
	var body healthResponse
	expect(t, call(newTestServer(nil, nil), "GET", "/health", nil), http.StatusOK, &body)
	if body.Status != "ok" || body.Service != "cart" || body.Redis != "up" {
		t.Errorf("body = %+v", body)
	}
	expect(t, call(newTestServer(errors.New("down"), nil), "GET", "/health", nil), http.StatusServiceUnavailable, &body)
	if body.Status != "degraded" || body.Redis != "down" {
		t.Errorf("degraded body = %+v", body)
	}
}

func TestCartLifecycleOverHTTP(t *testing.T) {
	h := newTestServer(nil, nil)
	id := newCart(t, h)
	base := "/carts/" + id

	var v cart.View
	expect(t, call(h, "GET", base, nil), http.StatusOK, &v)
	if len(v.Lines) != 0 || v.ID != id {
		t.Fatalf("new cart = %+v", v)
	}

	rec := call(h, "POST", base+"/items", map[string]any{"sku": "KURTA-1", "quantity": 2}, "Idempotency-Key", "key-aaaa-0001")
	expect(t, rec, http.StatusOK, &v)
	if v.SubtotalCents != 9000 || v.ItemCount != 2 || v.Lines[0].CompareAtCents != 5999 {
		t.Fatalf("after add = %+v", v)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("cart responses must not be cacheable")
	}

	// The same request again (a retry) changes nothing and says so.
	rec = call(h, "POST", base+"/items", map[string]any{"sku": "KURTA-1", "quantity": 2}, "Idempotency-Key", "key-aaaa-0001")
	expect(t, rec, http.StatusOK, &v)
	if rec.Header().Get("Idempotent-Replayed") != "true" || v.ItemCount != 2 {
		t.Errorf("retry: replayed=%q count=%d", rec.Header().Get("Idempotent-Replayed"), v.ItemCount)
	}

	expect(t, call(h, "PUT", base+"/items/KURTA-1", map[string]int{"quantity": 5}), http.StatusOK, &v)
	if v.ItemCount != 5 || v.SubtotalCents != 22500 {
		t.Errorf("after put = %+v", v)
	}
	expect(t, call(h, "PUT", base+"/items/SAREE-1", map[string]int{"quantity": 1}), http.StatusOK, &v)
	if len(v.Lines) != 2 {
		t.Errorf("lines = %d, want 2", len(v.Lines))
	}
	expect(t, call(h, "DELETE", base+"/items/KURTA-1", nil), http.StatusOK, &v)
	if len(v.Lines) != 1 || v.Lines[0].SKU != "SAREE-1" {
		t.Errorf("after delete item = %+v", v.Lines)
	}

	expect(t, call(h, "DELETE", base, nil), http.StatusNoContent, nil)
	expect(t, call(h, "GET", base, nil), http.StatusOK, &v)
	if len(v.Lines) != 0 {
		t.Errorf("after clear = %+v", v.Lines)
	}
}

func TestBadRequests(t *testing.T) {
	h := newTestServer(nil, nil)
	id := newCart(t, h)
	idem := []string{"Idempotency-Key", "key-aaaa-0001"}
	tests := []struct {
		name         string
		method, path string
		body         any
		headers      []string
	}{
		{"malformed cart id", "GET", "/carts/not-a-real-id", nil, nil},
		{"id with path tricks", "GET", "/carts/..%2f..%2fetc", nil, nil},
		{"add without idempotency key", "POST", "/carts/" + id + "/items", map[string]any{"sku": "KURTA-1", "quantity": 1}, nil},
		{"add zero quantity", "POST", "/carts/" + id + "/items", map[string]any{"sku": "KURTA-1", "quantity": 0}, idem},
		{"add too many", "POST", "/carts/" + id + "/items", map[string]any{"sku": "KURTA-1", "quantity": 11}, idem},
		{"add unknown sku", "POST", "/carts/" + id + "/items", map[string]any{"sku": "GHOST-1", "quantity": 1}, idem},
		{"add with a price field", "POST", "/carts/" + id + "/items", map[string]any{"sku": "KURTA-1", "quantity": 1, "priceCents": 1}, idem},
		{"add not json", "POST", "/carts/" + id + "/items", "sku=KURTA-1", idem},
		{"add two documents", "POST", "/carts/" + id + "/items", `{"sku":"KURTA-1","quantity":1}{"x":1}`, idem},
		{"add empty body", "POST", "/carts/" + id + "/items", "", idem},
		{"put negative", "PUT", "/carts/" + id + "/items/KURTA-1", map[string]int{"quantity": -1}, nil},
		{"put unknown sku", "PUT", "/carts/" + id + "/items/GHOST-1", map[string]int{"quantity": 1}, nil},
		{"put bad sku", "PUT", "/carts/" + id + "/items/a%20b", map[string]int{"quantity": 1}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := call(h, tc.method, tc.path, tc.body, tc.headers...)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
			}
			var e errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error == "" {
				t.Errorf("want a JSON error body, got %s", rec.Body)
			}
		})
	}
}

func TestLimitsReturnConflict(t *testing.T) {
	h := newTestServer(nil, nil)
	id := newCart(t, h)
	url := "/carts/" + id + "/items"
	expect(t, call(h, "POST", url, map[string]any{"sku": "KURTA-1", "quantity": 10}, "Idempotency-Key", "key-aaaa-0001"), http.StatusOK, nil)
	rec := call(h, "POST", url, map[string]any{"sku": "KURTA-1", "quantity": 1}, "Idempotency-Key", "key-aaaa-0002")
	expect(t, rec, http.StatusConflict, nil)
	if !strings.Contains(rec.Body.String(), "quantity limit") {
		t.Errorf("body = %s", rec.Body)
	}
}

func TestCatalogDownIs503(t *testing.T) {
	h := newTestServer(nil, errors.New("connection refused"))
	id := newCart(t, h)
	rec := call(h, "POST", "/carts/"+id+"/items", map[string]any{"sku": "KURTA-1", "quantity": 1}, "Idempotency-Key", "key-aaaa-0001")
	expect(t, rec, http.StatusServiceUnavailable, nil)
	if strings.Contains(rec.Body.String(), "refused") {
		t.Errorf("internal detail leaked to the client: %s", rec.Body)
	}
}

func TestMethodsAndUnknownRoutes(t *testing.T) {
	h := newTestServer(nil, nil)
	id := newCart(t, h)
	expect(t, call(h, "POST", "/health", nil), http.StatusMethodNotAllowed, nil)
	expect(t, call(h, "PATCH", "/carts/"+id, nil), http.StatusMethodNotAllowed, nil)
	expect(t, call(h, "GET", "/nope", nil), http.StatusNotFound, nil)
}

func TestRecoverPanics(t *testing.T) {
	h := logRequests(recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })))
	expect(t, call(h, "GET", "/", nil), http.StatusInternalServerError, nil)
}

// A cart id is a credential: the request log must show the route, not the id.
func TestLogsDoNotContainCartIDs(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	h := newTestServer(nil, nil)
	id := newCart(t, h)
	call(h, "GET", "/carts/"+id, nil)

	out := buf.String()
	if strings.Contains(out, id) {
		t.Errorf("log leaked the cart id:\n%s", out)
	}
	if !strings.Contains(out, "GET /carts/{id} -> 200") {
		t.Errorf("log should name the route, got:\n%s", out)
	}
}
