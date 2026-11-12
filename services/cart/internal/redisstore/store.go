// Package redisstore implements cart.Store on Redis. It is the only package in
// the service that imports the Redis client.
//
// Data layout:
//
//	cart:{<id>}          hash   field = SKU, value = quantity   (expires after cart.CartTTL)
//	idem:{<id>}:<key>    string marker that an Add with this key was applied (expires after cart.IdempotencyTTL)
//
// The {<id>} part is a "hash tag": in Redis Cluster only the text inside the
// braces decides which node holds the key, so a cart and its markers always
// live together, and one script can touch both.
package redisstore

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
)

//go:embed apply.lua
var applyLua string

// Result codes returned by apply.lua.
const (
	statusApplied       = 0
	statusReplayed      = 1
	statusTooManyLines  = 2
	statusQuantityLimit = 3
)

// Store is a cart.Store backed by Redis.
type Store struct {
	client *redis.Client
	apply  *redis.Script
}

var _ cart.Store = (*Store)(nil)

// Connect opens a client from a URL like redis://localhost:6379 and checks that
// the server answers.
func Connect(ctx context.Context, url string) (*Store, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis at %s: %w", opts.Addr, err)
	}
	return &Store{client: client, apply: redis.NewScript(applyLua)}, nil
}

// Close releases the connection pool.
func (s *Store) Close() error { return s.client.Close() }

// Ping reports whether Redis is reachable. The /health endpoint uses it.
func (s *Store) Ping(ctx context.Context) error { return s.client.Ping(ctx).Err() }

func cartKey(id string) string { return "cart:{" + id + "}" }

func idemKey(id, key string) string { return "idem:{" + id + "}:" + key }

// run executes apply.lua. The script does the whole change (limit checks, the
// write, the expiry, the idempotency marker) as one atomic step.
func (s *Store) run(ctx context.Context, mode, cartID, sku string, qty int, key string) (status int64, err error) {
	keys := []string{cartKey(cartID)}
	if key != "" {
		keys = append(keys, idemKey(cartID, key))
	}
	res, err := s.apply.Run(ctx, s.client, keys,
		mode, sku, qty,
		cart.MaxQuantity, cart.MaxLines,
		int64(cart.CartTTL.Seconds()), int64(cart.IdempotencyTTL.Seconds()),
	).Int64Slice()
	if err != nil {
		return 0, fmt.Errorf("run cart script: %w", err)
	}
	if len(res) != 2 {
		return 0, fmt.Errorf("cart script returned %d values, want 2", len(res))
	}
	return res[0], nil
}

func statusError(status int64) error {
	switch status {
	case statusTooManyLines:
		return cart.ErrTooManyLines
	case statusQuantityLimit:
		return cart.ErrQuantityLimit
	}
	return nil
}

func (s *Store) Add(ctx context.Context, cartID, sku string, qty int, key string) (bool, error) {
	status, err := s.run(ctx, "add", cartID, sku, qty, key)
	if err != nil {
		return false, err
	}
	return status == statusReplayed, statusError(status)
}

func (s *Store) Set(ctx context.Context, cartID, sku string, qty int) error {
	status, err := s.run(ctx, "set", cartID, sku, qty, "")
	if err != nil {
		return err
	}
	return statusError(status)
}

func (s *Store) Items(ctx context.Context, cartID string) (map[string]int, error) {
	raw, err := s.client.HGetAll(ctx, cartKey(cartID)).Result()
	if err != nil {
		return nil, fmt.Errorf("read cart hash: %w", err)
	}
	out := make(map[string]int, len(raw))
	for sku, v := range raw {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("cart %s: quantity of %s is %q: %w", cartID, sku, v, err)
		}
		out[sku] = n
	}
	return out, nil
}

func (s *Store) Delete(ctx context.Context, cartID string) error {
	if err := s.client.Del(ctx, cartKey(cartID)).Err(); err != nil {
		return fmt.Errorf("delete cart: %w", err)
	}
	return nil
}
