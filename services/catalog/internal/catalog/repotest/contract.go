// Package repotest is a shared test suite for catalog.Repository
// implementations. The in-memory repo runs it in every `go test`; the MongoDB
// store runs it when MONGO_URI is set. Passing the same suite is what keeps
// the fast fake honest about how the real database behaves.
package repotest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// Store is a Repository that can also be loaded with data.
type Store interface {
	catalog.Repository
	Upsert(ctx context.Context, p catalog.Product) error
	UpsertCollection(ctx context.Context, c catalog.Collection) error
}

var t0 = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

// mk builds a valid product with one variant per price.
func mk(handle, title, ptype, desc string, tags []string, prices []int64, day int, status string) catalog.Product {
	sizes := make([]string, len(prices))
	variants := make([]catalog.Variant, len(prices))
	for i, price := range prices {
		sizes[i] = fmt.Sprintf("V%d", i+1)
		variants[i] = catalog.Variant{
			SKU:             fmt.Sprintf("%s-%d", handle, i+1),
			Title:           sizes[i],
			SelectedOptions: map[string]string{"Size": sizes[i]},
			PriceCents:      price,
			Currency:        "USD",
		}
	}
	created := t0.Add(time.Duration(day) * 24 * time.Hour)
	return catalog.Product{
		Handle: handle, Title: title, Description: desc, Vendor: "Test",
		ProductType: ptype, Tags: tags, Status: status,
		Options:   []catalog.Option{{Name: "Size", Values: sizes}},
		Variants:  variants,
		Images:    []catalog.Image{{URL: "/placeholders/x.svg", Alt: title}},
		CreatedAt: created, UpdatedAt: created,
	}
}

// Fixture returns the products and collections the suite loads. The data is
// designed to hit the tricky cases: equal prices (tie-break by handle), equal
// creation times, overlapping tags, a multi-variant product whose "from"
// price is its cheapest variant, and a draft that must stay invisible.
//
//	handle          type      from-price  created  tags
//	a-silk-saree    Saree        20000      d1     women silk wedding saree
//	b-cotton-saree  Saree         6000      d2     women cotton casual saree
//	c-silk-kurta    Kurta         9000      d3     men silk festive kurta
//	d-cotton-kurta  Kurta         4500      d4     men cotton casual kurta
//	e-sale-kurta    Kurta         4500      d5     men cotton sale kurta
//	f-jutti         Jutti         3999      d6     women footwear festive sale   (variants 3999, 5999)
//	g-jhumka        Jewelry       2499      d7     women accessories festive
//	h-dupatta       Dupatta       3999      d8     women silk accessories
//	i-lehenga       Lehenga      45000      d9     women silk wedding lehenga
//	j-tunic         Tunic         3999      d10    women linen casual
//	k-shirt         Shirt         3999      d10    men linen casual              (same time as j)
//	z-draft         Saree         1000      d11    women silk saree              (draft)
func Fixture() ([]catalog.Product, []catalog.Collection) {
	const act, draft = catalog.StatusActive, catalog.StatusDraft
	products := []catalog.Product{
		mk("a-silk-saree", "Gold Silk Saree", "Saree", "Rich silk drape for weddings", []string{"women", "silk", "wedding", "saree"}, []int64{20000, 25000}, 1, act),
		mk("b-cotton-saree", "Soft Cotton Saree", "Saree", "Light everyday drape", []string{"women", "cotton", "casual", "saree"}, []int64{6000}, 2, act),
		mk("c-silk-kurta", "Silk Festive Kurta", "Kurta", "Smooth finish for celebrations", []string{"men", "silk", "festive", "kurta"}, []int64{9000}, 3, act),
		mk("d-cotton-kurta", "Cotton Everyday Kurta", "Kurta", "Breathable daily wear", []string{"men", "cotton", "casual", "kurta"}, []int64{4500}, 4, act),
		mk("e-sale-kurta", "Cotton Sale Kurta", "Kurta", "Reduced for the season", []string{"men", "cotton", "sale", "kurta"}, []int64{4500}, 5, act),
		mk("f-jutti", "Embroidered Jutti", "Jutti", "Hand finished footwear", []string{"women", "footwear", "festive", "sale"}, []int64{3999, 5999}, 6, act),
		mk("g-jhumka", "Gold Jhumka", "Jewelry", "Antique gold earrings", []string{"women", "accessories", "festive"}, []int64{2499}, 7, act),
		mk("h-dupatta", "Silk Dupatta", "Dupatta", "Soft stole with tassels", []string{"women", "silk", "accessories"}, []int64{3999}, 8, act),
		mk("i-lehenga", "Wedding Lehenga", "Lehenga", "Heavy embroidered set", []string{"women", "silk", "wedding", "lehenga"}, []int64{45000}, 9, act),
		mk("j-tunic", "Linen Tunic", "Tunic", "Relaxed summer fit", []string{"women", "linen", "casual"}, []int64{3999}, 10, act),
		mk("k-shirt", "Linen Shirt", "Shirt", "Crisp collar and cuffs", []string{"men", "linen", "casual"}, []int64{3999}, 10, act),
		mk("z-draft", "Silk Draft Saree", "Saree", "Not ready for sale", []string{"women", "silk", "saree"}, []int64{1000}, 11, draft),
	}
	collections := []catalog.Collection{
		{Handle: "women", Title: "Women", Description: "All women's wear", Tag: "women"},
		{Handle: "silk-edit", Title: "Silk Edit", Description: "Everything in silk", Tag: "silk"},
		{Handle: "sale", Title: "Sale", Description: "Reduced prices", Tag: "sale"},
	}
	return products, collections
}

