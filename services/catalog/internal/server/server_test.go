package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog/repotest"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard) // keep request logs out of test output
	m.Run()
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

// newTestServer serves the shared contract fixture (11 active products, 1
// draft, 3 collections) from the in-memory repository.
func newTestServer(mongoErr error) http.Handler {
	repo := catalog.NewMemoryRepo()
	products, collections := repotest.Fixture()
	for _, p := range products {
		_ = repo.Upsert(context.Background(), p)
	}
	for _, c := range collections {
		_ = repo.UpsertCollection(context.Background(), c)
	}
	return New(Deps{Mongo: fakePinger{err: mongoErr}, Catalog: catalog.NewService(repo)})
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

// get requests target, asserts the status code, and decodes the JSON body into v.
func get(t *testing.T, h http.Handler, target string, wantCode int, v any) *httptest.ResponseRecorder {
	t.Helper()
	rec := do(h, http.MethodGet, target)
	if rec.Code != wantCode {
		t.Fatalf("GET %s = %d, want %d; body: %s", target, rec.Code, wantCode, rec.Body)
	}
	if v != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("GET %s: bad JSON: %v\n%s", target, err, rec.Body)
		}
	}
	return rec
}

func handles(p catalog.Page) []string {
	out := []string{}
	for _, it := range p.Items {
		out = append(out, it.Handle)
	}
	return out
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
			var got healthResponse
			get(t, newTestServer(tc.pingErr), "/health", tc.wantCode, &got)
			if got.Status != tc.wantStatus || got.Mongo != tc.wantMongo || got.Service != "catalog" {
				t.Fatalf("body = %+v", got)
			}
		})
	}
}

func TestListProducts(t *testing.T) {
	var page catalog.Page
	rec := get(t, newTestServer(nil), "/products", http.StatusOK, &page)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if len(page.Items) != 11 || page.Items[0].Handle != "a-silk-saree" || page.NextCursor != "" {
		t.Fatalf("items = %v, cursor %q; want all 11 active products from a-silk-saree, no next page", handles(page), page.NextCursor)
	}
	var jutti catalog.Summary
	for _, it := range page.Items {
		if it.Handle == "f-jutti" {
			jutti = it
		}
	}
	if jutti.PriceFromCents != 3999 {
		t.Errorf("f-jutti priceFromCents = %d, want its cheapest variant 3999", jutti.PriceFromCents)
	}
	// A list card must not carry the full variant list.
	if strings.Contains(rec.Body.String(), `"variants"`) {
		t.Error("list response should contain summaries, not full variants")
	}
}

func TestFiltersAndSortOverHTTP(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   []string
	}{
		{"type and tag", "/products?type=Kurta&tag=cotton", []string{"d-cotton-kurta", "e-sale-kurta"}},
		{"repeated tag params are ANDed, then sorted", "/products?tag=women&tag=silk&sort=price_asc", []string{"h-dupatta", "a-silk-saree", "i-lehenga"}},
		{"price range, priciest first, ties by handle", "/products?minPrice=4500&maxPrice=9000&sort=price_desc", []string{"c-silk-kurta", "b-cotton-saree", "d-cotton-kurta", "e-sale-kurta"}},
		{"empty limit is ignored", "/products?limit=&tag=sale", []string{"e-sale-kurta", "f-jutti"}},
		{"no matches", "/products?tag=nonexistent", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var page catalog.Page
			get(t, newTestServer(nil), tc.target, http.StatusOK, &page)
			if !slices.Equal(handles(page), tc.want) {
				t.Errorf("got  %v\nwant %v", handles(page), tc.want)
			}
		})
	}

	t.Run("empty result is [] not null", func(t *testing.T) {
		rec := get(t, newTestServer(nil), "/products?tag=nonexistent", http.StatusOK, nil)
		if !strings.Contains(rec.Body.String(), `"items":[]`) {
			t.Errorf("body = %s", rec.Body)
		}
	})
}

func TestPaginationOverHTTP(t *testing.T) {
	h := newTestServer(nil)
	want := []string{"g-jhumka", "f-jutti", "h-dupatta", "j-tunic", "k-shirt", "d-cotton-kurta", "e-sale-kurta", "b-cotton-saree", "c-silk-kurta", "a-silk-saree", "i-lehenga"}

	var seen []string
	target := "/products?sort=price_asc&limit=4"
	for i := 0; i < 10; i++ {
		var page catalog.Page
		get(t, h, target, http.StatusOK, &page)
		seen = append(seen, handles(page)...)
		if page.NextCursor == "" {
			break
		}
		target = "/products?sort=price_asc&limit=4&after=" + page.NextCursor
	}
	if !slices.Equal(seen, want) {
		t.Fatalf("walked pages and saw\n %v\nwant\n %v", seen, want)
	}
}

func TestSearchOverHTTP(t *testing.T) {
	h := newTestServer(nil)

	var page catalog.Page
	get(t, h, "/products?q=silk", http.StatusOK, &page)
	got := handles(page)
	slices.Sort(got)
	if !slices.Equal(got, []string{"a-silk-saree", "c-silk-kurta", "h-dupatta", "i-lehenga"}) {
		t.Errorf("q=silk -> %v", got)
	}
	if page.NextCursor != "" {
		t.Errorf("search must not be paged, got cursor %q", page.NextCursor)
	}

	get(t, h, "/products?q=silk&tag=wedding&limit=1", http.StatusOK, &page)
	if len(page.Items) != 1 {
		t.Errorf("limit=1 returned %d items", len(page.Items))
	}
	get(t, h, "/products?q=zzzzqqq", http.StatusOK, &page)
	if len(page.Items) != 0 {
		t.Errorf("no-match search returned %v", handles(page))
	}
}

