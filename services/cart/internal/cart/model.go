// Package cart is the shopping cart's domain: what a cart is, the rules it
// must follow, and the Service that applies them.
//
// A cart stores only SKUs and quantities. Titles and prices are looked up from
// the catalog each time the cart is read, so a price can never go stale or be
// tampered with by the client.
package cart

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Business limits. They live here, not in the store, so every Store
// implementation enforces the same numbers.
const (
	MaxQuantity    = 10                  // per line
	MaxLines       = 50                  // distinct SKUs per cart
	CartTTL        = 30 * 24 * time.Hour // an untouched cart expires; any change renews it
	IdempotencyTTL = 24 * time.Hour      // how long a retry of the same request is recognized
)

var (
	// ErrTooManyLines means adding this SKU would exceed MaxLines.
	ErrTooManyLines = errors.New("cart: too many different items")
	// ErrQuantityLimit means the quantity would exceed MaxQuantity.
	ErrQuantityLimit = errors.New("cart: quantity limit reached")
)

// InvalidError means the caller sent something malformed. Handlers turn it
// into a 400.
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

func invalidf(format string, args ...any) error {
	return &InvalidError{Msg: fmt.Sprintf(format, args...)}
}

var (
	idPattern   = regexp.MustCompile(`^[0-9a-f]{32}$`)
	skuPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	idemPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,128}$`)
)

// NewID returns a fresh cart id: 128 random bits as 32 hex characters. The id
// is the cart's only credential, so it must be unguessable.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("cart: no randomness available: " + err.Error()) // cannot happen on a healthy OS
	}
	return hex.EncodeToString(b[:])
}

// ValidID reports whether s is a well-formed cart id. Checking the shape also
// keeps odd characters out of Redis key names.
func ValidID(s string) bool { return idPattern.MatchString(s) }

// ValidSKU reports whether s looks like a SKU.
func ValidSKU(s string) bool { return skuPattern.MatchString(s) }

// ValidIdempotencyKey reports whether s is usable as an Idempotency-Key.
func ValidIdempotencyKey(s string) bool { return idemPattern.MatchString(s) }

// Image is a product picture reference.
type Image struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

// Line is one row of a cart as the shopper sees it.
type Line struct {
	SKU            string `json:"sku"`
	ProductHandle  string `json:"productHandle,omitempty"`
	Title          string `json:"title"`
	VariantTitle   string `json:"variantTitle,omitempty"`
	Image          *Image `json:"image,omitempty"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unitPriceCents"`
	CompareAtCents int64  `json:"compareAtCents,omitempty"`
	LineTotalCents int64  `json:"lineTotalCents"`
	// Available is false when the catalog no longer sells this SKU (unpublished
	// or removed). Such a line is shown so it can be removed, but costs nothing
	// and is left out of the totals.
	Available bool `json:"available"`
}

// View is a cart priced against the current catalog.
type View struct {
	ID            string `json:"id"`
	Lines         []Line `json:"lines"` // ordered by SKU so rows do not jump around
	ItemCount     int    `json:"itemCount"`
	SubtotalCents int64  `json:"subtotalCents"`
	Currency      string `json:"currency"`
}

// VariantInfo is what the cart needs to know about a SKU from the catalog.
type VariantInfo struct {
	SKU            string
	ProductHandle  string
	ProductTitle   string
	Title          string
	PriceCents     int64
	CompareAtCents int64
	Currency       string
	Image          *Image
}
