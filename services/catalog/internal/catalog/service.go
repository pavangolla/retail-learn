package catalog

import (
	"context"
	"fmt"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Service holds the catalog's business logic. It is deliberately thin in
// Phase 1; filters, search and collections arrive in slice 1b.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by repo.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Page is one page of product summaries.
type Page struct {
	Items []Summary `json:"items"`
	// NextCursor is the value to pass as `after` to get the next page.
	// It is empty on the last page.
	NextCursor string `json:"nextCursor,omitempty"`
}

// List returns one page of products in handle order.
//
// Keyset pagination: instead of "skip N rows" (which gets slower the deeper
// you go and can skip or repeat items when data changes), the client sends
// the last handle it saw and we return the rows after it. We ask the
// repository for one extra row just to learn whether another page exists.
func (s *Service) List(ctx context.Context, limit int, after string) (Page, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	products, err := s.repo.List(ctx, ListParams{Limit: limit + 1, After: after})
	if err != nil {
		return Page{}, fmt.Errorf("list products: %w", err)
	}

	page := Page{Items: make([]Summary, 0, len(products))}
	if len(products) > limit {
		products = products[:limit]
		page.NextCursor = products[len(products)-1].Handle
	}
	for _, p := range products {
		page.Items = append(page.Items, p.Summary())
	}
	return page, nil
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