func TestBadRequests(t *testing.T) {
	h := newTestServer(nil)
	for _, target := range []string{
		"/products?limit=abc",
		"/products?limit=0",
		"/products?limit=-3",
		"/products?minPrice=abc",
		"/products?minPrice=-5",
		"/products?maxPrice=1.5",
		"/products?minPrice=500&maxPrice=100",
		"/products?sort=cheapest",
		"/products?after=garbage!",
		"/products?q=silk&sort=price_asc",
		"/products?q=silk&after=abc",
		"/products?q=" + strings.Repeat("x", 101),
		"/products?tag=a&tag=b&tag=c&tag=d&tag=e&tag=f&tag=g&tag=h&tag=i&tag=j&tag=k",
		"/products/facets?minPrice=x",
		"/collections/silk-edit/products?sort=bogus",
		"/collections/silk-edit/facets?maxPrice=-1",
	} {
		t.Run(target, func(t *testing.T) {
			var e errorResponse
			get(t, h, target, http.StatusBadRequest, &e)
			if e.Error == "" {
				t.Error("400 must carry an explanatory error message")
			}
		})
	}

	// A cursor is bound to the sort that produced it.
	var page catalog.Page
	get(t, h, "/products?sort=price_asc&limit=2", http.StatusOK, &page)
	var e errorResponse
	get(t, h, "/products?sort=newest&after="+page.NextCursor, http.StatusBadRequest, &e)
	if !strings.Contains(e.Error, "different sort") {
		t.Errorf("error = %q", e.Error)
	}
}

func TestFacetsOverHTTP(t *testing.T) {
	h := newTestServer(nil)

	var all catalog.Facets
	get(t, h, "/products/facets", http.StatusOK, &all)
	if all.Total != 11 || all.PriceMinCents != 2499 || all.PriceMaxCents != 45000 || all.ProductTypes[0].Value != "Kurta" {
		t.Errorf("facets = %+v", all)
	}

	var silk catalog.Facets
	get(t, h, "/products/facets?tag=silk", http.StatusOK, &silk)
	if silk.Total != 4 {
		t.Errorf("silk facets total = %d, want 4", silk.Total)
	}

	// The literal route must win over /products/{handle}.
	rec := get(t, h, "/products/facets", http.StatusOK, nil)
	if !strings.Contains(rec.Body.String(), `"productTypes"`) {
		t.Errorf("/products/facets was handled as a product lookup: %s", rec.Body)
	}
}

func TestCollectionsOverHTTP(t *testing.T) {
	h := newTestServer(nil)

	var list collectionList
	get(t, h, "/collections", http.StatusOK, &list)
	var handlesList []string
	for _, c := range list.Items {
		handlesList = append(handlesList, c.Handle)
	}
	if !slices.Equal(handlesList, []string{"sale", "silk-edit", "women"}) {
		t.Errorf("collections = %v", handlesList)
	}

	var c catalog.Collection
	get(t, h, "/collections/silk-edit", http.StatusOK, &c)
	if c.Title != "Silk Edit" || c.Tag != "silk" {
		t.Errorf("collection = %+v", c)
	}

	var page catalog.Page
	get(t, h, "/collections/silk-edit/products?tag=women&sort=price_asc", http.StatusOK, &page)
	if !slices.Equal(handles(page), []string{"h-dupatta", "a-silk-saree", "i-lehenga"}) {
		t.Errorf("silk-edit products = %v", handles(page))
	}
	get(t, h, "/collections/sale/products?q=kurta", http.StatusOK, &page)
	if !slices.Equal(handles(page), []string{"e-sale-kurta"}) {
		t.Errorf("search in sale = %v", handles(page))
	}

	var facets catalog.Facets
	get(t, h, "/collections/silk-edit/facets", http.StatusOK, &facets)
	if facets.Total != 4 {
		t.Errorf("silk-edit facets total = %d", facets.Total)
	}

	for _, target := range []string{"/collections/nope", "/collections/nope/products", "/collections/nope/facets", "/collections/Bad%20Handle"} {
		var e errorResponse
		get(t, h, target, http.StatusNotFound, &e)
		if e.Error != "collection not found" {
			t.Errorf("GET %s error = %q", target, e.Error)
		}
	}
}

func TestGetProduct(t *testing.T) {
	h := newTestServer(nil)

	var p catalog.Product
	get(t, h, "/products/f-jutti", http.StatusOK, &p)
	if p.Handle != "f-jutti" || len(p.Variants) != 2 || p.Variants[1].PriceCents != 5999 {
		t.Fatalf("product = %+v", p)
	}

	for _, target := range []string{"/products/missing", "/products/z-draft", "/products/Bad%20Handle"} {
		var e errorResponse
		get(t, h, target, http.StatusNotFound, &e)
		if e.Error != "product not found" {
			t.Errorf("GET %s error = %q", target, e.Error)
		}
	}
}

func TestMethodsAndUnknownRoutes(t *testing.T) {
	h := newTestServer(nil)
	for _, target := range []string{"/products", "/health", "/collections", "/products/facets"} {
		if rec := do(h, http.MethodPost, target); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s = %d, want 405", target, rec.Code)
		}
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
