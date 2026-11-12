package cart

import "context"

// Store holds cart contents. There are two implementations: MemoryStore (used
// by tests) and the Redis one. Package carttest runs the same behavior checks
// against both.
//
// The methods are small and each one is atomic, because two requests for the
// same cart can arrive at the same moment (two browser tabs, a double click).
type Store interface {
	// Add increases the quantity of sku in the cart by qty (qty >= 1).
	//
	// idemKey makes the call safe to retry: if the same cart has already
	// applied a successful Add with this key, nothing changes and replayed is
	// true. An empty idemKey disables that check. A call that fails (a limit
	// is hit) is not remembered, so it can be retried later.
	//
	// It returns ErrQuantityLimit if the line would exceed MaxQuantity and
	// ErrTooManyLines if a new line would exceed MaxLines.
	Add(ctx context.Context, cartID, sku string, qty int, idemKey string) (replayed bool, err error)

	// Set replaces the quantity of sku with qty. 0 removes the line. It is
	// idempotent by nature: repeating it gives the same result.
	Set(ctx context.Context, cartID, sku string, qty int) error

	// Items returns sku -> quantity. An unknown or empty cart gives an empty map.
	Items(ctx context.Context, cartID string) (map[string]int, error)

	// Delete removes the whole cart. Deleting a missing cart is not an error.
	Delete(ctx context.Context, cartID string) error

	// Ping reports whether the backing store is reachable.
	Ping(ctx context.Context) error
}
