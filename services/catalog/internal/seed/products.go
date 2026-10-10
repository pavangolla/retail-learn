// Package seed produces a deterministic sample catalog for local development.
// Running it twice yields identical products, so loading it is idempotent.
package seed

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
)

// Size ranges. A product picks one by key.
var sizeSets = map[string][]string{
	"apparel": {"S", "M", "L", "XL"},
	"free":    {"Free Size"},
	"shoes":   {"6", "7", "8", "9"},
	"one":     {"One Size"},
	"kids":    {"2-4Y", "4-6Y", "6-8Y"},
}

// Rotating palette; each product takes two colors from it by position.
var palette = []string{"Orange", "Maroon", "Teal", "Mustard", "Navy", "Emerald", "Pink", "Ivory"}

type spec struct {
	title    string
	ptype    string // product type shown in the UI and used for filtering
	image    string // placeholder image key: /placeholders/<image>.svg
	gender   string // women | men | girls
	fabric   string
	occasion string // casual | festive | wedding | party | formal
	cents    int64
	sale     bool
	sizes    string   // key into sizeSets
	colors   []string // optional override of the rotating palette
	bestSell bool
	draft    bool
}

var specs = []spec{
	// Women: kurta sets
	{title: "Cotton Floral Print Kurta Set", ptype: "Kurta Set", image: "kurta-set", gender: "women", fabric: "cotton", occasion: "casual", cents: 10500, sizes: "apparel", bestSell: true},
	{title: "Silk Blend Kurta Set for Elegant Occasions", ptype: "Kurta Set", image: "kurta-set", gender: "women", fabric: "silk", occasion: "festive", cents: 12000, sizes: "apparel"},
	{title: "Fine Cotton Kurta Set with Striped Borders", ptype: "Kurta Set", image: "kurta-set", gender: "women", fabric: "cotton", occasion: "casual", cents: 8999, sale: true, sizes: "apparel"},
	{title: "Chanderi Straight Kurta Set", ptype: "Kurta Set", image: "kurta-set", gender: "women", fabric: "chanderi", occasion: "festive", cents: 9999, sizes: "apparel"},
	{title: "Rayon Printed 3pc Kurti Set", ptype: "Kurta Set", image: "kurta-set", gender: "women", fabric: "rayon", occasion: "casual", cents: 6999, sale: true, sizes: "apparel", bestSell: true},
	{title: "Embroidered Georgette Palazzo Set", ptype: "Kurta Set", image: "kurta-set", gender: "women", fabric: "georgette", occasion: "party", cents: 11999, sizes: "apparel"},

	// Women: tunics and kaftans
	{title: "Swiss Dot Chiffon Tunic with Embroidery", ptype: "Tunic", image: "tunic", gender: "women", fabric: "chiffon", occasion: "casual", cents: 3899, sizes: "apparel"},
	{title: "Cotton Blend All Over Vine Tunic", ptype: "Tunic", image: "tunic", gender: "women", fabric: "cotton", occasion: "casual", cents: 3999, sale: true, sizes: "apparel", bestSell: true},
	{title: "Linen Cotton Blend V-neck Tunic", ptype: "Tunic", image: "tunic", gender: "women", fabric: "linen", occasion: "casual", cents: 3899, sizes: "apparel"},
	{title: "Cotton Peplum Top Tunic", ptype: "Tunic", image: "tunic", gender: "women", fabric: "cotton", occasion: "casual", cents: 3299, sizes: "apparel"},
	{title: "Cotton Flared Tunic for Summer", ptype: "Tunic", image: "tunic", gender: "women", fabric: "cotton", occasion: "casual", cents: 4500, sizes: "apparel"},
	{title: "Cotton Kaftan Relaxation Tunic", ptype: "Kaftan", image: "tunic", gender: "women", fabric: "cotton", occasion: "casual", cents: 3499, sale: true, sizes: "free"},
	{title: "Chiffon Crepe Kaftan", ptype: "Kaftan", image: "tunic", gender: "women", fabric: "chiffon", occasion: "festive", cents: 18000, sizes: "free"},

	// Women: anarkali
	{title: "Rayon Gathered Anarkali Dress", ptype: "Anarkali", image: "anarkali", gender: "women", fabric: "rayon", occasion: "festive", cents: 7599, sizes: "apparel"},
	{title: "Premium Cotton Anarkali Tunic Set", ptype: "Anarkali", image: "anarkali", gender: "women", fabric: "cotton", occasion: "festive", cents: 7599, sale: true, sizes: "apparel", bestSell: true},
	{title: "Embroidered Georgette Anarkali Gown", ptype: "Anarkali", image: "anarkali", gender: "women", fabric: "georgette", occasion: "party", cents: 14999, sizes: "apparel"},

	// Women: sarees
	{title: "Kanchipuram Pure Silk Saree", ptype: "Saree", image: "saree", gender: "women", fabric: "silk", occasion: "wedding", cents: 24999, sizes: "free", bestSell: true},
	{title: "Mangalagiri Cotton Saree", ptype: "Saree", image: "saree", gender: "women", fabric: "cotton", occasion: "casual", cents: 6999, sizes: "free"},
	{title: "Uppada Silk Saree", ptype: "Saree", image: "saree", gender: "women", fabric: "silk", occasion: "festive", cents: 15999, sizes: "free"},
	{title: "Mysore Silk Saree", ptype: "Saree", image: "saree", gender: "women", fabric: "silk", occasion: "wedding", cents: 17999, sizes: "free"},
	{title: "Designer Georgette Saree with Sequins", ptype: "Saree", image: "saree", gender: "women", fabric: "georgette", occasion: "party", cents: 12999, sale: true, sizes: "free"},
	{title: "Banarasi Silk Blend Saree", ptype: "Saree", image: "saree", gender: "women", fabric: "silk", occasion: "wedding", cents: 19999, sizes: "free"},
	{title: "Linen Cotton Saree with Tassels", ptype: "Saree", image: "saree", gender: "women", fabric: "linen", occasion: "casual", cents: 5999, sizes: "free"},
	{title: "Chiffon Printed Saree", ptype: "Saree", image: "saree", gender: "women", fabric: "chiffon", occasion: "casual", cents: 4999, sale: true, sizes: "free"},

	// Women and girls: lehengas
	{title: "Silk Bridal Lehenga Set", ptype: "Lehenga", image: "lehenga", gender: "women", fabric: "silk", occasion: "wedding", cents: 44999, sizes: "apparel"},
	{title: "Georgette Embroidered Lehenga Choli", ptype: "Lehenga", image: "lehenga", gender: "women", fabric: "georgette", occasion: "wedding", cents: 29999, sale: true, sizes: "apparel"},
	{title: "Kids Traditional Lehenga", ptype: "Lehenga", image: "lehenga", gender: "girls", fabric: "silk", occasion: "festive", cents: 5999, sizes: "kids"},
	{title: "Kids Traditional Frock", ptype: "Frock", image: "anarkali", gender: "girls", fabric: "cotton", occasion: "festive", cents: 4999, sizes: "kids"},

	// Men
	{title: "Cotton Straight-fit Kurta Set", ptype: "Kurta", image: "mens-kurta", gender: "men", fabric: "cotton", occasion: "casual", cents: 7099, sizes: "apparel", bestSell: true},
	{title: "Cotton Silk Blend Straight-Cut Kurta Ensemble", ptype: "Kurta", image: "mens-kurta", gender: "men", fabric: "silk", occasion: "festive", cents: 7599, sizes: "apparel"},
	{title: "Textured Rayon Straight Kurta", ptype: "Kurta", image: "mens-kurta", gender: "men", fabric: "rayon", occasion: "casual", cents: 4500, sizes: "apparel"},
	{title: "Silk Blend Festive Kurta", ptype: "Kurta", image: "mens-kurta", gender: "men", fabric: "silk", occasion: "festive", cents: 8999, sale: true, sizes: "apparel"},
	{title: "Linen Half Kurta Desi Modern", ptype: "Kurta", image: "mens-kurta", gender: "men", fabric: "linen", occasion: "casual", cents: 5499, sizes: "apparel"},
	{title: "Embroidered Wedding Sherwani", ptype: "Sherwani", image: "sherwani", gender: "men", fabric: "silk", occasion: "wedding", cents: 34999, sizes: "apparel"},
	{title: "Brocade Nehru Jacket", ptype: "Jacket", image: "sherwani", gender: "men", fabric: "brocade", occasion: "festive", cents: 8999, sizes: "apparel"},
	{title: "Everyday Polyester Shirt", ptype: "Shirt", image: "mens-kurta", gender: "men", fabric: "polyester", occasion: "formal", cents: 6000, sizes: "apparel"},
	{title: "Chinon Open Jacket Shirt", ptype: "Shirt", image: "mens-kurta", gender: "men", fabric: "chinon", occasion: "casual", cents: 3499, sizes: "apparel"},

	// Footwear
	{title: "Silver and Black Embroidered Jutti", ptype: "Jutti", image: "footwear", gender: "women", fabric: "leather", occasion: "festive", cents: 3999, sizes: "shoes", colors: []string{"Silver", "Black"}},
	{title: "Embroidered Heels", ptype: "Heels", image: "footwear", gender: "women", fabric: "leather", occasion: "party", cents: 4999, sale: true, sizes: "shoes", colors: []string{"Gold", "Maroon"}, bestSell: true},
	{title: "Beaded Formal Mule", ptype: "Mules", image: "footwear", gender: "women", fabric: "leather", occasion: "formal", cents: 4499, sizes: "shoes", colors: []string{"Gold", "Black"}},
	{title: "Rhinestone Dressy Flat", ptype: "Flats", image: "footwear", gender: "women", fabric: "leather", occasion: "party", cents: 4499, sizes: "shoes", colors: []string{"Silver", "Gold"}},
	{title: "Floral Block Mules", ptype: "Mules", image: "footwear", gender: "women", fabric: "leather", occasion: "casual", cents: 4000, sale: true, sizes: "shoes", colors: []string{"Pink", "Ivory"}},
	{title: "Kolhapuri Handmade Chappals", ptype: "Chappals", image: "footwear", gender: "women", fabric: "leather", occasion: "casual", cents: 3199, sale: true, sizes: "shoes", colors: []string{"Tan", "Brown"}},
	{title: "Zardosi Wedding Jutti", ptype: "Jutti", image: "footwear", gender: "men", fabric: "leather", occasion: "wedding", cents: 5499, sizes: "shoes", colors: []string{"Gold", "Maroon"}},
	{title: "Floral Slide Sandals", ptype: "Sandals", image: "footwear", gender: "women", fabric: "leather", occasion: "casual", cents: 3000, sale: true, sizes: "shoes", colors: []string{"Ivory", "Teal"}},

	// Accessories
	{title: "Antique Gold Jhumka Earrings", ptype: "Jewelry", image: "jewelry", gender: "women", fabric: "alloy", occasion: "festive", cents: 2499, sizes: "one", colors: []string{"Gold", "Silver"}, bestSell: true},
	{title: "Kundan Choker Necklace", ptype: "Jewelry", image: "jewelry", gender: "women", fabric: "alloy", occasion: "wedding", cents: 4999, sizes: "one", colors: []string{"Gold", "Silver"}},
	{title: "Oxidised Silver Bangles Set", ptype: "Jewelry", image: "jewelry", gender: "women", fabric: "alloy", occasion: "festive", cents: 1999, sale: true, sizes: "one", colors: []string{"Silver", "Gold"}},
	{title: "Pearl Maang Tikka", ptype: "Jewelry", image: "jewelry", gender: "women", fabric: "pearl", occasion: "wedding", cents: 1799, sizes: "one", colors: []string{"Gold", "Silver"}},
	{title: "Banarasi Silk Dupatta", ptype: "Dupatta", image: "dupatta", gender: "women", fabric: "silk", occasion: "festive", cents: 3999, sizes: "one"},
	{title: "Phulkari Cotton Dupatta", ptype: "Dupatta", image: "dupatta", gender: "women", fabric: "cotton", occasion: "casual", cents: 2999, sizes: "one"},
	{title: "Embroidered Potli Bag", ptype: "Bag", image: "bag", gender: "women", fabric: "silk", occasion: "festive", cents: 2999, sizes: "one"},
	{title: "Beaded Evening Clutch", ptype: "Bag", image: "bag", gender: "women", fabric: "silk", occasion: "party", cents: 3499, sale: true, sizes: "one"},

	// A draft product: stored but hidden from the storefront.
	{title: "Coming Soon Preview Lehenga", ptype: "Lehenga", image: "lehenga", gender: "women", fabric: "silk", occasion: "wedding", cents: 39999, sizes: "apparel", draft: true},
}

