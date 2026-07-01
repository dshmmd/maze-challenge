package service

import (
	"context"
	"log"
	"time"
)

// RunSettlementWorker periodically settles auctions whose window has closed
// (R6/R8 settlement, D4). It runs until ctx is cancelled. The tick interval
// controls how promptly expired auctions are finalized; settlement itself is
// idempotent, so a missed or doubled tick is harmless.
func (m *Market) RunSettlementWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := m.SettleDueAuctions(ctx)
			if err != nil {
				log.Printf("settlement worker: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("settlement worker: settled %d auction(s)", n)
			}
		}
	}
}
