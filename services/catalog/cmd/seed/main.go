// Command seed loads the sample catalog into MongoDB. It is safe to run
// repeatedly: products are upserted by handle.
//
//	go run ./cmd/seed
package main

import (
	"context"
	"log"
	"time"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
	"github.com/pavangolla/retail-learn/services/catalog/internal/config"
	"github.com/pavangolla/retail-learn/services/catalog/internal/mongostore"
	"github.com/pavangolla/retail-learn/services/catalog/internal/seed"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	store, err := mongostore.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		return err
	}
	defer store.Close(context.Background())

	if err := store.EnsureIndexes(ctx); err != nil {
		return err
	}

	products := seed.Products()
	active := 0
	for _, p := range products {
		// Refuse to load anything that breaks the catalog's own rules.
		if err := p.Validate(); err != nil {
			return err
		}
		if err := store.Upsert(ctx, p); err != nil {
			return err
		}
		if p.Status == catalog.StatusActive {
			active++
		}
	}
	log.Printf("seeded %d products (%d active, %d draft) into %q",
		len(products), active, len(products)-active, cfg.MongoDB)
	return nil
}
