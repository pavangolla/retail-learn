package cart

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// ErrUnavailable means a service the cart depends on (the catalog) could not
// answer. Handlers turn it into a 503 so clients know to retry later.
var ErrUnavailable = errors.New("cart: dependency unavailable")

// Catalog is the part of the catalog service the cart depends on. In
// production it is an HTTP client; in tests, a fake.
type Catalog interface {
	// Variants returns info for the SKUs it knows, in any order, leaving out
	// unknown or unpublished ones.
	Variants(ctx context.Context, skus []string) ([]VariantInfo, error)
}

// Service applies the cart rules on top of a Store and a Catalog.
type Service struct {
	store   Store
	catalog Catalog
}

// NewService returns a Service.
func NewService(store Store, catalog Catalog) *Service {
	return &Service{store: store, catalog: catalog}
}

// Create returns a new cart id. Nothing is stored until the first item is
// added, so abandoned empty carts cost nothing.
func (s *Service) Create() string { return NewID() }

// Get returns the cart priced against the current catalog. A well-formed id
// that has no items is an empty cart, not an error.
func (s *Service) Get(ctx context.Context, cartID string) (View, error) {
	if !ValidID(cartID) {
		return View{}, invalidf("invalid cart id")
	}
	return s.view(ctx, cartID)
}

// Add puts qty more of sku into the cart. idemKey (the Idempotency-Key header)
// is required: a network retry then cannot add the item twice.
func (s *Service) Add(ctx context.Context, cartID, sku string, qty int, idemKey string) (View, bool, error) {
	if !ValidID(cartID) {
		return View{}, false, invalidf("invalid cart id")
	}
	if !ValidSKU(sku) {
		return View{}, false, invalidf("invalid sku")
	}
	if qty < 1 || qty > MaxQuantity {
		return View{}, false, invalidf("quantity must be between 1 and %d", MaxQuantity)
	}
	if !ValidIdempotencyKey(idemKey) {
		return View{}, false, invalidf("Idempotency-Key header is required (8-128 letters, digits, '.', '_' or '-')")
	}
	if err := s.mustSell(ctx, sku); err != nil {
		return View{}, false, err
	}
	replayed, err := s.store.Add(ctx, cartID, sku, qty, idemKey)
	if err != nil {
		return View{}, false, err
	}
	v, err := s.view(ctx, cartID)
	return v, replayed, err
}

// SetQuantity sets a line to exactly qty. qty 0 removes it.
func (s *Service) SetQuantity(ctx context.Context, cartID, sku string, qty int) (View, error) {
	if !ValidID(cartID) {
		return View{}, invalidf("invalid cart id")
	}
	if !ValidSKU(sku) {
		return View{}, invalidf("invalid sku")
	}
	if qty < 0 || qty > MaxQuantity {
		return View{}, invalidf("quantity must be between 0 and %d", MaxQuantity)
	}
	// Removing never needs the catalog, so a delisted product can always be removed.
	if qty > 0 {
		if err := s.mustSell(ctx, sku); err != nil {
			return View{}, err
		}
	}
	if err := s.store.Set(ctx, cartID, sku, qty); err != nil {
		return View{}, err
	}
	return s.view(ctx, cartID)
}

// Remove deletes one line.
func (s *Service) Remove(ctx context.Context, cartID, sku string) (View, error) {
	return s.SetQuantity(ctx, cartID, sku, 0)
}

// Clear empties the cart.
func (s *Service) Clear(ctx context.Context, cartID string) error {
	if !ValidID(cartID) {
		return invalidf("invalid cart id")
	}
	return s.store.Delete(ctx, cartID)
}

// mustSell rejects a SKU the catalog does not currently sell. The check is made
// here, on the server, so a client cannot put an invented SKU in a cart.
func (s *Service) mustSell(ctx context.Context, sku string) error {
	found, err := s.catalog.Variants(ctx, []string{sku})
	if err != nil {
		return fmt.Errorf("%w: check sku with catalog: %v", ErrUnavailable, err)
	}
	for _, v := range found {
		if v.SKU == sku {
			return nil
		}
	}
	return invalidf("unknown sku %q", sku)
}

// view reads the cart and prices every line from the catalog.
func (s *Service) view(ctx context.Context, cartID string) (View, error) {
	items, err := s.store.Items(ctx, cartID)
	if err != nil {
		return View{}, fmt.Errorf("read cart: %w", err)
	}
	v := View{ID: cartID, Lines: []Line{}, Currency: "USD"}
	if len(items) == 0 {
		return v, nil
	}

	skus := make([]string, 0, len(items))
	for sku := range items {
		skus = append(skus, sku)
	}
	sort.Strings(skus)

	// MaxLines (50) equals the catalog's per-request limit, so this is one call.
	infos, err := s.catalog.Variants(ctx, skus)
	if err != nil {
		return View{}, fmt.Errorf("%w: price cart: %v", ErrUnavailable, err)
	}
	bySKU := make(map[string]VariantInfo, len(infos))
	for _, info := range infos {
		bySKU[info.SKU] = info
	}

	currencySet := false
	for _, sku := range skus {
		qty := items[sku]
		info, ok := bySKU[sku]
		if !ok {
			v.Lines = append(v.Lines, Line{SKU: sku, Title: "No longer available", Quantity: qty})
			continue
		}
		total := info.PriceCents * int64(qty)
		v.Lines = append(v.Lines, Line{
			SKU: sku, ProductHandle: info.ProductHandle, Title: info.ProductTitle,
			VariantTitle: info.Title, Image: info.Image, Quantity: qty,
			UnitPriceCents: info.PriceCents, CompareAtCents: info.CompareAtCents,
			LineTotalCents: total, Available: true,
		})
		v.ItemCount += qty
		v.SubtotalCents += total
		if !currencySet {
			v.Currency, currencySet = info.Currency, true
		}
	}
	return v, nil
}
