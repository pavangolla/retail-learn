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

// fixture makes n active products with sortable handles p01, p02, ...
func fixture(n int) *MemoryRepo {
	r := NewMemoryRepo()
	for i := 1; i <= n; i++ {
		p := validProduct()
		p.Handle = fmt.Sprintf("p%02d", i)
		r.Upsert(p)
	}
	return r
}

func TestServiceListPagination(t *testing.T) {
	svc := NewService(fixture(7))
	ctx := context.Background()

	var seen []string
	after := ""
	pages := 0
	for {
		page, err := svc.List(ctx, 3, after)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, it := range page.Items {
			seen = append(seen, it.Handle)
		}
		if page.NextCursor == "" {
			break
		}
		after = page.NextCursor
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
	}

	if pages != 3 {
		t.Errorf("pages = %d, want 3 (3+3+1)", pages)
	}
	want := []string{"p01", "p02", "p03", "p04", "p05", "p06", "p07"}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Errorf("seen = %v, want %v (no gaps, no repeats)", seen, want)
	}
}

func TestServiceListExactMultipleHasNoExtraPage(t *testing.T) {
	svc := NewService(fixture(6))
	page, err := svc.List(context.Background(), 3, "p03")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 || page.NextCursor != "" {
		t.Fatalf("last page = %d items, cursor %q; want 3 items and no cursor", len(page.Items), page.NextCursor)
	}
}

func TestServiceListLimitClamping(t *testing.T) {
	svc := NewService(fixture(150))
	ctx := context.Background()

	def, _ := svc.List(ctx, 0, "")
	if len(def.Items) != DefaultLimit {
		t.Errorf("default limit gave %d items, want %d", len(def.Items), DefaultLimit)
	}
	big, _ := svc.List(ctx, 100000, "")
	if len(big.Items) != MaxLimit {
		t.Errorf("huge limit gave %d items, want clamp to %d", len(big.Items), MaxLimit)
	}
}

func TestServiceHidesDrafts(t *testing.T) {
	repo := fixture(2)
	draft := validProduct()
	draft.Handle = "p03"
	draft.Status = StatusDraft
	repo.Upsert(draft)
	svc := NewService(repo)
	ctx := context.Background()

	page, _ := svc.List(ctx, 10, "")
	if len(page.Items) != 2 {
		t.Errorf("list shows %d items, want 2 (draft hidden)", len(page.Items))
	}
	if _, err := svc.Get(ctx, "p03"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(draft) error = %v, want ErrNotFound", err)
	}
}

func TestServiceGet(t *testing.T) {
	svc := NewService(fixture(2))
	ctx := context.Background()

	p, err := svc.Get(ctx, "p02")
	if err != nil || p.Handle != "p02" || len(p.Variants) != 2 {
		t.Fatalf("Get(p02) = %+v, %v", p.Handle, err)
	}
	for _, h := range []string{"missing", "Not A Handle", ""} {
		if _, err := svc.Get(ctx, h); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) error = %v, want ErrNotFound", h, err)
		}
	}
}
