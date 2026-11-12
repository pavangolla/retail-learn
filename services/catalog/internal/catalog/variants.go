package catalog

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// MaxVariantLookup is how many SKUs one lookup may ask for.
const MaxVariantLookup = 50

var skuPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidSKU reports whether s looks like a SKU.
func ValidSKU(s string) bool { return skuPattern.MatchString(s) }

// VariantInfo is what another service (the cart, later checkout) needs to know
// about one purchasable variant: its identity, current price and a card image.
// It is a flat, read-only view; the catalog stays the owner of prices.
type VariantInfo struct {
	SKU            string `json:"sku"`
	ProductHandle  string `json:"productHandle"`
	ProductTitle   string `json:"productTitle"`
	Title          string `json:"title"` // variant title, e.g. "Red / S"
	PriceCents     int64  `json:"priceCents"`
	CompareAtCents int64  `json:"compareAtCents,omitempty"`
	Currency       string `json:"currency"`
	Image          *Image `json:"image,omitempty"`
}

// NewVariantInfo flattens a variant of p. Both repositories use it so they
// build the same view.
func NewVariantInfo(p Product, v Variant) VariantInfo {
	info := VariantInfo{
		SKU:            v.SKU,
		ProductHandle:  p.Handle,
		ProductTitle:   p.Title,
		Title:          v.Title,
		PriceCents:     v.PriceCents,
		CompareAtCents: v.CompareAtCents,
		Currency:       v.Currency,
	}
	if len(p.Images) > 0 {
		img := p.Images[0]
		info.Image = &img
	}
	return info
}

// Variants looks up several variants by SKU in one call, so a caller with a
// 10-line cart makes one request, not ten.
//
// Only variants of active products are returned. Unknown SKUs (a typo, a
// product since unpublished) are simply left out: the caller compares what it
// asked for with what came back. Results follow the order of the request.
func (s *Service) Variants(ctx context.Context, skus []string) ([]VariantInfo, error) {
	if len(skus) == 0 {
		return nil, invalidf("at least one sku is required")
	}
	if len(skus) > MaxVariantLookup {
		return nil, invalidf("at most %d skus per request", MaxVariantLookup)
	}

	order := make([]string, 0, len(skus))
	seen := make(map[string]bool, len(skus))
	for _, raw := range skus {
		sku := strings.TrimSpace(raw)
		if !ValidSKU(sku) {
			return nil, invalidf("invalid sku %q", raw)
		}
		if !seen[sku] {
			seen[sku] = true
			order = append(order, sku)
		}
	}

	found, err := s.repo.FindVariants(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("find variants: %w", err)
	}
	bySKU := make(map[string]VariantInfo, len(found))
	for _, v := range found {
		bySKU[v.SKU] = v
	}
	out := make([]VariantInfo, 0, len(found))
	for _, sku := range order {
		if v, ok := bySKU[sku]; ok {
			out = append(out, v)
		}
	}
	return out, nil
}
