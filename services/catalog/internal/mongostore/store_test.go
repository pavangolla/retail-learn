package mongostore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog/repotest"
	"github.com/pavangolla/retail-learn/services/catalog/internal/seed"
)

// These are integration tests: they need a real MongoDB. They skip unless
// MONGO_URI is set, so plain `go test ./...` still works without Docker.
//
//	docker compose up -d
//	MONGO_URI=mongodb://localhost:27017 go test ./internal/mongostore/
func newTestStore(t *testing.T) *Store {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI not set; skipping MongoDB integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// A throwaway database per test keeps runs isolated from your dev data.
	dbName := fmt.Sprintf("catalog_test_%d", time.Now().UnixNano())
	s, err := Connect(ctx, uri, dbName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = s.client.Database(dbName).Drop(c)
		_ = s.Close(c)
	})
	if err := s.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStoreRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	products := seed.Products()
	for _, p := range products {
		if err := s.Upsert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	// Seeding twice must not create duplicates.
	for _, p := range products {
		if err := s.Upsert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	want := products[0]
	got, err := s.GetByHandle(ctx, want.Handle)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != want.Title || len(got.Variants) != len(want.Variants) ||
		got.Variants[0].PriceCents != want.Variants[0].PriceCents ||
		got.Variants[0].SelectedOptions["Size"] != want.Variants[0].SelectedOptions["Size"] ||
		!got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}

	if _, err := s.GetByHandle(ctx, "coming-soon-preview-lehenga"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("draft product should be hidden, got err=%v", err)
	}
	if _, err := s.GetByHandle(ctx, "does-not-exist"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("missing product err = %v, want ErrNotFound", err)
	}
}

// TestRepositoryContract runs the same behaviour suite the in-memory repository
// passes, against real MongoDB (in a throwaway database).
func TestRepositoryContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) repotest.Store { return newTestStore(t) })
}

// TestSeedThroughService loads the real seed data and pages through it with the
// Service, the way the HTTP layer does.
func TestSeedThroughService(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for _, p := range seed.Products() {
		if err := s.Upsert(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range seed.Collections() {
		if err := s.UpsertCollection(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	svc := catalog.NewService(s)
	total, cursor := 0, ""
	for pages := 0; pages < 20; pages++ {
		page, err := svc.List(ctx, catalog.ListQuery{Limit: 20, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		total += len(page.Items)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	active := 0
	for _, p := range seed.Products() {
		if p.Status == catalog.StatusActive {
			active++
		}
	}
	if total != active {
		t.Fatalf("paged through %d products, want %d active", total, active)
	}

	// Every seeded collection should have products in it.
	for _, c := range seed.Collections() {
		page, err := svc.ListInCollection(ctx, c.Handle, catalog.ListQuery{Limit: 1})
		if err != nil {
			t.Fatalf("collection %s: %v", c.Handle, err)
		}
		if len(page.Items) == 0 {
			t.Errorf("collection %s is empty", c.Handle)
		}
	}
}

func TestUniqueHandleIndex(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := seed.Products()[0]
	if _, err := s.products.InsertOne(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.products.InsertOne(ctx, p); err == nil {
		t.Fatal("inserting a second product with the same handle should fail on the unique index")
	}
}
