package cart_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
)

// fakeCatalog sells a fixed set of SKUs and counts how often it is asked.
type fakeCatalog struct {
	sold  map[string]cart.VariantInfo
	calls int
	err   error
}

func (f *fakeCatalog) Variants(_ context.Context, skus []string) ([]cart.VariantInfo, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	var out []cart.VariantInfo
	for _, s := range skus {
		if v, ok := f.sold[s]; ok {
			out = append(out, v)
		}
	}
	return out, nil
}

func newFake() *fakeCatalog {
	mk := func(sku, title string, price, compare int64) cart.VariantInfo {
		return cart.VariantInfo{
			SKU: sku, ProductHandle: strings.ToLower(sku), ProductTitle: title, Title: "Red / S",
			PriceCents: price, CompareAtCents: compare, Currency: "USD",
			Image: &cart.Image{URL: "/placeholders/x.svg", Alt: title},
		}
	}
	return &fakeCatalog{sold: map[string]cart.VariantInfo{
		"SAREE-1": mk("SAREE-1", "Silk Saree", 20000, 0),
		"KURTA-1": mk("KURTA-1", "Cotton Kurta", 4500, 5999),
	}}
}

func TestServiceAddAndView(t *testing.T) {
	ctx := context.Background()
	cat := newFake()
	svc := cart.NewService(cart.NewMemoryStore(), cat)
	id := svc.Create()

	if _, _, err := svc.Add(ctx, id, "SAREE-1", 1, "idem-key-0001"); err != nil {
		t.Fatal(err)
	}
	v, replayed, err := svc.Add(ctx, id, "KURTA-1", 2, "idem-key-0002")
	if err != nil || replayed {
		t.Fatalf("err=%v replayed=%v", err, replayed)
	}

	// Lines are sorted by SKU; prices come from the catalog, not the caller.
	if len(v.Lines) != 2 || v.Lines[0].SKU != "KURTA-1" || v.Lines[1].SKU != "SAREE-1" {
		t.Fatalf("lines = %+v", v.Lines)
	}
	if k := v.Lines[0]; k.Quantity != 2 || k.UnitPriceCents != 4500 || k.LineTotalCents != 9000 ||
		k.CompareAtCents != 5999 || !k.Available || k.Title != "Cotton Kurta" {
		t.Errorf("kurta line = %+v", k)
	}
	if v.ItemCount != 3 || v.SubtotalCents != 29000 || v.Currency != "USD" {
		t.Errorf("totals: count=%d subtotal=%d currency=%s", v.ItemCount, v.SubtotalCents, v.Currency)
	}
}

func TestServiceRetryDoesNotDoubleAdd(t *testing.T) {
	ctx := context.Background()
	svc := cart.NewService(cart.NewMemoryStore(), newFake())
	id := svc.Create()

	if _, _, err := svc.Add(ctx, id, "SAREE-1", 1, "same-key-123"); err != nil {
		t.Fatal(err)
	}
	v, replayed, err := svc.Add(ctx, id, "SAREE-1", 1, "same-key-123")
	if err != nil || !replayed {
		t.Fatalf("err=%v replayed=%v, want a replay", err, replayed)
	}
	if v.Lines[0].Quantity != 1 {
		t.Errorf("quantity = %d, want 1", v.Lines[0].Quantity)
	}
}

func TestServiceValidation(t *testing.T) {
	ctx := context.Background()
	svc := cart.NewService(cart.NewMemoryStore(), newFake())
	id := svc.Create()

	var invalid *cart.InvalidError
	tests := []struct {
		name string
		call func() error
	}{
		{"bad cart id", func() error { _, _, err := svc.Add(ctx, "../etc", "SAREE-1", 1, "idem-key-0001"); return err }},
		{"short cart id", func() error { _, err := svc.Get(ctx, "abc"); return err }},
		{"bad sku", func() error { _, _, err := svc.Add(ctx, id, "a b", 1, "idem-key-0001"); return err }},
		{"zero quantity", func() error { _, _, err := svc.Add(ctx, id, "SAREE-1", 0, "idem-key-0001"); return err }},
		{"huge quantity", func() error { _, _, err := svc.Add(ctx, id, "SAREE-1", 11, "idem-key-0001"); return err }},
		{"missing idempotency key", func() error { _, _, err := svc.Add(ctx, id, "SAREE-1", 1, ""); return err }},
		{"short idempotency key", func() error { _, _, err := svc.Add(ctx, id, "SAREE-1", 1, "abc"); return err }},
		{"unknown sku", func() error { _, _, err := svc.Add(ctx, id, "GHOST-1", 1, "idem-key-0001"); return err }},
		{"set negative", func() error { _, err := svc.SetQuantity(ctx, id, "SAREE-1", -1); return err }},
		{"set unknown sku", func() error { _, err := svc.SetQuantity(ctx, id, "GHOST-1", 2); return err }},
	}
	for _, tc := range tests {
		if err := tc.call(); !errors.As(err, &invalid) {
			t.Errorf("%s: err = %v, want InvalidError", tc.name, err)
		}
	}
}

