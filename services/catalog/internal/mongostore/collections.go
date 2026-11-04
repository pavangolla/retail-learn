package mongostore

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// ListCollections returns every collection ordered by handle.
func (s *Store) ListCollections(ctx context.Context) ([]catalog.Collection, error) {
	cur, err := s.collections.Find(ctx, bson.D{},
		options.Find().SetSort(bson.D{{Key: "handle", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("find collections: %w", err)
	}
	var out []catalog.Collection
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("decode collections: %w", err)
	}
	return out, nil
}

// GetCollection returns one collection, or catalog.ErrNotFound.
func (s *Store) GetCollection(ctx context.Context, handle string) (catalog.Collection, error) {
	var c catalog.Collection
	err := s.collections.FindOne(ctx, bson.D{{Key: "handle", Value: handle}}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return catalog.Collection{}, catalog.ErrNotFound
	}
	if err != nil {
		return catalog.Collection{}, fmt.Errorf("find collection %q: %w", handle, err)
	}
	return c, nil
}

// UpsertCollection inserts or replaces a collection by handle.
func (s *Store) UpsertCollection(ctx context.Context, c catalog.Collection) error {
	_, err := s.collections.ReplaceOne(ctx,
		bson.D{{Key: "handle", Value: c.Handle}},
		c,
		options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("upsert collection %q: %w", c.Handle, err)
	}
	return nil
}
