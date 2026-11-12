package cart

import (
	"context"
	"sync"
)

// MemoryStore is an in-memory Store for tests and local experiments. It keeps
// the same rules as the Redis store but never expires anything.
type MemoryStore struct {
	mu    sync.Mutex
	carts map[string]map[string]int
	done  map[string]bool // idempotency markers: cartID + "\x00" + key
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{carts: map[string]map[string]int{}, done: map[string]bool{}}
}

// apply is the shared change logic for Add and Set. The caller holds the lock.
func (m *MemoryStore) apply(cartID, sku string, target int) error {
	lines := m.carts[cartID]
	current := lines[sku]
	if target > MaxQuantity {
		return ErrQuantityLimit
	}
	if target > 0 && current == 0 && len(lines) >= MaxLines {
		return ErrTooManyLines
	}
	if target == 0 {
		delete(lines, sku)
		if len(lines) == 0 {
			delete(m.carts, cartID) // like Redis, an empty cart does not exist
		}
		return nil
	}
	if lines == nil {
		lines = map[string]int{}
		m.carts[cartID] = lines
	}
	lines[sku] = target
	return nil
}

func (m *MemoryStore) Add(_ context.Context, cartID, sku string, qty int, idemKey string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	marker := cartID + "\x00" + idemKey
	if idemKey != "" && m.done[marker] {
		return true, nil
	}
	if err := m.apply(cartID, sku, m.carts[cartID][sku]+qty); err != nil {
		return false, err
	}
	if idemKey != "" {
		m.done[marker] = true
	}
	return false, nil
}

func (m *MemoryStore) Set(_ context.Context, cartID, sku string, qty int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.apply(cartID, sku, qty)
}

func (m *MemoryStore) Items(_ context.Context, cartID string) (map[string]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]int, len(m.carts[cartID]))
	for k, v := range m.carts[cartID] {
		out[k] = v
	}
	return out, nil
}

func (m *MemoryStore) Delete(_ context.Context, cartID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.carts, cartID)
	return nil
}

func (m *MemoryStore) Ping(context.Context) error { return nil }
