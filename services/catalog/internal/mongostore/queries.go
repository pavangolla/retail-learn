package mongostore

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// This file turns the catalog's query types into MongoDB query documents.
// They are plain functions that build values, so they can be unit-tested
// without a database.

// filterDoc builds the filter for a catalog.Filter. Storefront queries only
// ever see active products, so status is always part of it.
func filterDoc(f catalog.Filter) bson.D {
	doc := bson.D{{Key: "status", Value: catalog.StatusActive}}
	if f.ProductType != "" {
		doc = append(doc, bson.E{Key: "productType", Value: f.ProductType})
	}
	if len(f.Tags) > 0 {
		// $all: the product's tags array must contain every listed tag.
		doc = append(doc, bson.E{Key: "tags", Value: bson.D{{Key: "$all", Value: f.Tags}}})
	}

	// Price filters use the derived priceFromCents field (the cheapest variant),
	// because MongoDB cannot index "the minimum of an embedded array".
	var price bson.D
	if f.MinPriceCents > 0 {
		price = append(price, bson.E{Key: "$gte", Value: f.MinPriceCents})
	}
	if f.MaxPriceCents > 0 {
		price = append(price, bson.E{Key: "$lte", Value: f.MaxPriceCents})
	}
	if len(price) > 0 {
		doc = append(doc, bson.E{Key: "priceFromCents", Value: price})
	}
	return doc
}

// sortSpec names the document field a sort orders by and its direction.
// An empty field means the default order, which is just the handle.
func sortSpec(s catalog.Sort) (field string, dir int) {
	switch s {
	case catalog.SortPriceAsc:
		return "priceFromCents", 1
	case catalog.SortPriceDesc:
		return "priceFromCents", -1
	case catalog.SortNewest:
		return "createdAt", -1
	}
	return "", 1
}

// sortDoc builds the sort document. The handle is always the final key: it is
// unique, so it makes the order total and the keyset cursor exact.
func sortDoc(s catalog.Sort) bson.D {
	field, dir := sortSpec(s)
	if field == "" {
		return bson.D{{Key: "handle", Value: 1}}
	}
	return bson.D{{Key: field, Value: dir}, {Key: "handle", Value: 1}}
}

// afterCond builds the keyset condition "strictly after pos" for a sort. For a
// compound order (sort key, then handle) that is: the key is further along, OR
// the key is equal and the handle is further along.
func afterCond(s catalog.Sort, pos catalog.Position) bson.E {
	field, dir := sortSpec(s)
	handleAfter := bson.D{{Key: "$gt", Value: pos.Handle}}
	if field == "" {
		return bson.E{Key: "handle", Value: handleAfter}
	}

	var val any = pos.Key
	if field == "createdAt" {
		val = time.UnixMilli(pos.Key).UTC() // dates are stored with millisecond precision
	}
	further := "$gt" // ascending: larger values come later
	if dir < 0 {
		further = "$lt" // descending: smaller values come later
	}
	return bson.E{Key: "$or", Value: bson.A{
		bson.D{{Key: field, Value: bson.D{{Key: further, Value: val}}}},
		bson.D{{Key: field, Value: val}, {Key: "handle", Value: handleAfter}},
	}}
}

// facetPipeline builds the aggregation behind catalog.Facets: one pass over the
// matching products that computes counts per type, counts per tag, and the
// price range, using $facet to run the three sub-pipelines side by side.
func facetPipeline(f catalog.Filter) bson.A {
	countBy := func(field string) bson.A {
		return bson.A{
			bson.D{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$" + field},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			}}},
			bson.D{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}, {Key: "_id", Value: 1}}}},
		}
	}

	tagStages := bson.A{bson.D{{Key: "$unwind", Value: "$tags"}}}
	tagStages = append(tagStages, countBy("tags")...)

	return bson.A{
		bson.D{{Key: "$match", Value: filterDoc(f)}},
		bson.D{{Key: "$facet", Value: bson.D{
			{Key: "productTypes", Value: countBy("productType")},
			{Key: "tags", Value: tagStages},
			{Key: "price", Value: bson.A{
				bson.D{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: nil},
					{Key: "min", Value: bson.D{{Key: "$min", Value: "$priceFromCents"}}},
					{Key: "max", Value: bson.D{{Key: "$max", Value: "$priceFromCents"}}},
					{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
			}},
		}}},
	}
}
