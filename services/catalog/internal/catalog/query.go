package catalog

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// InvalidError means the caller asked for something malformed or not allowed
// (a bad sort name, min price above max price, a cursor from another sort).
// The HTTP layer turns it into a 400 and shows Msg to the client.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

func invalidf(format string, args ...any) error {
	return &InvalidError{Msg: fmt.Sprintf(format, args...)}
}

// Sort selects the order of a product listing.
type Sort string

const (
	SortDefault   Sort = ""           // handle ascending
	SortPriceAsc  Sort = "price_asc"  // cheapest first
	SortPriceDesc Sort = "price_desc" // most expensive first
	SortNewest    Sort = "newest"     // most recently created first
)

// ParseSort validates a sort name from a request.
func ParseSort(s string) (Sort, error) {
	switch v := Sort(s); v {
	case SortDefault, SortPriceAsc, SortPriceDesc, SortNewest:
		return v, nil
	}
	return "", invalidf("sort must be one of price_asc, price_desc, newest (omit it for the default order)")
}

// Filter narrows a listing. Zero values mean "no constraint".
type Filter struct {
	ProductType   string   // exact match, e.g. "Saree"
	Tags          []string // product must have ALL of these tags
	MinPriceCents int64    // lowest variant price, inclusive
	MaxPriceCents int64    // lowest variant price, inclusive
}

// normalize trims input and drops empty or duplicate tags.
func (f Filter) normalize() Filter {
	f.ProductType = strings.TrimSpace(f.ProductType)
	seen := make(map[string]bool, len(f.Tags))
	tags := make([]string, 0, len(f.Tags))
	for _, t := range f.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	f.Tags = tags
	return f
}

// MaxFilterTags bounds how many tags one request may combine.
const MaxFilterTags = 10

func (f Filter) validate() error {
	switch {
	case f.MinPriceCents < 0 || f.MaxPriceCents < 0:
		return invalidf("prices must not be negative")
	case f.MinPriceCents > 0 && f.MaxPriceCents > 0 && f.MinPriceCents > f.MaxPriceCents:
		return invalidf("minPrice must not be greater than maxPrice")
	case len(f.Tags) > MaxFilterTags:
		return invalidf("at most %d tags can be combined", MaxFilterTags)
	}
	return nil
}

// Position marks where a page ended, so the next page can start right after
// it. Key is the value of the active sort (price in cents, creation time in
// unix milliseconds, or unused for the default order); Handle breaks ties.
type Position struct {
	Key    int64
	Handle string
}

// SortKey returns the value of p that sort s orders by.
func SortKey(p Product, s Sort) int64 {
	switch s {
	case SortPriceAsc, SortPriceDesc:
		return p.PriceFromCents
	case SortNewest:
		return p.CreatedAt.UnixMilli()
	default:
		return 0
	}
}

// Descending reports whether sort s orders its Key from high to low.
// The handle tie-break is always ascending.
func (s Sort) Descending() bool { return s == SortPriceDesc || s == SortNewest }

// ListParams is what a repository needs to return one page of products.
type ListParams struct {
	Filter Filter
	Sort   Sort
	Limit  int
	After  *Position // nil for the first page
}

// SearchParams is what a repository needs to run a text search.
type SearchParams struct {
	Query  string
	Filter Filter
	Limit  int
}

// FacetCount is one filter choice and how many products it would match.
type FacetCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Facets summarizes a filtered set of products so a UI can build its filter
// sidebar: which types and tags exist, with counts, and the price range.
type Facets struct {
	Total         int          `json:"total"`
	ProductTypes  []FacetCount `json:"productTypes"`
	Tags          []FacetCount `json:"tags"`
	PriceMinCents int64        `json:"priceMinCents"`
	PriceMaxCents int64        `json:"priceMaxCents"`
}

// Cursors are opaque to clients: a base64 token carrying the sort it belongs
// to and the Position to resume after. Because Position includes the sort key,
// a cursor from one sort order cannot be replayed under another.
type cursorPayload struct {
	Sort   Sort   `json:"s"`
	Key    int64  `json:"k"`
	Handle string `json:"h"`
}

func encodeCursor(s Sort, pos Position) string {
	b, _ := json.Marshal(cursorPayload{Sort: s, Key: pos.Key, Handle: pos.Handle})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(token string, s Sort) (*Position, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, invalidf("invalid cursor")
	}
	var c cursorPayload
	if err := json.Unmarshal(raw, &c); err != nil || !ValidHandle(c.Handle) {
		return nil, invalidf("invalid cursor")
	}
	if c.Sort != s {
		return nil, invalidf("cursor was issued for a different sort order")
	}
	return &Position{Key: c.Key, Handle: c.Handle}, nil
}
