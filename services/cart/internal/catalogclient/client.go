// Package catalogclient is the cart's HTTP client for the catalog service.
// The cart never reads the catalog's database: it asks the catalog's API, the
// way every service talks to another.
package catalogclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
)

// Client implements cart.Catalog over HTTP.
type Client struct {
	base string
	http *http.Client
}

var _ cart.Catalog = (*Client)(nil)

// New returns a client for the catalog at baseURL (e.g. http://localhost:8081).
// The timeout matters: without one, a hung catalog would hang every cart request.
func New(baseURL string) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

type variantsResponse struct {
	Items []struct {
		SKU            string      `json:"sku"`
		ProductHandle  string      `json:"productHandle"`
		ProductTitle   string      `json:"productTitle"`
		Title          string      `json:"title"`
		PriceCents     int64       `json:"priceCents"`
		CompareAtCents int64       `json:"compareAtCents"`
		Currency       string      `json:"currency"`
		Image          *cart.Image `json:"image"`
	} `json:"items"`
}

// Variants calls GET /variants?sku=...&sku=...
func (c *Client) Variants(ctx context.Context, skus []string) ([]cart.VariantInfo, error) {
	q := url.Values{}
	for _, s := range skus {
		q.Add("sku", s)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/variants?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build catalog request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog /variants answered %d", resp.StatusCode)
	}

	var body variantsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode catalog response: %w", err)
	}
	out := make([]cart.VariantInfo, 0, len(body.Items))
	for _, it := range body.Items {
		out = append(out, cart.VariantInfo{
			SKU: it.SKU, ProductHandle: it.ProductHandle, ProductTitle: it.ProductTitle,
			Title: it.Title, PriceCents: it.PriceCents, CompareAtCents: it.CompareAtCents,
			Currency: it.Currency, Image: it.Image,
		})
	}
	return out, nil
}
