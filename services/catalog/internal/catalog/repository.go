package catalog

import (
	"context"
	"errors"
)

// ErrNotFound means no visible product or collection matches.
// Handlers turn it into a 404.
var ErrNotFound = errors.New("catalog: not found")

// Repository is the storage the Service needs. Both the in-memory store (used
// by tests) and the MongoDB store implement it, so business logic and HTTP
// handlers never depend on a database. Product methods are storefront-facing:
// they only ever return products with status "active".
//
// The shared contract tests in package repotest hold every implementation to
// the same behavior.
type Repository interface {
	// List returns one page of active products matching the filter, in the
	// order given by params.Sort, starting after params.After.
	List(ctx context.Context, p ListParams) ([]Product, error)
	// Search returns active products matching the text query, best match
	// first, up to params.Limit. There is no paging.
	Search(ctx context.Context, p SearchParams) ([]Product, error)
	// GetByHandle returns one active product, or ErrNotFound.
	GetByHandle(ctx context.Context, handle string) (Product, error)
	// Facets summarizes the active products that match the filter.
	Facets(ctx context.Context, f Filter) (Facets, error)

	// ListCollections returns every collection ordered by handle.
	ListCollections(ctx context.Context) ([]Collection, error)
	// GetCollection returns one collection, or ErrNotFound.
	GetCollection(ctx context.Context, handle string) (Collection, error)
}