func TestServiceLimitsSurface(t *testing.T) {
	ctx := context.Background()
	svc := cart.NewService(cart.NewMemoryStore(), newFake())
	id := svc.Create()
	if _, _, err := svc.Add(ctx, id, "SAREE-1", cart.MaxQuantity, "idem-key-0001"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Add(ctx, id, "SAREE-1", 1, "idem-key-0002"); !errors.Is(err, cart.ErrQuantityLimit) {
		t.Errorf("err = %v, want ErrQuantityLimit", err)
	}
}

func TestServiceSetRemoveClear(t *testing.T) {
	ctx := context.Background()
	svc := cart.NewService(cart.NewMemoryStore(), newFake())
	id := svc.Create()

	if _, err := svc.SetQuantity(ctx, id, "SAREE-1", 3); err != nil {
		t.Fatal(err)
	}
	v, err := svc.SetQuantity(ctx, id, "SAREE-1", 2)
	if err != nil || v.Lines[0].Quantity != 2 || v.SubtotalCents != 40000 {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	v, err = svc.Remove(ctx, id, "SAREE-1")
	if err != nil || len(v.Lines) != 0 || v.SubtotalCents != 0 {
		t.Fatalf("after remove: v=%+v err=%v", v, err)
	}

	if _, err := svc.SetQuantity(ctx, id, "KURTA-1", 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.Clear(ctx, id); err != nil {
		t.Fatal(err)
	}
	if v, _ := svc.Get(ctx, id); len(v.Lines) != 0 {
		t.Errorf("lines after clear = %+v", v.Lines)
	}
}

func TestEmptyCartIsNotAnError(t *testing.T) {
	cat := newFake()
	svc := cart.NewService(cart.NewMemoryStore(), cat)
	v, err := svc.Get(context.Background(), cart.NewID())
	if err != nil || len(v.Lines) != 0 || v.Lines == nil || v.SubtotalCents != 0 {
		t.Errorf("v=%+v err=%v", v, err)
	}
	if cat.calls != 0 {
		t.Errorf("an empty cart should not call the catalog (calls=%d)", cat.calls)
	}
}

// A product can be unpublished while it sits in someone's cart. The line stays
// (so it can be removed) but costs nothing.
func TestDelistedProductStaysButIsNotCharged(t *testing.T) {
	ctx := context.Background()
	cat := newFake()
	svc := cart.NewService(cart.NewMemoryStore(), cat)
	id := svc.Create()
	for _, sku := range []string{"SAREE-1", "KURTA-1"} {
		if _, err := svc.SetQuantity(ctx, id, sku, 1); err != nil {
			t.Fatal(err)
		}
	}
	delete(cat.sold, "SAREE-1") // the catalog stops selling it

	v, err := svc.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if v.Lines[1].SKU != "SAREE-1" || v.Lines[1].Available || v.Lines[1].LineTotalCents != 0 {
		t.Errorf("delisted line = %+v", v.Lines[1])
	}
	if v.SubtotalCents != 4500 || v.ItemCount != 1 {
		t.Errorf("subtotal=%d count=%d, want only the available line", v.SubtotalCents, v.ItemCount)
	}
	if _, err := svc.Remove(ctx, id, "SAREE-1"); err != nil {
		t.Errorf("removing a delisted line must work: %v", err)
	}
}

func TestCatalogFailureIsReported(t *testing.T) {
	cat := newFake()
	svc := cart.NewService(cart.NewMemoryStore(), cat)
	id := svc.Create()
	if _, err := svc.SetQuantity(context.Background(), id, "SAREE-1", 1); err != nil {
		t.Fatal(err)
	}
	cat.err = errors.New("catalog down")
	if _, err := svc.Get(context.Background(), id); !errors.Is(err, cart.ErrUnavailable) {
		t.Errorf("err = %v, want ErrUnavailable when the catalog is down", err)
	}
}
