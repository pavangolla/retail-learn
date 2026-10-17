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

const (
	productsCollection    = "products"
	collectionsCollection = "collections"
)

// Store is a catalog.Repository backed by MongoDB: one collection of products
// and one of catalog collections.
type Store struct {
	client      *mongo.Client
	products    *mongo.Collection
	collections *mongo.Collection
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
	db := client.Database(dbName)
	return &Store{
		client:      client,
		products:    db.Collection(productsCollection),
		collections: db.Collection(collectionsCollection),
	}, nil
}

// Close releases the connection pool.
func (s *Store) Close(ctx context.Context) error { return s.client.Disconnect(ctx) }

// Ping reports whether MongoDB is reachable. The /health endpoint uses it.
func (s *Store) Ping(ctx context.Context) error { return s.client.Ping(ctx, nil) }

// EnsureIndexes creates the indexes the queries rely on. It is safe to run on
// every start: MongoDB ignores an index that already exists.
//
// Every index starts with the field the query filters by (equality first),
// then the sort fields, ending in handle, so MongoDB can walk the index in
// order instead of sorting in memory. Each index costs write time and disk, so
// add one only for a query that needs it; check with explain().
func (s *Store) EnsureIndexes(ctx context.Context) error {
	idx := func(name string, keys bson.D) mongo.IndexModel {
		return mongo.IndexModel{Keys: keys, Options: options.Index().SetName(name)}
	}
	products := []mongo.IndexModel{
		{
			// One product per handle; also makes GetByHandle a direct lookup.
			Keys:    bson.D{{Key: "handle", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("handle_unique"),
		},
		// Default listing order.
		idx("status_handle", bson.D{{Key: "status", Value: 1}, {Key: "handle", Value: 1}}),
		// Sort by price. Ascending and descending need separate indexes because the
		// handle tie-break is always ascending.
		idx("status_price_asc_handle", bson.D{{Key: "status", Value: 1}, {Key: "priceFromCents", Value: 1}, {Key: "handle", Value: 1}}),
		idx("status_price_desc_handle", bson.D{{Key: "status", Value: 1}, {Key: "priceFromCents", Value: -1}, {Key: "handle", Value: 1}}),
		idx("status_created_desc_handle", bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}, {Key: "handle", Value: 1}}),
		// Filter by product type, and by tag (a multikey index: one entry per tag).
		idx("type_status_handle", bson.D{{Key: "productType", Value: 1}, {Key: "status", Value: 1}, {Key: "handle", Value: 1}}),
		idx("tags_status_handle", bson.D{{Key: "tags", Value: 1}, {Key: "status", Value: 1}, {Key: "handle", Value: 1}}),
		{
			// Full-text search. A collection may have only one text index. Weights
			// make a title match count 10x a description match.
			Keys: bson.D{
				{Key: "title", Value: "text"},
				{Key: "productType", Value: "text"},
				{Key: "tags", Value: "text"},
				{Key: "description", Value: "text"},
			},
			Options: options.Index().SetName("search_text").SetWeights(bson.D{
				{Key: "title", Value: 10},
				{Key: "productType", Value: 5},
				{Key: "tags", Value: 5},
				{Key: "description", Value: 1},
			}),
		},
	}
	if _, err := s.products.Indexes().CreateMany(ctx, products); err != nil {
		return fmt.Errorf("create product indexes: %w", err)
	}

	collections := []mongo.IndexModel{{
		Keys:    bson.D{{Key: "handle", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("collection_handle_unique"),
	}}
	if _, err := s.collections.Indexes().CreateMany(ctx, collections); err != nil {
		return fmt.Errorf("create collection indexes: %w", err)
	}
	return nil
}

// find runs a product query and decodes every result.
func (s *Store) find(ctx context.Context, filter any, opts *options.FindOptionsBuilder) ([]catalog.Product, error) {
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

// List returns one page of active products: filtered, sorted, and resumed
// after params.After (keyset pagination).
func (s *Store) List(ctx context.Context, params catalog.ListParams) ([]catalog.Product, error) {
	filter := filterDoc(params.Filter)
	if params.After != nil {
		filter = append(filter, afterCond(params.Sort, *params.After))
	}
	opts := options.Find().SetSort(sortDoc(params.Sort))
	if params.Limit > 0 {
		opts.SetLimit(int64(params.Limit))
	}
	return s.find(ctx, filter, opts)
}

// Search runs a text query and returns the best matches first. MongoDB tokenizes
// and stems the query ("sarees" finds "saree"), matches products containing any
// of its words, and scores them using the index weights.
func (s *Store) Search(ctx context.Context, params catalog.SearchParams) ([]catalog.Product, error) {
	filter := append(filterDoc(params.Filter),
		bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: params.Query}}})

	score := bson.D{{Key: "$meta", Value: "textScore"}}
	opts := options.Find().SetSort(bson.D{{Key: "score", Value: score}, {Key: "handle", Value: 1}})
	if params.Limit > 0 {
		opts.SetLimit(int64(params.Limit))
	}
	return s.find(ctx, filter, opts)
}

// Facets counts product types and tags and finds the price range for the
// products matching f, in a single aggregation.
func (s *Store) Facets(ctx context.Context, f catalog.Filter) (catalog.Facets, error) {
	cur, err := s.products.Aggregate(ctx, facetPipeline(f))
	if err != nil {
		return catalog.Facets{}, fmt.Errorf("aggregate facets: %w", err)
	}
	var docs []facetDoc
	if err := cur.All(ctx, &docs); err != nil {
		return catalog.Facets{}, fmt.Errorf("decode facets: %w", err)
	}

	var out catalog.Facets
	if len(docs) == 0 { // $facet always yields one document; be defensive anyway
		return out, nil
	}
	d := docs[0]
	for _, c := range d.ProductTypes {
		out.ProductTypes = append(out.ProductTypes, catalog.FacetCount{Value: c.Value, Count: c.Count})
	}
	for _, c := range d.Tags {
		out.Tags = append(out.Tags, catalog.FacetCount{Value: c.Value, Count: c.Count})
	}
	if len(d.Price) > 0 {
		out.Total = d.Price[0].Total
		out.PriceMinCents = d.Price[0].Min
		out.PriceMaxCents = d.Price[0].Max
	}
	return out, nil
}

// facetDoc is the single document the $facet stage returns.
type facetDoc struct {
	ProductTypes []facetCount `bson:"productTypes"`
	Tags         []facetCount `bson:"tags"`
	Price        []priceStats `bson:"price"` // empty when nothing matched
}

type facetCount struct {
	Value string `bson:"_id"`
	Count int    `bson:"count"`
}

type priceStats struct {
	Min   int64 `bson:"min"`
	Max   int64 `bson:"max"`
	Total int   `bson:"total"`
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
	p.ComputeDerived() // keep priceFromCents in step with the variants
	_, err := s.products.ReplaceOne(ctx,
		bson.D{{Key: "handle", Value: p.Handle}},
		p,
		options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("upsert product %q: %w", p.Handle, err)
	}
	return nil
}
