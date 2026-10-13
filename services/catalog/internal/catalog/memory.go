package catalog

import (
	"context"
	"sort"
	"sync"
)

// MemoryRepo is an in-memory Repository. It backs the unit tests and lets
// the HTTP layer be developed without a database.
type MemoryRepo struct {
	mu       sync.RWMutex
	products map[string]Product
}

// NewMemoryRepo returns a repo preloaded with the given products.
func NewMemoryRepo(products ...Product) *MemoryRepo {
	r := &MemoryRepo{products: make(map[string]Product, len(products))}
	for _, p := range products {
		r.products[p.Handle] = p
	}
	return r
}

// Upsert inserts or replaces a product by handle.
func (r *MemoryRepo) Upsert(p Product) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.products[p.Handle] = p
}

// Ping makes MemoryRepo usable wherever a health-checkable store is expected.
func (r *MemoryRepo) Ping(context.Context) error { return nil }

func (r *MemoryRepo) List(_ context.Context, params ListParams) ([]Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handles := make([]string, 0, len(r.products))
	for h, p := range r.products {
		if p.Status == StatusActive && h > params.After {
			handles = append(handles, h)
		}
	}
	sort.Strings(handles)
	if params.Limit > 0 && len(handles) > params.Limit {
		handles = handles[:params.Limit]
	}

	out := make([]Product, 0, len(handles))
	for _, h := range handles {
		out = append(out, r.products[h])
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
