package catalog

import (
	"context"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// MemoryRepo is an in-memory Repository. It backs the unit tests and lets the
// HTTP layer be developed without a database. It follows the same contract as
// the MongoDB store, with one deliberate simplification: its text search
// matches whole words, while MongoDB's also stems them ("sarees" finds
// "saree"). Tests that run against both use words where the two agree.
type MemoryRepo struct {
	mu          sync.RWMutex
	products    map[string]Product
	collections map[string]Collection
}

// NewMemoryRepo returns a repo preloaded with the given products.
func NewMemoryRepo(products ...Product) *MemoryRepo {
	r := &MemoryRepo{
		products:    make(map[string]Product, len(products)),
		collections: make(map[string]Collection),
	}
	for _, p := range products {
		p.ComputeDerived()
		r.products[p.Handle] = p
	}
	return r
}

// Upsert inserts or replaces a product by handle.
func (r *MemoryRepo) Upsert(_ context.Context, p Product) error {
	p.ComputeDerived()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.products[p.Handle] = p
	return nil
}

// UpsertCollection inserts or replaces a collection by handle.
func (r *MemoryRepo) UpsertCollection(_ context.Context, c Collection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collections[c.Handle] = c
	return nil
}

// Ping makes MemoryRepo usable wherever a health-checkable store is expected.
func (r *MemoryRepo) Ping(context.Context) error { return nil }

// matches reports whether p is visible and satisfies the filter.
func matches(p Product, f Filter) bool {
	if p.Status != StatusActive {
		return false
	}
	if f.ProductType != "" && p.ProductType != f.ProductType {
		return false
	}
	for _, want := range f.Tags {
		found := false
		for _, have := range p.Tags {
			if have == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.MinPriceCents > 0 && p.PriceFromCents < f.MinPriceCents {
		return false
	}
	if f.MaxPriceCents > 0 && p.PriceFromCents > f.MaxPriceCents {
		return false
	}
	return true
}

// compare orders (keyA, handleA) against (keyB, handleB) under sort s:
// negative if A comes first, positive if B does, zero if they are equal.
func compare(s Sort, keyA int64, handleA string, keyB int64, handleB string) int {
	if keyA != keyB {
		c := -1
		if keyA > keyB {
			c = 1
		}
		if s.Descending() {
			c = -c
		}
		return c
	}
	return strings.Compare(handleA, handleB)
}

func (r *MemoryRepo) List(_ context.Context, params ListParams) ([]Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []Product
	for _, p := range r.products {
		if !matches(p, params.Filter) {
			continue
		}
		if a := params.After; a != nil &&
			compare(params.Sort, SortKey(p, params.Sort), p.Handle, a.Key, a.Handle) <= 0 {
			continue // at or before the cursor
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return compare(params.Sort, SortKey(out[i], params.Sort), out[i].Handle,
			SortKey(out[j], params.Sort), out[j].Handle) < 0
	})
	if params.Limit > 0 && len(out) > params.Limit {
		out = out[:params.Limit]
	}
	return out, nil
}

// words lowercases s and splits it into letter/digit runs.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// score rates how well p matches the query words. The weights mirror the
// MongoDB text index: title counts most, then type and tags, then description.
func score(p Product, terms map[string]bool) int {
	total := 0
	count := func(text string, weight int) {
		for _, w := range words(text) {
			if terms[w] {
				total += weight
			}
		}
	}
	count(p.Title, 10)
	count(p.ProductType, 5)
	count(strings.Join(p.Tags, " "), 5)
	count(p.Description, 1)
	return total
}

func (r *MemoryRepo) Search(_ context.Context, params SearchParams) ([]Product, error) {
	terms := make(map[string]bool)
	for _, w := range words(params.Query) {
		terms[w] = true
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	type scored struct {
		p     Product
		score int
	}
	var hits []scored
	for _, p := range r.products {
		if !matches(p, params.Filter) {
			continue
		}
		if sc := score(p, terms); sc > 0 {
			hits = append(hits, scored{p, sc})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].p.Handle < hits[j].p.Handle
	})
	if params.Limit > 0 && len(hits) > params.Limit {
		hits = hits[:params.Limit]
	}
	out := make([]Product, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.p)
	}
	return out, nil
}

func (r *MemoryRepo) GetByHandle(_ context.Context, handle string) (Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.products[handle]
	if !ok || p.Status != StatusActive {
		return Product{}, ErrNotFound
	}
	return p, nil
}

func (r *MemoryRepo) Facets(_ context.Context, f Filter) (Facets, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := map[string]int{}
	tags := map[string]int{}
	var out Facets
	for _, p := range r.products {
		if !matches(p, f) {
			continue
		}
		if out.Total == 0 || p.PriceFromCents < out.PriceMinCents {
			out.PriceMinCents = p.PriceFromCents
		}
		if p.PriceFromCents > out.PriceMaxCents {
			out.PriceMaxCents = p.PriceFromCents
		}
		out.Total++
		types[p.ProductType]++
		for _, t := range p.Tags {
			tags[t]++
		}
	}
	out.ProductTypes = rank(types)
	out.Tags = rank(tags)
	return out, nil
}

// rank turns counts into a list ordered by count descending, then value.
func rank(counts map[string]int) []FacetCount {
	out := make([]FacetCount, 0, len(counts))
	for v, c := range counts {
		out = append(out, FacetCount{Value: v, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func (r *MemoryRepo) ListCollections(context.Context) ([]Collection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Collection, 0, len(r.collections))
	for _, c := range r.collections {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handle < out[j].Handle })
	return out, nil
}

func (r *MemoryRepo) GetCollection(_ context.Context, handle string) (Collection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.collections[handle]
	if !ok {
		return Collection{}, ErrNotFound
	}
	return c, nil
}
