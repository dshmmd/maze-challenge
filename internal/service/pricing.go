package service

import (
	"context"
	"time"

	"github.com/dshmmd/maze-challenge/internal/domain"
)

// OraclePrice returns the advisory current price for an item from the external
// Price Oracle (D13: advisory/display-only). The oracle is flaky — it may be
// slow or return zero/negative values — so this method:
//   - bounds the call with a short timeout (a slow oracle never blocks a page);
//   - rejects non-positive readings and falls back to the last good value;
//   - caches each good reading so display degrades gracefully (R15).
//
// The boolean reports whether any price (fresh or cached) is available.
func (m *Market) OraclePrice(ctx context.Context, itemID string) (domain.Gold, bool) {
	cctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	price, err := m.oracle.BasePrice(cctx, itemID)
	if err == nil && price.IsPositive() {
		m.priceMu.Lock()
		m.lastGoodPrice[itemID] = price
		m.priceMu.Unlock()
		return price, true
	}

	// Bad/slow reading: serve the last good price if we have one.
	m.priceMu.RLock()
	cached, ok := m.lastGoodPrice[itemID]
	m.priceMu.RUnlock()
	return cached, ok
}
