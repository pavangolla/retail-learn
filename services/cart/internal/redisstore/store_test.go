package redisstore

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
	"github.com/pavangolla/retail-learn/services/cart/internal/cart/carttest"
)

// Integration tests: they need a real Redis and skip unless REDIS_URL is set.
//
//	docker compose up -d
//	REDIS_URL=redis://localhost:6379 go test ./internal/redisstore/
func newTestStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set; skipping Redis integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// The same behavior suite the in-memory store passes. Each subtest uses a fresh
// random cart id, so nothing needs cleaning up between runs.
func TestStoreContract(t *testing.T) {
	carttest.Run(t, func(t *testing.T) cart.Store { return newTestStore(t) })
}

// The cart must really expire, and every change must renew the clock.
func TestExpiry(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id := cart.NewID()

	if _, err := s.Add(ctx, id, "A-1", 1, "expiry-key-001"); err != nil {
		t.Fatal(err)
	}
	ttl, err := s.client.TTL(ctx, cartKey(id)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 || ttl > cart.CartTTL {
		t.Errorf("cart ttl = %v, want within (0, %v]", ttl, cart.CartTTL)
	}
	ttl, err = s.client.TTL(ctx, idemKey(id, "expiry-key-001")).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 || ttl > cart.IdempotencyTTL {
		t.Errorf("idempotency marker ttl = %v, want within (0, %v]", ttl, cart.IdempotencyTTL)
	}
}

// Emptying a cart should leave no key behind (Redis removes empty hashes).
func TestEmptyCartLeavesNoKey(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id := cart.NewID()
	if err := s.Set(ctx, id, "A-1", 2); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, id, "A-1", 0); err != nil {
		t.Fatal(err)
	}
	if n, err := s.client.Exists(ctx, cartKey(id)).Result(); err != nil || n != 0 {
		t.Errorf("exists = %d, err = %v, want 0", n, err)
	}
}
