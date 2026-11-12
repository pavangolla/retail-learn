package cart_test

import (
	"testing"

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
	"github.com/pavangolla/retail-learn/services/cart/internal/cart/carttest"
)

func TestMemoryStoreContract(t *testing.T) {
	carttest.Run(t, func(t *testing.T) cart.Store { return cart.NewMemoryStore() })
}
