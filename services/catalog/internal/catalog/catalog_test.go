package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func validProduct() Product {
	return Product{
		Handle:      "cotton-kaftan",
		Title:       "Cotton Kaftan",
		Status:      StatusActive,
		ProductType: "Kaftan",
		Options: []Option{
			{Name: "Color", Values: []string{"Orange", "Teal"}},
			{Name: "Size", Values: []string{"S", "M"}},
		},
		Variants: []Variant{
			{SKU: "K-OR-S", Title: "Orange / S", SelectedOptions: map[string]string{"Color": "Orange", "Size": "S"}, PriceCents: 3499, Currency: "USD"},
			{SKU: "K-OR-M", Title: "Orange / M", SelectedOptions: map[string]string{"Color": "Orange", "Size": "M"}, PriceCents: 3999, CompareAtCents: 4999, Currency: "USD"},
		},
		Images: []Image{{URL: "/placeholders/kaftan.svg", Alt: "Kaftan"}},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Product)
		wantErr string // substring; empty means valid
	}{
		{"valid", func(p *Product) {}, ""},
		{"bad handle", func(p *Product) { p.Handle = "Not A Slug" }, "must be a lowercase slug"},
		{"empty title", func(p *Product) { p.Title = "" }, "title is required"},
		{"bad status", func(p *Product) { p.Status = "archived" }, "status"},
		{"no variants", func(p *Product) { p.Variants = nil }, "at least one variant"},
		{"duplicate sku", func(p *Product) { p.Variants[1].SKU = "K-OR-S" }, "duplicate sku"},
		{"zero price", func(p *Product) { p.Variants[0].PriceCents = 0 }, "priceCents must be positive"},
		{"compare-at below price", func(p *Product) { p.Variants[1].CompareAtCents = 100 }, "compareAtCents"},
		{"bad currency", func(p *Product) { p.Variants[0].Currency = "usd" }, "currency"},
		{"missing option value", func(p *Product) { delete(p.Variants[0].SelectedOptions, "Size") }, `missing a value for option "Size"`},
		{"unlisted option value", func(p *Product) { p.Variants[0].SelectedOptions["Color"] = "Purple" }, "not a listed value"},
		{"unknown option", func(p *Product) { p.Variants[0].SelectedOptions["Fabric"] = "Silk" }, `unknown option "Fabric"`},
		{"option without values", func(p *Product) { p.Options[0].Values = nil }, "needs a name and at least one value"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validProduct()
			tc.mutate(&p)
			err := p.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	p := validProduct()
	p.Title = ""
	p.Handle = "BAD"
	err := p.Validate()
	if err == nil || !strings.Contains(err.Error(), "title") || !strings.Contains(err.Error(), "handle") {
		t.Fatalf("want both title and handle problems, got %v", err)
	}
}

func TestCollectionValidate(t *testing.T) {
	good := Collection{Handle: "silk-edit", Title: "Silk Edit", Tag: "silk"}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid collection rejected: %v", err)
	}
	bad := Collection{Handle: "Silk Edit", Title: "", Tag: "Silk!"}
	err := bad.Validate()
	for _, want := range []string{"handle", "title", "tag"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want a %q problem reported, got %v", want, err)
		}
	}
}

func TestComputeDerived(t *testing.T) {
	p := validProduct()
	p.Variants[0].PriceCents = 5000
	p.Variants[1].PriceCents = 3999
	p.ComputeDerived()
	if p.PriceFromCents != 3999 {
		t.Errorf("PriceFromCents = %d, want the cheapest variant 3999", p.PriceFromCents)
	}

	p.Variants = nil
	p.ComputeDerived()
	if p.PriceFromCents != 0 {
		t.Errorf("no variants: PriceFromCents = %d, want 0", p.PriceFromCents)
	}
}

func TestSummaryUsesCheapestVariant(t *testing.T) {
	p := validProduct()
	p.Variants[0].PriceCents = 5000
	p.Variants[0].CompareAtCents = 0
	p.Variants[1].PriceCents = 3999
	p.Variants[1].CompareAtCents = 4999

	s := p.Summary()
	if s.PriceFromCents != 3999 || s.CompareAtFromCents != 4999 || s.Currency != "USD" {
		t.Fatalf("summary price = %+v", s)
	}
	if s.Image == nil || s.Image.URL != "/placeholders/kaftan.svg" {
		t.Fatalf("summary image = %+v", s.Image)
	}
	if s.Tags == nil {
		t.Fatal("tags should be an empty slice, not nil, so JSON is [] not null")
	}
}

