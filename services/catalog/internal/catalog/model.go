// Package catalog holds the product catalog's domain model, its business
// rules, and the Repository interface that storage implementations satisfy.
package catalog

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

// Status values for a product. Only active products are visible on the storefront.
const (
	StatusActive = "active"
	StatusDraft  = "draft"
)

// Product is one sellable item, e.g. "Cotton Kaftan Relaxation Tunic".
//
// Variants are embedded in the product, not stored separately. A product
// page always needs the product and all its variants together, so keeping
// them in one MongoDB document means one read and atomic updates. This is the
// main reason a document store suits a catalog.
//
// Money is always integer cents plus a currency code, never floats.
type Product struct {
	Handle      string    `json:"handle" bson:"handle"` // URL slug; the public identifier
	Title       string    `json:"title" bson:"title"`
	Description string    `json:"description" bson:"description"` // plain text
	Vendor      string    `json:"vendor" bson:"vendor"`
	ProductType string    `json:"productType" bson:"productType"`
	Tags        []string  `json:"tags" bson:"tags"`
	Status      string    `json:"status" bson:"status"`
	Options     []Option  `json:"options" bson:"options"`
	Variants    []Variant `json:"variants" bson:"variants"`
	Images      []Image   `json:"images" bson:"images"`
	// PriceFromCents is the lowest variant price. It is derived data, stored on
	// the document only so MongoDB can filter and sort on it (it cannot index
	// "the minimum of an embedded array"). Call ComputeDerived before saving.
	PriceFromCents int64     `json:"-" bson:"priceFromCents"`
	CreatedAt      time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt" bson:"updatedAt"`
}

// Option is a dimension a shopper chooses, e.g. Color or Size.
type Option struct {
	Name   string   `json:"name" bson:"name"`
	Values []string `json:"values" bson:"values"`
}

// Variant is one purchasable combination of option values, e.g. Orange / M.
// The SKU identifies it for inventory and orders. Stock counts are NOT here:
// inventory becomes its own service in Phase 4.
type Variant struct {
	SKU             string            `json:"sku" bson:"sku"`
	Title           string            `json:"title" bson:"title"`
	SelectedOptions map[string]string `json:"selectedOptions" bson:"selectedOptions"`
	PriceCents      int64             `json:"priceCents" bson:"priceCents"`
	CompareAtCents  int64             `json:"compareAtCents,omitempty" bson:"compareAtCents,omitempty"` // 0 = no discount shown
	Currency        string            `json:"currency" bson:"currency"`
}

// Image is a product picture. URLs may be absolute or paths served by the storefront.
type Image struct {
	URL string `json:"url" bson:"url"`
	Alt string `json:"alt" bson:"alt"`
}

// ComputeDerived refreshes fields calculated from other fields. Every code
// path that saves a product must call it, or price filters and sorts go stale.
func (p *Product) ComputeDerived() {
	p.PriceFromCents = 0
	for i, v := range p.Variants {
		if i == 0 || v.PriceCents < p.PriceFromCents {
			p.PriceFromCents = v.PriceCents
		}
	}
}

// Collection is a named group of products defined by a rule, like a Shopify
// "smart collection". The rule is simple: a product belongs to the collection
// when it carries Tag.
type Collection struct {
	Handle      string `json:"handle" bson:"handle"`
	Title       string `json:"title" bson:"title"`
	Description string `json:"description" bson:"description"`
	Tag         string `json:"tag" bson:"tag"`
}

// Validate checks a collection's business rules.
func (c Collection) Validate() error {
	var errs []error
	if !ValidHandle(c.Handle) {
		errs = append(errs, fmt.Errorf("handle %q must be a lowercase slug (a-z, 0-9, hyphens)", c.Handle))
	}
	if c.Title == "" {
		errs = append(errs, errors.New("title is required"))
	}
	if !ValidHandle(c.Tag) {
		errs = append(errs, fmt.Errorf("tag %q must be a lowercase slug", c.Tag))
	}
	return errors.Join(errs...)
}

