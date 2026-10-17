package catalog

import (
	"context"
	"fmt"
	"strings"
)

const (
	DefaultLimit     = 20
	MaxLimit         = 100
	MaxSearchResults = 50  // search is top-N by relevance, never paged
	MaxSearchLen     = 100 // characters in a search query
)

// Service holds the catalog's business logic: it validates requests, applies
// defaults and limits, and turns repository rows into API pages.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by repo.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Page is one page of product summaries.
type Page struct {
	Items []Summary `json:"items"`
	// NextCursor is the opaque value to pass as `after` to get the next page.
	// It is empty on the last page, and always empty for search results.
	NextCursor string `json:"nextCursor,omitempty"`
}

// ListQuery describes what a client asked for.
type ListQuery struct {
	Filter Filter
	Sort   Sort
	Limit  int    // 0 means DefaultLimit
	Cursor string // opaque token from a previous Page
	Search string // when set, results are a relevance-ranked search instead
}

// List returns one page of products.
//
// Without a search term it is a normal listing: filtered, sorted, and paged
// with keyset pagination. Instead of "skip N rows" (slower the deeper you go,
// and prone to skipping or repeating items when data changes), the client
// sends a cursor naming the last item it saw and we return the items after
// it. We ask the repository for one extra row just to learn whether another
// page exists.
//
// With a search term the result is the best matches by relevance, capped at
// MaxSearchResults, with no paging.
func (s *Service) List(ctx context.Context, q ListQuery) (Page, error) {
	q.Filter = q.Filter.normalize()
	if err := q.Filter.validate(); err != nil {
		return Page{}, err
	}
	if term := strings.TrimSpace(q.Search); term != "" {
		q.Search = term
		return s.search(ctx, q)
	}

	limit := clampLimit(q.Limit, MaxLimit)
	after, err := decodeCursor(q.Cursor, q.Sort)
	if err != nil {
		return Page{}, err
	}

	products, err := s.repo.List(ctx, ListParams{
		Filter: q.Filter,
		Sort:   q.Sort,
		Limit:  limit + 1,
		After:  after,
	})
	if err != nil {
		return Page{}, fmt.Errorf("list products: %w", err)
	}

	page := Page{Items: make([]Summary, 0, len(products))}
	if len(products) > limit {
		products = products[:limit]
		last := products[len(products)-1]
		page.NextCursor = encodeCursor(q.Sort, Position{Key: SortKey(last, q.Sort), Handle: last.Handle})
	}
	for _, p := range products {
		page.Items = append(page.Items, p.Summary())
	}
	return page, nil
}

func (s *Service) search(ctx context.Context, q ListQuery) (Page, error) {
	if q.Sort != SortDefault || q.Cursor != "" {
		return Page{}, invalidf("sort and after cannot be combined with q: search results are ordered by relevance and are not paged")
	}
	if len([]rune(q.Search)) > MaxSearchLen {
		return Page{}, invalidf("q must be at most %d characters", MaxSearchLen)
	}

	products, err := s.repo.Search(ctx, SearchParams{
		Query:  q.Search,
		Filter: q.Filter,
		Limit:  clampLimit(q.Limit, MaxSearchResults),
	})
	if err != nil {
		return Page{}, fmt.Errorf("search products: %w", err)
	}
	page := Page{Items: make([]Summary, 0, len(products))}
	for _, p := range products {
		page.Items = append(page.Items, p.Summary())
	}
	return page, nil
}

func clampLimit(limit, max int) int {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > max {
		limit = max
	}
	return limit
}

// Get returns the full product (all variants and images) for a handle.
func (s *Service) Get(ctx context.Context, handle string) (Product, error) {
	if !ValidHandle(handle) {
		return Product{}, ErrNotFound
	}
	p, err := s.repo.GetByHandle(ctx, handle)
	if err != nil {
		return Product{}, err // keep ErrNotFound identity for errors.Is
	}
	return p, nil
}

// Facets summarizes the products matching a filter, for building filter UIs.
func (s *Service) Facets(ctx context.Context, f Filter) (Facets, error) {
	f = f.normalize()
	if err := f.validate(); err != nil {
		return Facets{}, err
	}
	out, err := s.repo.Facets(ctx, f)
	if err != nil {
		return Facets{}, fmt.Errorf("facets: %w", err)
	}
	if out.ProductTypes == nil {
		out.ProductTypes = []FacetCount{}
	}
	if out.Tags == nil {
		out.Tags = []FacetCount{}
	}
	return out, nil
}

// Collections lists every collection.
func (s *Service) Collections(ctx context.Context) ([]Collection, error) {
	cs, err := s.repo.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	if cs == nil {
		cs = []Collection{}
	}
	return cs, nil
}

// Collection returns one collection by handle.
func (s *Service) Collection(ctx context.Context, handle string) (Collection, error) {
	if !ValidHandle(handle) {
		return Collection{}, ErrNotFound
	}
	return s.repo.GetCollection(ctx, handle)
}

// ListInCollection is List restricted to the products in a collection: it adds
// the collection's tag to whatever filter the client supplied.
func (s *Service) ListInCollection(ctx context.Context, handle string, q ListQuery) (Page, error) {
	c, err := s.Collection(ctx, handle)
	if err != nil {
		return Page{}, err
	}
	q.Filter = withTag(q.Filter, c.Tag)
	return s.List(ctx, q)
}

// CollectionFacets is Facets restricted to the products in a collection.
func (s *Service) CollectionFacets(ctx context.Context, handle string, f Filter) (Facets, error) {
	c, err := s.Collection(ctx, handle)
	if err != nil {
		return Facets{}, err
	}
	return s.Facets(ctx, withTag(f, c.Tag))
}

func withTag(f Filter, tag string) Filter {
	tags := make([]string, 0, len(f.Tags)+1)
	tags = append(tags, f.Tags...)
	f.Tags = append(tags, tag)
	return f
}
