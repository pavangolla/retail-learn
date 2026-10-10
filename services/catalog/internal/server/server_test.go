package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard) // keep request logs out of test output
	m.Run()
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func product(handle, status string) catalog.Product {
	return catalog.Product{
		Handle:      handle,
		Title:       "Product " + handle,
		Status:      status,
		ProductType: "Kurta",
		Tags:        []string{"cotton"},
		Options:     []catalog.Option{{Name: "Size", Values: []string{"S", "M"}}},
		Variants: []catalog.Variant{
			{SKU: handle + "-S", Title: "S", SelectedOptions: map[string]string{"Size": "S"}, PriceCents: 4500, Currency: "USD"},
			{SKU: handle + "-M", Title: "M", SelectedOptions: map[string]string{"Size": "M"}, PriceCents: 4900, Currency: "USD"},
		},
		Images: []catalog.Image{{URL: "/placeholders/kurta.svg", Alt: handle}},
	}
}

func newTestServer(mongoErr error, products ...catalog.Product) http.Handler {
	repo := catalog.NewMemoryRepo(products...)
	return New(Deps{Mongo: fakePinger{err: mongoErr}, Catalog: catalog.NewService(repo)})
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantCode   int
		wantStatus string
		wantMongo  string
	}{
		{"mongo up", nil, http.StatusOK, "ok", "up"},
		{"mongo down", errors.New("connection refused"), http.StatusServiceUnavailable, "degraded", "down"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(newTestServer(tc.pingErr), http.MethodGet, "/health")

			if rec.Code != tc.wantCode {
				t.Fatalf("status code = %d, want %d", rec.Code, tc.wantCode)
			}
			var got healthResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("bad JSON: %v", err)
			}
			if got.Status != tc.wantStatus || got.Mongo != tc.wantMongo || got.Service != "catalog" {
				t.Fatalf("body = %+v", got)
			}
		})
	}
}

func TestListProducts(t *testing.T) {
	h := newTestServer(nil, product("b-kurta", "active"), product("a-kurta", "active"), product("c-draft", "draft"))

	rec := do(h, http.MethodGet, "/products")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	var page catalog.Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].Handle != "a-kurta" || page.Items[1].Handle != "b-kurta" {
		t.Fatalf("items = %+v, want a-kurta then b-kurta (sorted, draft hidden)", page.Items)
	}
	if page.Items[0].PriceFromCents != 4500 {
		t.Errorf("priceFromCents = %d, want 4500", page.Items[0].PriceFromCents)
	}
	// A list card must not carry the full variant list.
	if strings.Contains(rec.Body.String(), `"variants"`) {
		t.Error("list response should contain summaries, not full variants")
	}
}

func TestListProductsPaginationOverHTTP(t *testing.T) {
	var ps []catalog.Product
	for i := 1; i <= 5; i++ {
		ps = append(ps, product(fmt.Sprintf("p%d", i), "active"))
	}
	h := newTestServer(nil, ps...)

	var seen []string
	target := "/products?limit=2"
	for i := 0; i < 10; i++ {
		rec := do(h, http.MethodGet, target)
		var page catalog.Page
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		for _, it := range page.Items {
			seen = append(seen, it.Handle)
		}
		if page.NextCursor == "" {
			break
		}
		target = "/products?limit=2&after=" + page.NextCursor
	}
	if got := strings.Join(seen, ","); got != "p1,p2,p3,p4,p5" {
		t.Fatalf("walked pages and saw %s", got)
	}
}

func TestListProductsBadLimit(t *testing.T) {
	h := newTestServer(nil, product("a", "active"))
	for _, q := range []string{"limit=abc", "limit=0", "limit=-3", "limit="} {
		rec := do(h, http.MethodGet, "/products?"+q)
		want := http.StatusBadRequest
		if q == "limit=" { // empty value is treated as absent
			want = http.StatusOK
		}
		if rec.Code != want {
			t.Errorf("?%s -> %d, want %d", q, rec.Code, want)
		}
	}
}

func TestGetProduct(t *testing.T) {
	h := newTestServer(nil, product("kaftan", "active"), product("hidden", "draft"))

	rec := do(h, http.MethodGet, "/products/kaftan")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var p catalog.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Handle != "kaftan" || len(p.Variants) != 2 || p.Variants[1].PriceCents != 4900 {
		t.Fatalf("product = %+v", p)
	}

	for _, target := range []string{"/products/missing", "/products/hidden", "/products/Bad%20Handle"} {
		rec := do(h, http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", target, rec.Code)
		}
		var e errorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error == "" {
			t.Errorf("GET %s body %q is not the standard error JSON", target, rec.Body)
		}
	}
}

func TestMethodsAndUnknownRoutes(t *testing.T) {
	h := newTestServer(nil, product("a", "active"))
	if rec := do(h, http.MethodPost, "/products"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /products = %d, want 405", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/health"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /health = %d, want 405", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope = %d, want 404", rec.Code)
	}
}

func TestRecoverPanics(t *testing.T) {
	h := logRequests(recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	rec := do(h, http.MethodGet, "/")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal error") {
		t.Errorf("body = %q", rec.Body)
	}
}
