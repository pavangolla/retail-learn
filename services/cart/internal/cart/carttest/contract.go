// Package carttest is a shared test suite for cart.Store implementations. The
// in-memory store runs it in every `go test`; the Redis store runs it when
// REDIS_URL is set. Passing the same suite keeps the fake honest.
package carttest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
)

// Run checks the Store contract. Every subtest uses its own fresh cart id, so
// one shared database is enough.
func Run(t *testing.T, newStore func(t *testing.T) cart.Store) {
	t.Helper()
	s := newStore(t)
	ctx := context.Background()

	items := func(t *testing.T, id string) map[string]int {
		t.Helper()
		got, err := s.Items(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	t.Run("unknown cart is empty", func(t *testing.T) {
		if got := items(t, cart.NewID()); len(got) != 0 {
			t.Errorf("items = %v, want none", got)
		}
	})

	t.Run("add accumulates", func(t *testing.T) {
		id := cart.NewID()
		for _, qty := range []int{2, 3} {
			if _, err := s.Add(ctx, id, "A-1", qty, ""); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.Add(ctx, id, "B-1", 1, ""); err != nil {
			t.Fatal(err)
		}
		if got, want := items(t, id), map[string]int{"A-1": 5, "B-1": 1}; !reflect.DeepEqual(got, want) {
			t.Errorf("items = %v, want %v", got, want)
		}
	})

	t.Run("carts are isolated", func(t *testing.T) {
		a, b := cart.NewID(), cart.NewID()
		if _, err := s.Add(ctx, a, "A-1", 1, ""); err != nil {
			t.Fatal(err)
		}
		if got := items(t, b); len(got) != 0 {
			t.Errorf("other cart sees %v", got)
		}
	})

	t.Run("quantity limit", func(t *testing.T) {
		id := cart.NewID()
		if _, err := s.Add(ctx, id, "A-1", cart.MaxQuantity, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Add(ctx, id, "A-1", 1, ""); !errors.Is(err, cart.ErrQuantityLimit) {
			t.Errorf("add over the limit: err = %v, want ErrQuantityLimit", err)
		}
		if err := s.Set(ctx, id, "A-1", cart.MaxQuantity+1); !errors.Is(err, cart.ErrQuantityLimit) {
			t.Errorf("set over the limit: err = %v, want ErrQuantityLimit", err)
		}
		if got := items(t, id); got["A-1"] != cart.MaxQuantity {
			t.Errorf("a rejected change must leave the cart alone, got %v", got)
		}
	})

	t.Run("line limit", func(t *testing.T) {
		id := cart.NewID()
		for i := 0; i < cart.MaxLines; i++ {
			if _, err := s.Add(ctx, id, fmt.Sprintf("SKU-%d", i), 1, ""); err != nil {
				t.Fatalf("line %d: %v", i, err)
			}
		}
		if _, err := s.Add(ctx, id, "ONE-TOO-MANY", 1, ""); !errors.Is(err, cart.ErrTooManyLines) {
			t.Errorf("err = %v, want ErrTooManyLines", err)
		}
		// Existing lines can still change at the cap.
		if _, err := s.Add(ctx, id, "SKU-0", 1, ""); err != nil {
			t.Errorf("adding to an existing line at the cap: %v", err)
		}
		// Removing a line frees a slot.
		if err := s.Set(ctx, id, "SKU-1", 0); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Add(ctx, id, "ONE-TOO-MANY", 1, ""); err != nil {
			t.Errorf("after removing a line: %v", err)
		}
	})

	t.Run("set and remove", func(t *testing.T) {
		id := cart.NewID()
		if err := s.Set(ctx, id, "A-1", 4); err != nil {
			t.Fatal(err)
		}
		if err := s.Set(ctx, id, "A-1", 4); err != nil { // repeating changes nothing
			t.Fatal(err)
		}
		if got := items(t, id); got["A-1"] != 4 {
			t.Errorf("items = %v", got)
		}
		if err := s.Set(ctx, id, "A-1", 0); err != nil {
			t.Fatal(err)
		}
		if err := s.Set(ctx, id, "never-there", 0); err != nil {
			t.Errorf("removing a missing line should be fine: %v", err)
		}
		if got := items(t, id); len(got) != 0 {
			t.Errorf("items = %v, want empty", got)
		}
	})

	t.Run("idempotent add", func(t *testing.T) {
		id := cart.NewID()
		replayed, err := s.Add(ctx, id, "A-1", 2, "key-00000001")
		if err != nil || replayed {
			t.Fatalf("first call: replayed=%v err=%v", replayed, err)
		}
		replayed, err = s.Add(ctx, id, "A-1", 2, "key-00000001") // a network retry
		if err != nil || !replayed {
			t.Fatalf("retry: replayed=%v err=%v, want replayed", replayed, err)
		}
		if got := items(t, id); got["A-1"] != 2 {
			t.Errorf("quantity after retry = %d, want 2 (not added twice)", got["A-1"])
		}
		if _, err := s.Add(ctx, id, "A-1", 1, "key-00000002"); err != nil { // a different key is a new request
			t.Fatal(err)
		}
		if got := items(t, id); got["A-1"] != 3 {
			t.Errorf("quantity = %d, want 3", got["A-1"])
		}
		// The same key in another cart is unrelated.
		other := cart.NewID()
		if replayed, err := s.Add(ctx, other, "A-1", 1, "key-00000001"); err != nil || replayed {
			t.Errorf("same key, other cart: replayed=%v err=%v", replayed, err)
		}
	})

	t.Run("a rejected add can be retried", func(t *testing.T) {
		id := cart.NewID()
		if _, err := s.Add(ctx, id, "A-1", cart.MaxQuantity, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Add(ctx, id, "A-1", 1, "key-retry-001"); !errors.Is(err, cart.ErrQuantityLimit) {
			t.Fatalf("err = %v", err)
		}
		if err := s.Set(ctx, id, "A-1", 1); err != nil { // make room
			t.Fatal(err)
		}
		replayed, err := s.Add(ctx, id, "A-1", 1, "key-retry-001")
		if err != nil || replayed {
			t.Errorf("retry after failure: replayed=%v err=%v, want it applied", replayed, err)
		}
		if got := items(t, id); got["A-1"] != 2 {
			t.Errorf("quantity = %d, want 2", got["A-1"])
		}
	})

	t.Run("concurrent adds do not lose updates", func(t *testing.T) {
		id := cart.NewID()
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := s.Add(ctx, id, "A-1", 1, ""); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if got := items(t, id); got["A-1"] != 8 {
			t.Errorf("quantity = %d, want 8", got["A-1"])
		}
	})

	t.Run("delete", func(t *testing.T) {
		id := cart.NewID()
		if _, err := s.Add(ctx, id, "A-1", 1, ""); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, id); err != nil {
			t.Errorf("deleting twice: %v", err)
		}
		if got := items(t, id); len(got) != 0 {
			t.Errorf("items = %v", got)
		}
	})
}
