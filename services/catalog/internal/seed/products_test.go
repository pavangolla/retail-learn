package seed

import (
	"reflect"
	"testing"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

func TestProductsAreValid(t *testing.T) {
	for _, p := range Products() {
		if err := p.Validate(); err != nil {
			t.Errorf("%s: %v", p.Handle, err)
		}
	}
}

func TestProductsAreUnique(t *testing.T) {
	handles := map[string]bool{}
	skus := map[string]bool{}
	for _, p := range Products() {
		if handles[p.Handle] {
			t.Errorf("duplicate handle %q", p.Handle)
		}
		handles[p.Handle] = true
		for _, v := range p.Variants {
			if skus[v.SKU] {
				t.Errorf("duplicate sku %q (SKUs must be unique across the whole catalog)", v.SKU)
			}
			skus[v.SKU] = true
		}
	}
}

func TestCatalogShape(t *testing.T) {
	active, drafts, sale, withVariants := 0, 0, 0, 0
	types := map[string]bool{}
	for _, p := range Products() {
		if p.Status == catalog.StatusDraft {
			drafts++
			continue
		}
		active++
		types[p.ProductType] = true
		if len(p.Variants) > 1 {
			withVariants++
		}
		for _, tag := range p.Tags {
			if tag == "sale" {
				sale++
				if p.Variants[0].CompareAtCents <= p.Variants[0].PriceCents {
					t.Errorf("%s is tagged sale but has no compare-at price", p.Handle)
				}
			}
		}
	}
	if active < 50 {
		t.Errorf("active products = %d, want at least 50", active)
	}
	if drafts != 1 {
		t.Errorf("drafts = %d, want 1", drafts)
	}
	if sale == 0 || len(types) < 8 || withVariants < 40 {
		t.Errorf("catalog too uniform: sale=%d types=%d multiVariant=%d", sale, len(types), withVariants)
	}
}

func TestProductsAreDeterministic(t *testing.T) {
	if !reflect.DeepEqual(Products(), Products()) {
		t.Fatal("Products() must return identical data on every call so seeding is idempotent")
	}
}

func TestSlug(t *testing.T) {
	tests := map[string]string{
		"Cotton Kaftan Relaxation Tunic": "cotton-kaftan-relaxation-tunic",
		"  Silver & Black -- Jutti!  ":   "silver-black-jutti",
		"3pc Kurti Set":                  "3pc-kurti-set",
	}
	for in, want := range tests {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCollectionsAreValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Collections() {
		if err := c.Validate(); err != nil {
			t.Errorf("%s: %v", c.Handle, err)
		}
		if seen[c.Handle] {
			t.Errorf("duplicate collection handle %q", c.Handle)
		}
		seen[c.Handle] = true
	}
}

// A collection is a tag rule, so a collection nobody's tagged for would be an
// empty page in the storefront.
func TestEveryCollectionHasActiveProducts(t *testing.T) {
	count := map[string]int{}
	for _, p := range Products() {
		if p.Status != catalog.StatusActive {
			continue
		}
		for _, tag := range p.Tags {
			count[tag]++
		}
	}
	for _, c := range Collections() {
		if count[c.Tag] == 0 {
			t.Errorf("collection %q (tag %q) has no active products", c.Handle, c.Tag)
		}
	}
}

func TestCollectionsAreDeterministic(t *testing.T) {
	if !reflect.DeepEqual(Collections(), Collections()) {
		t.Fatal("Collections() must be identical on every call")
	}
}