func TestParseSort(t *testing.T) {
	for in, want := range map[string]Sort{
		"": SortDefault, "price_asc": SortPriceAsc, "price_desc": SortPriceDesc, "newest": SortNewest,
	} {
		got, err := ParseSort(in)
		if err != nil || got != want {
			t.Errorf("ParseSort(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"cheapest", "PRICE_ASC", "title"} {
		var ie *InvalidError
		if _, err := ParseSort(in); !errors.As(err, &ie) {
			t.Errorf("ParseSort(%q) error = %v, want *InvalidError", in, err)
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	pos := Position{Key: 3999, Handle: "f-jutti"}
	token := encodeCursor(SortPriceAsc, pos)
	if strings.ContainsAny(token, "+/=") {
		t.Errorf("token %q is not URL-safe", token)
	}

	got, err := decodeCursor(token, SortPriceAsc)
	if err != nil || got == nil || *got != pos {
		t.Fatalf("decode = %+v, %v; want %+v", got, err, pos)
	}
	if none, err := decodeCursor("", SortNewest); none != nil || err != nil {
		t.Errorf("empty cursor = %v, %v; want nil, nil", none, err)
	}

	for name, bad := range map[string]string{
		"not base64": "%%%", "not json": "bm90IGpzb24", "bad handle": encodeCursor(SortPriceAsc, Position{Handle: "Bad Handle"}),
	} {
		var ie *InvalidError
		if _, err := decodeCursor(bad, SortPriceAsc); !errors.As(err, &ie) {
			t.Errorf("%s: error = %v, want *InvalidError", name, err)
		}
	}
	var ie *InvalidError
	if _, err := decodeCursor(token, SortNewest); !errors.As(err, &ie) || !strings.Contains(err.Error(), "different sort") {
		t.Errorf("cursor reused under another sort: error = %v", err)
	}
}

// fixture makes n active products with sortable handles p001, p002, ...
func fixture(n int) *MemoryRepo {
	r := NewMemoryRepo()
	for i := 1; i <= n; i++ {
		p := validProduct()
		p.Handle = fmt.Sprintf("p%03d", i)
		p.Title = fmt.Sprintf("Product %d", i)
		p.Tags = []string{"silk"}
		_ = r.Upsert(context.Background(), p)
	}
	return r
}

func TestListRejectsBadRequests(t *testing.T) {
	svc := NewService(fixture(3))
	ctx := context.Background()
	good, err := svc.List(ctx, ListQuery{Limit: 1, Sort: SortPriceAsc})
	if err != nil || good.NextCursor == "" {
		t.Fatalf("setup: %+v, %v", good, err)
	}

	tests := []struct {
		name    string
		q       ListQuery
		wantMsg string
	}{
		{"min above max", ListQuery{Filter: Filter{MinPriceCents: 500, MaxPriceCents: 100}}, "minPrice"},
		{"negative price", ListQuery{Filter: Filter{MinPriceCents: -1}}, "negative"},
		{"too many tags", ListQuery{Filter: Filter{Tags: strings.Fields("a b c d e f g h i j k")}}, "at most"},
		{"garbage cursor", ListQuery{Cursor: "garbage!"}, "invalid cursor"},
		{"cursor from another sort", ListQuery{Sort: SortNewest, Cursor: good.NextCursor}, "different sort"},
		{"search with sort", ListQuery{Search: "silk", Sort: SortPriceAsc}, "cannot be combined"},
		{"search with cursor", ListQuery{Search: "silk", Cursor: good.NextCursor}, "cannot be combined"},
		{"search too long", ListQuery{Search: strings.Repeat("x", MaxSearchLen+1)}, "at most"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.List(ctx, tc.q)
			var ie *InvalidError
			if !errors.As(err, &ie) || !strings.Contains(ie.Msg, tc.wantMsg) {
				t.Fatalf("error = %v, want *InvalidError containing %q", err, tc.wantMsg)
			}
		})
	}
}

func TestFilterIsNormalized(t *testing.T) {
	svc := NewService(fixture(4))
	page, err := svc.List(context.Background(), ListQuery{Filter: Filter{Tags: []string{" SILK ", "silk", "  "}, ProductType: " Kaftan "}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 4 {
		t.Errorf("messy tags/type should be trimmed, lowercased (tags) and de-duplicated; got %d items", len(page.Items))
	}
}

func TestListLimits(t *testing.T) {
	svc := NewService(fixture(150))
	ctx := context.Background()

	def, _ := svc.List(ctx, ListQuery{})
	if len(def.Items) != DefaultLimit || def.NextCursor == "" {
		t.Errorf("default limit gave %d items (cursor %q), want %d with a next page", len(def.Items), def.NextCursor, DefaultLimit)
	}
	big, _ := svc.List(ctx, ListQuery{Limit: 100000})
	if len(big.Items) != MaxLimit {
		t.Errorf("huge limit gave %d items, want clamp to %d", len(big.Items), MaxLimit)
	}
	found, err := svc.List(ctx, ListQuery{Search: "product", Limit: 100000})
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Items) != MaxSearchResults || found.NextCursor != "" {
		t.Errorf("search gave %d items (cursor %q), want %d and no cursor", len(found.Items), found.NextCursor, MaxSearchResults)
	}
}

func TestServiceHidesDrafts(t *testing.T) {
	repo := fixture(2)
	draft := validProduct()
	draft.Handle = "p003"
	draft.Status = StatusDraft
	_ = repo.Upsert(context.Background(), draft)
	svc := NewService(repo)
	ctx := context.Background()

	page, _ := svc.List(ctx, ListQuery{Limit: 10})
	if len(page.Items) != 2 {
		t.Errorf("list shows %d items, want 2 (draft hidden)", len(page.Items))
	}
	if _, err := svc.Get(ctx, "p003"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(draft) error = %v, want ErrNotFound", err)
	}
}

func TestServiceGet(t *testing.T) {
	svc := NewService(fixture(2))
	ctx := context.Background()

	p, err := svc.Get(ctx, "p002")
	if err != nil || p.Handle != "p002" || len(p.Variants) != 2 {
		t.Fatalf("Get(p002) = %+v, %v", p.Handle, err)
	}
	for _, h := range []string{"missing", "Not A Handle", ""} {
		if _, err := svc.Get(ctx, h); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) error = %v, want ErrNotFound", h, err)
		}
	}
}
