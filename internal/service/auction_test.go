package service

import (
	"context"
	"testing"
	"time"

	"github.com/dshmmd/maze-challenge/internal/adapters/clock"
	"github.com/dshmmd/maze-challenge/internal/adapters/idgen"
	"github.com/dshmmd/maze-challenge/internal/adapters/memstore"
	"github.com/dshmmd/maze-challenge/internal/adapters/oracle"
	"github.com/dshmmd/maze-challenge/internal/domain"
)

func newMarketWithClock(t *testing.T) (*Market, *memstore.Store, *clock.Fake) {
	t.Helper()
	store := memstore.New()
	fake := clock.NewFake(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	m := NewMarket(store, idgen.Hex{}, fake, oracle.NewMock(nil), DefaultConfig())
	return m, store, fake
}

// TestAuctionReserveReleaseSettle walks the whole money path: a bid reserves,
// being outbid releases immediately (R7), and settlement debits the winner and
// credits the seller (R5/R7).
func TestAuctionReserveReleaseSettle(t *testing.T) {
	ctx := context.Background()
	m, store, fake := newMarketWithClock(t)
	store.SeedWallet("seller", 0)
	store.SeedWallet("g1", 1_000)
	store.SeedWallet("g2", 1_000)

	item := mustCreate(t, m, CreateItemInput{Name: "Eye of the Dragon", Rarity: domain.Legendary, Seller: "seller", Price: 100})

	// g1 bids 100 → 100 reserved.
	if _, err := m.BidOnItem(ctx, item.ID, "g1", 100); err != nil {
		t.Fatalf("g1 bid: %v", err)
	}
	if w, _ := m.GetWallet(ctx, "g1"); w.Reserved != 100 || w.Available() != 900 {
		t.Fatalf("g1 wallet = %+v, want reserved 100 / available 900", w)
	}

	// g2 outbids at 200 → g1's reservation released, g2 reserves 200.
	if _, err := m.BidOnItem(ctx, item.ID, "g2", 200); err != nil {
		t.Fatalf("g2 bid: %v", err)
	}
	if w, _ := m.GetWallet(ctx, "g1"); w.Reserved != 0 {
		t.Fatalf("g1 reserved = %d after being outbid, want 0", w.Reserved)
	}
	if w, _ := m.GetWallet(ctx, "g2"); w.Reserved != 200 {
		t.Fatalf("g2 reserved = %d, want 200", w.Reserved)
	}

	// Advance past the window and settle.
	fake.Advance(25 * time.Hour)
	n, err := m.SettleDueAuctions(ctx)
	if err != nil || n != 1 {
		t.Fatalf("SettleDueAuctions = %d, %v; want 1, nil", n, err)
	}

	if w, _ := m.GetWallet(ctx, "g2"); w.Total != 800 || w.Reserved != 0 {
		t.Fatalf("winner g2 wallet = %+v, want total 800 / reserved 0", w)
	}
	if w, _ := m.GetWallet(ctx, "seller"); w.Total != 200 {
		t.Fatalf("seller total = %d, want 200", w.Total)
	}
	if it, _ := m.GetItem(ctx, item.ID); it.Status != domain.ItemSoldOut {
		t.Fatalf("item status = %s, want sold_out", it.Status)
	}
}

// TestAuctionNoBidsReturnsItem covers R8: an auction that closes with no bids
// returns the item to available.
func TestAuctionNoBidsReturnsItem(t *testing.T) {
	ctx := context.Background()
	m, store, fake := newMarketWithClock(t)
	store.SeedWallet("seller", 0)
	item := mustCreate(t, m, CreateItemInput{Name: "Lonely Blade", Rarity: domain.Legendary, Seller: "seller", Price: 100})

	fake.Advance(25 * time.Hour)
	if _, err := m.SettleDueAuctions(ctx); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if it, _ := m.GetItem(ctx, item.ID); it.Status != domain.ItemAvailable {
		t.Fatalf("item status = %s, want available", it.Status)
	}
}

// TestBidRejectsOwnItemAndLowBid covers R10/R11 through the service.
func TestBidRejectsOwnItemAndLowBid(t *testing.T) {
	ctx := context.Background()
	m, store, _ := newMarketWithClock(t)
	store.SeedWallet("seller", 1_000)
	store.SeedWallet("g1", 1_000)
	item := mustCreate(t, m, CreateItemInput{Name: "Crown", Rarity: domain.Legendary, Seller: "seller", Price: 100})

	if _, err := m.BidOnItem(ctx, item.ID, "seller", 100); err != domain.ErrCannotBidOwnItem {
		t.Fatalf("seller self-bid err = %v, want ErrCannotBidOwnItem", err)
	}
	if _, err := m.BidOnItem(ctx, item.ID, "g1", 100); err != nil {
		t.Fatalf("g1 bid 100: %v", err)
	}
	// 104 is < 5% over 100 → rejected.
	if _, err := m.BidOnItem(ctx, item.ID, "g1", 104); err != domain.ErrBidTooLow {
		t.Fatalf("low bid err = %v, want ErrBidTooLow", err)
	}
}

// TestDailyCapBlocksOverspend covers R14/D12 via a limit-order purchase.
func TestDailyCapBlocksOverspend(t *testing.T) {
	ctx := context.Background()
	m, store, _ := newMarketWithClock(t)
	store.SeedWallet("seller", 0)
	store.SeedGuild("buyer", 10_000, 150) // cap 150/day

	item := mustCreate(t, m, CreateItemInput{Name: "Potion", Rarity: domain.Common, Seller: "seller", Price: 100, Quantity: 10})

	if _, err := m.Purchase(ctx, item.ID, "buyer", 1); err != nil {
		t.Fatalf("first buy (100): %v", err)
	}
	// Second buy would push the day to 200 > cap 150.
	if _, err := m.Purchase(ctx, item.ID, "buyer", 1); err != domain.ErrDailyCapExceeded {
		t.Fatalf("second buy err = %v, want ErrDailyCapExceeded", err)
	}
}
