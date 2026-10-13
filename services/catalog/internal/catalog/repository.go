package catalog

import (
	"context"
	"errors"
)

// ErrNotFound means no visible product matches. Handlers turn it into a 404.
var ErrNotFound = errors.New("catalog: product not found")

// ListParams controls a product listing.
type ListParams struct {
	// Limit is the maximum number of products to return.
	Limit int
	// After is a keyset cursor: return only products whose handle sorts after it.
	After string
}

// Repository is the storage the Service needs. Both the in-memory store (used
// by tests) and the MongoDB store implement it, so business logic and HTTP
// handlers never depend on a database. All methods are storefront-facing:
// they only ever return products with status "active".
type Repository interface {
	// List returns active products ordered by handle ascending.
	List(ctx context.Context, p ListParams) ([]Product, error)
	// GetByHandle returns one active product, or ErrNotFound.
	GetByHandle(ctx context.Context, handle string) (Product, error)
}
