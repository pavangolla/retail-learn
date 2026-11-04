package catalog_test

import (
	"testing"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog/repotest"
)

// The in-memory repo must satisfy the same contract as the MongoDB store.
func TestMemoryRepoContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) repotest.Store { return catalog.NewMemoryRepo() })
}
