package catalogclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestVariants(t *testing.T) {
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		if r.URL.Path != "/variants" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"sku":"A-1","productHandle":"a","productTitle":"Alpha","title":"Red / S",
			"priceCents":4500,"compareAtCents":5999,"currency":"USD","image":{"url":"/p.svg","alt":"Alpha"}}]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL+"/").Variants(context.Background(), []string{"A-1", "B-2"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"A-1", "B-2"}; !reflect.DeepEqual(gotQuery["sku"], want) {
		t.Errorf("sku params = %v, want %v", gotQuery["sku"], want)
	}
	if len(got) != 1 || got[0].SKU != "A-1" || got[0].ProductTitle != "Alpha" || got[0].PriceCents != 4500 ||
		got[0].CompareAtCents != 5999 || got[0].Image == nil || got[0].Image.URL != "/p.svg" {
		t.Errorf("got %+v", got)
	}
}

func TestVariantsErrors(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"server error": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		"bad request":  func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) },
		"garbage body": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not json")) },
	}
	for name, h := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			if _, err := New(srv.URL).Variants(context.Background(), []string{"A-1"}); err == nil {
				t.Error("expected an error")
			}
		})
	}

	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close()
		if _, err := New(url).Variants(context.Background(), []string{"A-1"}); err == nil {
			t.Error("expected an error")
		}
	})
}