// Products returns the full sample catalog, including one draft product.
func Products() []catalog.Product {
	base := time.Date(2026, 1, 22, 9, 0, 0, 0, time.UTC)
	out := make([]catalog.Product, 0, len(specs))
	for i, s := range specs {
		out = append(out, build(i, s, base.Add(time.Duration(i)*6*time.Hour)))
	}
	return out
}

func build(i int, s spec, created time.Time) catalog.Product {
	colors := s.colors
	if colors == nil {
		colors = []string{palette[i%len(palette)], palette[(i+3)%len(palette)]}
	}
	sizes := sizeSets[s.sizes]

	tags := []string{s.gender, s.fabric, s.occasion, slug(s.ptype)}
	if s.sale {
		tags = append(tags, "sale")
	}
	if s.bestSell {
		tags = append(tags, "best-seller")
	}

	var compareAt int64
	if s.sale {
		// About 30% above the price, rounded to end in 99.
		compareAt = (s.cents*130/100/100)*100 - 1
	}

	prefix := strings.ToUpper(strings.ReplaceAll(s.image, "-", ""))
	if len(prefix) > 4 {
		prefix = prefix[:4]
	}

	variants := make([]catalog.Variant, 0, len(colors)*len(sizes))
	for _, c := range colors {
		for _, sz := range sizes {
			variants = append(variants, catalog.Variant{
				SKU:             fmt.Sprintf("%s%03d-%s-%s", prefix, i+1, code(c, 3), code(sz, 8)),
				Title:           c + " / " + sz,
				SelectedOptions: map[string]string{"Color": c, "Size": sz},
				PriceCents:      s.cents,
				CompareAtCents:  compareAt,
				Currency:        "USD",
			})
		}
	}

	status := catalog.StatusActive
	if s.draft {
		status = catalog.StatusDraft
	}

	return catalog.Product{
		Handle:      slug(s.title),
		Title:       s.title,
		Description: describe(s),
		Vendor:      "Retail-Learn",
		ProductType: s.ptype,
		Tags:        tags,
		Status:      status,
		Options: []catalog.Option{
			{Name: "Color", Values: colors},
			{Name: "Size", Values: sizes},
		},
		Variants:  variants,
		Images:    []catalog.Image{{URL: "/placeholders/" + s.image + ".svg", Alt: s.title}},
		CreatedAt: created,
		UpdatedAt: created,
	}
}

func describe(s spec) string {
	return fmt.Sprintf(
		"%s in %s, made for %s wear. Soft, comfortable and easy to style. This is sample catalog data for local development.",
		s.title, s.fabric, s.occasion)
}

// slug lowercases s and joins its letters and digits with single hyphens.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// code makes a short uppercase alphanumeric code for SKUs, at most n characters.
func code(s string, n int) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > n {
		out = out[:n]
	}
	return out
}