// Summary is the small shape used in product lists and cards. A list page
// should not ship every variant of every product, so it gets only what a card needs.
type Summary struct {
	Handle             string   `json:"handle"`
	Title              string   `json:"title"`
	ProductType        string   `json:"productType"`
	Tags               []string `json:"tags"`
	Image              *Image   `json:"image,omitempty"`
	PriceFromCents     int64    `json:"priceFromCents"`
	CompareAtFromCents int64    `json:"compareAtFromCents,omitempty"`
	Currency           string   `json:"currency"`
}

// Summary builds the card view: first image and the lowest-priced variant.
func (p Product) Summary() Summary {
	s := Summary{
		Handle:      p.Handle,
		Title:       p.Title,
		ProductType: p.ProductType,
		Tags:        p.Tags,
	}
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if len(p.Images) > 0 {
		img := p.Images[0]
		s.Image = &img
	}
	if len(p.Variants) > 0 {
		cheapest := p.Variants[0]
		for _, v := range p.Variants[1:] {
			if v.PriceCents < cheapest.PriceCents {
				cheapest = v
			}
		}
		s.PriceFromCents = cheapest.PriceCents
		s.CompareAtFromCents = cheapest.CompareAtCents
		s.Currency = cheapest.Currency
	}
	return s
}

var (
	handlePattern   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
)

// ValidHandle reports whether s looks like a product handle (lowercase slug).
func ValidHandle(s string) bool { return handlePattern.MatchString(s) }

// Validate checks the business rules for a product. It returns all problems
// found, joined, so a seed script or admin form can show them at once.
func (p Product) Validate() error {
	var msgs []string
	add := func(format string, args ...any) { msgs = append(msgs, fmt.Sprintf(format, args...)) }

	if !ValidHandle(p.Handle) {
		add("handle %q must be a lowercase slug (a-z, 0-9, hyphens)", p.Handle)
	}
	if p.Title == "" {
		add("title is required")
	}
	if p.Status != StatusActive && p.Status != StatusDraft {
		add("status %q must be %q or %q", p.Status, StatusActive, StatusDraft)
	}
	if len(p.Variants) == 0 {
		add("at least one variant is required")
	}

	allowed := make(map[string]map[string]bool, len(p.Options))
	for _, o := range p.Options {
		if o.Name == "" || len(o.Values) == 0 {
			add("option %q needs a name and at least one value", o.Name)
			continue
		}
		set := make(map[string]bool, len(o.Values))
		for _, v := range o.Values {
			set[v] = true
		}
		allowed[o.Name] = set
	}

	seenSKU := make(map[string]bool, len(p.Variants))
	for i, v := range p.Variants {
		switch {
		case v.SKU == "":
			add("variant %d: sku is required", i)
		case seenSKU[v.SKU]:
			add("variant %d: duplicate sku %q", i, v.SKU)
		}
		seenSKU[v.SKU] = true

		if v.PriceCents <= 0 {
			add("variant %q: priceCents must be positive", v.SKU)
		}
		if v.CompareAtCents != 0 && v.CompareAtCents < v.PriceCents {
			add("variant %q: compareAtCents must be 0 or at least priceCents", v.SKU)
		}
		if !currencyPattern.MatchString(v.Currency) {
			add("variant %q: currency %q must be a 3-letter code like USD", v.SKU, v.Currency)
		}
		for name := range allowed {
			val, ok := v.SelectedOptions[name]
			if !ok {
				add("variant %q: missing a value for option %q", v.SKU, name)
			} else if !allowed[name][val] {
				add("variant %q: %q is not a listed value of option %q", v.SKU, val, name)
			}
		}
		for name := range v.SelectedOptions {
			if _, ok := allowed[name]; !ok {
				add("variant %q: unknown option %q", v.SKU, name)
			}
		}
	}

	// Sort so the message is stable (map iteration order is random).
	sort.Strings(msgs)
	errs := make([]error, len(msgs))
	for i, m := range msgs {
		errs[i] = errors.New(m)
	}
	return errors.Join(errs...)
}
