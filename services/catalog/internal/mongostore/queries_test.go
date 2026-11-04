package mongostore

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// These tests need no database: they check the query documents themselves.

func TestFilterDoc(t *testing.T) {
	active := bson.E{Key: "status", Value: "active"}
	tests := []struct {
		name string
		in   catalog.Filter
		want bson.D
	}{
		{"empty filter still limits to active", catalog.Filter{}, bson.D{active}},
		{"type", catalog.Filter{ProductType: "Saree"}, bson.D{active, {Key: "productType", Value: "Saree"}}},
		{"tags use $all", catalog.Filter{Tags: []string{"silk", "wedding"}},
			bson.D{active, {Key: "tags", Value: bson.D{{Key: "$all", Value: []string{"silk", "wedding"}}}}}},
		{"min only", catalog.Filter{MinPriceCents: 500},
			bson.D{active, {Key: "priceFromCents", Value: bson.D{{Key: "$gte", Value: int64(500)}}}}},
		{"max only", catalog.Filter{MaxPriceCents: 900},
			bson.D{active, {Key: "priceFromCents", Value: bson.D{{Key: "$lte", Value: int64(900)}}}}},
		{"range", catalog.Filter{MinPriceCents: 500, MaxPriceCents: 900},
			bson.D{active, {Key: "priceFromCents", Value: bson.D{{Key: "$gte", Value: int64(500)}, {Key: "$lte", Value: int64(900)}}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := filterDoc(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %#v\nwant %#v", got, tc.want)
			}
		})
	}
}

func TestSortDoc(t *testing.T) {
	tests := map[catalog.Sort]bson.D{
		catalog.SortDefault:   {{Key: "handle", Value: 1}},
		catalog.SortPriceAsc:  {{Key: "priceFromCents", Value: 1}, {Key: "handle", Value: 1}},
		catalog.SortPriceDesc: {{Key: "priceFromCents", Value: -1}, {Key: "handle", Value: 1}},
		catalog.SortNewest:    {{Key: "createdAt", Value: -1}, {Key: "handle", Value: 1}},
	}
	for s, want := range tests {
		if got := sortDoc(s); !reflect.DeepEqual(got, want) {
			t.Errorf("sortDoc(%q) = %#v, want %#v", s, got, want)
		}
	}
}

func TestAfterCond(t *testing.T) {
	pos := catalog.Position{Key: 3999, Handle: "f-jutti"}
	handleAfter := bson.D{{Key: "$gt", Value: "f-jutti"}}

	t.Run("default order is just handle > cursor", func(t *testing.T) {
		want := bson.E{Key: "handle", Value: handleAfter}
		if got := afterCond(catalog.SortDefault, pos); !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v", got)
		}
	})

	t.Run("ascending price: larger price, or equal price and later handle", func(t *testing.T) {
		want := bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "priceFromCents", Value: bson.D{{Key: "$gt", Value: int64(3999)}}}},
			bson.D{{Key: "priceFromCents", Value: int64(3999)}, {Key: "handle", Value: handleAfter}},
		}}
		if got := afterCond(catalog.SortPriceAsc, pos); !reflect.DeepEqual(got, want) {
			t.Errorf("got %#v", got)
		}
	})

	t.Run("descending price: smaller price comes later, handle still ascending", func(t *testing.T) {
		got := afterCond(catalog.SortPriceDesc, pos)
		first := got.Value.(bson.A)[0].(bson.D)[0].Value.(bson.D)[0]
		if first.Key != "$lt" {
			t.Errorf("descending sort should compare with $lt, got %s", first.Key)
		}
		second := got.Value.(bson.A)[1].(bson.D)[1].Value
		if !reflect.DeepEqual(second, handleAfter) {
			t.Errorf("tie-break must stay handle > cursor, got %#v", second)
		}
	})

	t.Run("newest compares real dates, not numbers", func(t *testing.T) {
		ms := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC).UnixMilli()
		got := afterCond(catalog.SortNewest, catalog.Position{Key: ms, Handle: "j-tunic"})
		cond := got.Value.(bson.A)[0].(bson.D)[0]
		if cond.Key != "createdAt" {
			t.Fatalf("field = %s", cond.Key)
		}
		inner := cond.Value.(bson.D)[0]
		if inner.Key != "$lt" || !reflect.DeepEqual(inner.Value, time.UnixMilli(ms).UTC()) {
			t.Errorf("condition = %#v, want $lt with a time.Time", inner)
		}
	})
}

func TestFacetPipelineShape(t *testing.T) {
	p := facetPipeline(catalog.Filter{Tags: []string{"silk"}})
	if len(p) != 2 {
		t.Fatalf("pipeline has %d stages, want $match then $facet", len(p))
	}
	match := p[0].(bson.D)[0]
	if match.Key != "$match" || !reflect.DeepEqual(match.Value, filterDoc(catalog.Filter{Tags: []string{"silk"}})) {
		t.Errorf("first stage = %#v, want $match with the filter", match)
	}
	facet := p[1].(bson.D)[0]
	if facet.Key != "$facet" {
		t.Fatalf("second stage = %s", facet.Key)
	}
	var keys []string
	for _, e := range facet.Value.(bson.D) {
		keys = append(keys, e.Key)
	}
	if !reflect.DeepEqual(keys, []string{"productTypes", "tags", "price"}) {
		t.Errorf("facet branches = %v", keys)
	}
}