// Run loads Fixture into the (empty) store from newStore and checks the
// Repository contract through both the repository and the Service.
func Run(t *testing.T, newStore func(t *testing.T) Store) {
	t.Helper()
	ctx := context.Background()
	store := newStore(t)

	products, collections := Fixture()
	for _, p := range products {
		if err := p.Validate(); err != nil {
			t.Fatalf("fixture product %s is invalid: %v", p.Handle, err)
		}
		if err := store.Upsert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range collections {
		if err := c.Validate(); err != nil {
			t.Fatalf("fixture collection %s is invalid: %v", c.Handle, err)
		}
		if err := store.UpsertCollection(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	svc := catalog.NewService(store)

	t.Run("sorts", func(t *testing.T) { testSorts(t, svc) })
	t.Run("filters", func(t *testing.T) { testFilters(t, svc) })
	t.Run("pagination", func(t *testing.T) { testPagination(t, svc) })
	t.Run("search", func(t *testing.T) { testSearch(t, svc) })
	t.Run("facets", func(t *testing.T) { testFacets(t, svc) })
	t.Run("collections", func(t *testing.T) { testCollections(t, svc) })
	t.Run("get", func(t *testing.T) { testGet(t, svc) })
}

func handles(p catalog.Page) []string {
	out := make([]string, 0, len(p.Items))
	for _, it := range p.Items {
		out = append(out, it.Handle)
	}
	return out
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// list fetches everything for a query in one big page.
func list(t *testing.T, svc *catalog.Service, q catalog.ListQuery) []string {
	t.Helper()
	q.Limit = catalog.MaxLimit
	page, err := svc.List(context.Background(), q)
	if err != nil {
		t.Fatalf("List(%+v): %v", q, err)
	}
	return handles(page)
}

func expect(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s\n got  %v\n want %v", what, got, want)
	}
}

func testSorts(t *testing.T, svc *catalog.Service) {
	expect(t, "default order is handle ascending; draft hidden",
		list(t, svc, catalog.ListQuery{}),
		[]string{"a-silk-saree", "b-cotton-saree", "c-silk-kurta", "d-cotton-kurta", "e-sale-kurta", "f-jutti", "g-jhumka", "h-dupatta", "i-lehenga", "j-tunic", "k-shirt"})

	expect(t, "price_asc: cheapest first; ties by handle; f-jutti sorts by its cheapest variant",
		list(t, svc, catalog.ListQuery{Sort: catalog.SortPriceAsc}),
		[]string{"g-jhumka", "f-jutti", "h-dupatta", "j-tunic", "k-shirt", "d-cotton-kurta", "e-sale-kurta", "b-cotton-saree", "c-silk-kurta", "a-silk-saree", "i-lehenga"})

	expect(t, "price_desc: priciest first; ties still by handle ascending",
		list(t, svc, catalog.ListQuery{Sort: catalog.SortPriceDesc}),
		[]string{"i-lehenga", "a-silk-saree", "c-silk-kurta", "b-cotton-saree", "d-cotton-kurta", "e-sale-kurta", "f-jutti", "h-dupatta", "j-tunic", "k-shirt", "g-jhumka"})

	expect(t, "newest: latest first; equal timestamps by handle",
		list(t, svc, catalog.ListQuery{Sort: catalog.SortNewest}),
		[]string{"j-tunic", "k-shirt", "i-lehenga", "h-dupatta", "g-jhumka", "f-jutti", "e-sale-kurta", "d-cotton-kurta", "c-silk-kurta", "b-cotton-saree", "a-silk-saree"})
}

func testFilters(t *testing.T, svc *catalog.Service) {
	tests := []struct {
		name   string
		filter catalog.Filter
		want   []string
	}{
		{"by type (draft saree hidden)", catalog.Filter{ProductType: "Saree"}, []string{"a-silk-saree", "b-cotton-saree"}},
		{"by tag (draft silk hidden)", catalog.Filter{Tags: []string{"silk"}}, []string{"a-silk-saree", "c-silk-kurta", "h-dupatta", "i-lehenga"}},
		{"tags combine with AND", catalog.Filter{Tags: []string{"silk", "wedding"}}, []string{"a-silk-saree", "i-lehenga"}},
		{"tags with no overlap", catalog.Filter{Tags: []string{"silk", "cotton"}}, []string{}},
		{"unknown tag", catalog.Filter{Tags: []string{"nonexistent"}}, []string{}},
		{"min price is inclusive", catalog.Filter{MinPriceCents: 4500}, []string{"a-silk-saree", "b-cotton-saree", "c-silk-kurta", "d-cotton-kurta", "e-sale-kurta", "i-lehenga"}},
		{"max price is inclusive", catalog.Filter{MaxPriceCents: 4500}, []string{"d-cotton-kurta", "e-sale-kurta", "f-jutti", "g-jhumka", "h-dupatta", "j-tunic", "k-shirt"}},
		{"price range", catalog.Filter{MinPriceCents: 3999, MaxPriceCents: 4500}, []string{"d-cotton-kurta", "e-sale-kurta", "f-jutti", "h-dupatta", "j-tunic", "k-shirt"}},
		{"type and tag", catalog.Filter{ProductType: "Kurta", Tags: []string{"cotton"}}, []string{"d-cotton-kurta", "e-sale-kurta"}},
		{"type, tag and price", catalog.Filter{ProductType: "Kurta", Tags: []string{"men"}, MaxPriceCents: 5000}, []string{"d-cotton-kurta", "e-sale-kurta"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expect(t, "filter result", list(t, svc, catalog.ListQuery{Filter: tc.filter}), tc.want)
		})
	}

	expect(t, "filter and sort together",
		list(t, svc, catalog.ListQuery{Filter: catalog.Filter{Tags: []string{"women"}}, Sort: catalog.SortPriceAsc}),
		[]string{"g-jhumka", "f-jutti", "h-dupatta", "j-tunic", "b-cotton-saree", "a-silk-saree", "i-lehenga"})
}

// Walking page by page must produce exactly the unpaged order: no gaps, no
// repeats, correct handling of ties, for every sort.
func testPagination(t *testing.T, svc *catalog.Service) {
	ctx := context.Background()
	queries := []catalog.ListQuery{
		{},
		{Sort: catalog.SortPriceAsc},
		{Sort: catalog.SortPriceDesc},
		{Sort: catalog.SortNewest},
		{Sort: catalog.SortPriceDesc, Filter: catalog.Filter{Tags: []string{"women"}}},
		{Sort: catalog.SortNewest, Filter: catalog.Filter{MaxPriceCents: 4500}},
	}
	for _, base := range queries {
		for _, size := range []int{1, 2, 3, 5} {
			name := fmt.Sprintf("sort=%q tags=%v size=%d", base.Sort, base.Filter.Tags, size)
			t.Run(name, func(t *testing.T) {
				want := list(t, svc, base)

				var got []string
				q := base
				q.Limit = size
				for i := 0; i < 50; i++ {
					page, err := svc.List(ctx, q)
					if err != nil {
						t.Fatal(err)
					}
					if len(page.Items) > size {
						t.Fatalf("page has %d items, limit %d", len(page.Items), size)
					}
					got = append(got, handles(page)...)
					if page.NextCursor == "" {
						break
					}
					q.Cursor = page.NextCursor
				}
				expect(t, "paged walk equals unpaged order", got, want)
			})
		}
	}
}

func testSearch(t *testing.T, svc *catalog.Service) {
	ctx := context.Background()
	search := func(q string, f catalog.Filter, limit int) []string {
		t.Helper()
		page, err := svc.List(ctx, catalog.ListQuery{Search: q, Filter: f, Limit: limit})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		if page.NextCursor != "" {
			t.Errorf("search %q returned a cursor; search is not paged", q)
		}
		return handles(page)
	}

	silk := search("silk", catalog.Filter{}, 0)
	if !sameSet(silk, []string{"a-silk-saree", "c-silk-kurta", "h-dupatta", "i-lehenga"}) {
		t.Errorf(`search "silk" = %v, want a, c, h, i (draft excluded)`, silk)
	}
	if last := silk[len(silk)-1]; last != "i-lehenga" {
		t.Errorf(`search "silk": last = %s; i-lehenga matches only via tags, so title matches should rank first`, last)
	}

	if got := search("SILK", catalog.Filter{}, 0); !sameSet(got, silk) {
		t.Errorf(`search "SILK" = %v, want the same as lowercase`, got)
	}
	if got := search("gold", catalog.Filter{}, 0); !sameSet(got, []string{"a-silk-saree", "g-jhumka"}) {
		t.Errorf(`search "gold" = %v, want a, g`, got)
	}
	if got := search("jhumka dupatta", catalog.Filter{}, 0); !sameSet(got, []string{"g-jhumka", "h-dupatta"}) {
		t.Errorf(`search "jhumka dupatta" = %v, want either word to match`, got)
	}
	if got := search("silk", catalog.Filter{Tags: []string{"wedding"}}, 0); !sameSet(got, []string{"a-silk-saree", "i-lehenga"}) {
		t.Errorf(`search "silk" within tag wedding = %v, want a, i`, got)
	}
	if got := search("silk", catalog.Filter{}, 2); len(got) != 2 {
		t.Errorf(`search "silk" limit 2 returned %d results`, len(got))
	}
	if got := search("zzzzqqq", catalog.Filter{}, 0); len(got) != 0 {
		t.Errorf("search with no matches = %v, want none", got)
	}
	if got := search("draft", catalog.Filter{}, 0); len(got) != 0 {
		t.Errorf("a draft product must not be searchable, got %v", got)
	}
}

func testFacets(t *testing.T, svc *catalog.Service) {
	ctx := context.Background()
	count := func(fc []catalog.FacetCount, v string) int {
		for _, c := range fc {
			if c.Value == v {
				return c.Count
			}
		}
		return 0
	}

	all, err := svc.Facets(ctx, catalog.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 11 || all.PriceMinCents != 2499 || all.PriceMaxCents != 45000 {
		t.Errorf("all: total=%d min=%d max=%d, want 11 / 2499 / 45000", all.Total, all.PriceMinCents, all.PriceMaxCents)
	}
	if all.ProductTypes[0] != (catalog.FacetCount{Value: "Kurta", Count: 3}) ||
		all.ProductTypes[1] != (catalog.FacetCount{Value: "Saree", Count: 2}) {
		t.Errorf("type facets should lead with Kurta 3, Saree 2 (count desc), got %v", all.ProductTypes)
	}
	if len(all.ProductTypes) != 8 {
		t.Errorf("distinct types = %d, want 8", len(all.ProductTypes))
	}
	for tag, want := range map[string]int{"women": 7, "men": 4, "silk": 4, "casual": 4, "wedding": 2, "sale": 2} {
		if got := count(all.Tags, tag); got != want {
			t.Errorf("tag %q count = %d, want %d (the draft must not be counted)", tag, got, want)
		}
	}

	silk, err := svc.Facets(ctx, catalog.Filter{Tags: []string{"silk"}})
	if err != nil {
		t.Fatal(err)
	}
	if silk.Total != 4 || silk.PriceMinCents != 3999 || silk.PriceMaxCents != 45000 || count(silk.ProductTypes, "Saree") != 1 || count(silk.Tags, "men") != 1 {
		t.Errorf("silk facets = %+v", silk)
	}

	none, err := svc.Facets(ctx, catalog.Filter{Tags: []string{"nonexistent"}})
	if err != nil {
		t.Fatal(err)
	}
	if none.Total != 0 || none.PriceMinCents != 0 || none.PriceMaxCents != 0 || none.ProductTypes == nil || none.Tags == nil || len(none.Tags) != 0 {
		t.Errorf("empty facets = %+v; want zero totals and empty (non-nil) lists", none)
	}
}

func testCollections(t *testing.T, svc *catalog.Service) {
	ctx := context.Background()

	cs, err := svc.Collections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cs {
		got = append(got, c.Handle)
	}
	expect(t, "collections ordered by handle", got, []string{"sale", "silk-edit", "women"})

	c, err := svc.Collection(ctx, "silk-edit")
	if err != nil || c.Title != "Silk Edit" || c.Tag != "silk" {
		t.Fatalf("Collection(silk-edit) = %+v, %v", c, err)
	}
	if _, err := svc.Collection(ctx, "nope"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("unknown collection error = %v, want ErrNotFound", err)
	}

	page, err := svc.ListInCollection(ctx, "silk-edit", catalog.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, "products in silk-edit", handles(page), []string{"a-silk-saree", "c-silk-kurta", "h-dupatta", "i-lehenga"})

	page, err = svc.ListInCollection(ctx, "silk-edit", catalog.ListQuery{Filter: catalog.Filter{Tags: []string{"women"}}, Sort: catalog.SortPriceAsc})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, "silk-edit narrowed by tag and sorted", handles(page), []string{"h-dupatta", "a-silk-saree", "i-lehenga"})

	page, err = svc.ListInCollection(ctx, "sale", catalog.ListQuery{Search: "kurta"})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, "search inside the sale collection", handles(page), []string{"e-sale-kurta"})

	f, err := svc.CollectionFacets(ctx, "silk-edit", catalog.Filter{})
	if err != nil || f.Total != 4 {
		t.Errorf("CollectionFacets(silk-edit) = %+v, %v; want total 4", f, err)
	}

	if _, err := svc.ListInCollection(ctx, "nope", catalog.ListQuery{}); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("products of unknown collection error = %v, want ErrNotFound", err)
	}
	if _, err := svc.CollectionFacets(ctx, "nope", catalog.Filter{}); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("facets of unknown collection error = %v, want ErrNotFound", err)
	}
}

func testGet(t *testing.T, svc *catalog.Service) {
	ctx := context.Background()

	p, err := svc.Get(ctx, "f-jutti")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Embroidered Jutti" || len(p.Variants) != 2 || p.Variants[1].PriceCents != 5999 ||
		p.Variants[0].SelectedOptions["Size"] != "V1" || strings.TrimSpace(p.Images[0].URL) == "" {
		t.Errorf("round trip lost data: %+v", p)
	}
	if !p.CreatedAt.Equal(t0.Add(6 * 24 * time.Hour)) {
		t.Errorf("createdAt = %v", p.CreatedAt)
	}

	for _, h := range []string{"z-draft", "missing"} {
		if _, err := svc.Get(ctx, h); !errors.Is(err, catalog.ErrNotFound) {
			t.Errorf("Get(%q) error = %v, want ErrNotFound", h, err)
		}
	}
}
