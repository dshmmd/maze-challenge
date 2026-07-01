// Package oracle provides a mock implementation of the external Price Oracle.
//
// The real oracle "updates base prices every 30s and is sometimes slow or
// returns wrong prices (zero or negative)". This mock can be configured to
// reproduce all of those failure modes so the core's defensive handling (R15)
// can be exercised in tests.
package oracle

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/dshmmd/maze-challenge/internal/domain"
)

// Mock is a configurable, in-memory Price Oracle. The zero value is usable but
// returns no prices; set Prices (and optionally the fault knobs) to drive it.
type Mock struct {
	mu sync.RWMutex

	// Prices is the per-item base price the oracle currently advertises.
	Prices map[string]domain.Gold

	// Latency, if non-zero, is how long BasePrice blocks before returning,
	// simulating a slow oracle. Honors context cancellation.
	Latency time.Duration

	// FaultRate in [0,1] is the probability a call returns a bad (zero or
	// negative) price instead of the real one, simulating oracle errors.
	FaultRate float64

	rng *rand.Rand
}

// NewMock returns a Mock seeded with the given prices.
func NewMock(prices map[string]domain.Gold) *Mock {
	if prices == nil {
		prices = map[string]domain.Gold{}
	}
	return &Mock{
		Prices: prices,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// BasePrice returns the advertised base price for an item, subject to the
// configured latency and fault rate. Callers must validate the result — this
// mock can deliberately return zero or negative values.
func (m *Mock) BasePrice(ctx context.Context, itemID string) (domain.Gold, error) {
	if m.Latency > 0 {
		select {
		case <-time.After(m.Latency):
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.FaultRate > 0 && m.rng != nil && m.rng.Float64() < m.FaultRate {
		// Simulate a misbehaving oracle: a non-positive price.
		return domain.Gold(-1), nil
	}
	return m.Prices[itemID], nil
}

// SetPrice updates the advertised price for an item (e.g. the 30s refresh).
func (m *Mock) SetPrice(itemID string, price domain.Gold) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Prices[itemID] = price
}
