// Package mongostore implements catalog.Repository on MongoDB.
//
// It is the only package in the service that imports the MongoDB driver, so
// the rest of the code stays testable without a database.
package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

const productsCollection = "products"

// Store is a catalog.Repository backed by one MongoDB collection.
type Store struct {
	client   *mongo.Client
	products *mongo.Collection
}

// Compile-time check that Store satisfies the interface.
var _ catalog.Repository = (*Store)(nil)

// Connect opens a client and verifies the server is reachable.
func Connect(ctx context.Context, uri, dbName string) (*Store, error) {
	client, err := mongo.Connect(options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping mongodb at %s: %w", uri, err)
	}
	return &Store{
		client:   client,
		products: client.Database(dbName).Collection(productsCollection),
	}, nil
}

// Close releases the connection pool.
func (s *Store) Close(ctx context.Context) error { return s.client.Disconnect(ctx) }

// Ping reports whether MongoDB is reachable. The /health endpoint uses it.
func (s *Store) Ping(ctx context.Context) error { return s.client.Ping(ctx, nil) }

// EnsureIndexes creates the indexes the queries rely on. It is safe to run on
// every start: MongoDB ignores an index that already exists.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{
			// One product per handle; also makes GetByHandle a direct lookup.
			Keys:    bson.D{{Key: "handle", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("handle_unique"),
		},
		{
			// Serves List: filter on status, ordered by handle, keyset-paged.
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "handle", Value: 1}},
			Options: options.Index().SetName("status_handle"),
		},
	}
	if _, err := s.products.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

// List returns active products ordered by handle, after params.After.
func (s *Store) List(ctx context.Context, params catalog.ListParams) ([]catalog.Product, error) {
	filter := bson.D{{Key: "status", Value: catalog.StatusActive}}
	if params.After != "" {
		filter = append(filter, bson.E{Key: "handle", Value: bson.D{{Key: "$gt", Value: params.After}}})
	}

	opts := options.Find().SetSort(bson.D{{Key: "handle", Value: 1}})
	if params.Limit > 0 {
		opts.SetLimit(int64(params.Limit))
	}

	cur, err := s.products.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("find products: %w", err)
	}
	var out []catalog.Product
	if err := cur.All(ctx, &out); err != nil { // All also closes the cursor
		return nil, fmt.Errorf("decode products: %w", err)
	}
	return out, nil
}

// GetByHandle returns one active product, or catalog.ErrNotFound.
func (s *Store) GetByHandle(ctx context.Context, handle string) (catalog.Product, error) {
	filter := bson.D{
		{Key: "handle", Value: handle},
		{Key: "status", Value: catalog.StatusActive},
	}
	var p catalog.Product
	err := s.products.FindOne(ctx, filter).Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return catalog.Product{}, catalog.ErrNotFound
	}
	if err != nil {
		return catalog.Product{}, fmt.Errorf("find product %q: %w", handle, err)
	}
	return p, nil
}

// Upsert inserts or replaces a product by handle. The seed command uses it,
// which is what makes seeding safe to re-run.
func (s *Store) Upsert(ctx context.Context, p catalog.Product) error {
	_, err := s.products.ReplaceOne(ctx,
		bson.D{{Key: "handle", Value: p.Handle}},
		p,
		options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("upsert product %q: %w", p.Handle, err)
	}
	return nil
}
