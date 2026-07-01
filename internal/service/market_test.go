package service

import (
	"context"
	"sync"
	"testing"

	"github.com/dshmmd/maze-challenge/internal/adapters/clock"
	"github.com/dshmmd/maze-challenge/internal/adapters/idgen"
	"github.com/dshmmd/maze-challenge/internal/adapters/memstore"
	"github.com/dshmmd/maze-challenge/internal/adapters/oracle"
	"github.com/dshmmd/maze-challenge/internal/domain"
)

func newMarket(t *testing.T) (*Market, *memstore.Store) {
	t.Helper()
	store := memstore.New()
	return NewMarket(store, idgen.Hex{}, clock.Real{}, oracle.NewMock(nil), DefaultConfig()), store
}

func mustCreate(t *testing.T, m *Market, in CreateItemInput) *domain.Item {
	t.Helper()
	it, err := m.CreateItem(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	return it
}

func TestPurchaseMovesFunds(t *testing.T) {
	m, store := newMarket(t)
	store.SeedWallet("seller", 0)
	store.SeedWallet("buyer", 500)
	it := mustCreate(t, m, CreateItemInput{Name: "Potion", Rarity: domain.Rare, Seller: "seller", Price: 100, Quantity: 2})

	res, err := m.Purchase(context.Background(), it.ID, "buyer", 2)
	if err != nil {
		t.Fatalf("Purchase: %v", err)
	}
	if res.Cost != 200 {
		t.Fatalf("cost = %d, want 200", res.Cost)
	}

	buyer, _ := m.GetWallet(context.Background(), "buyer")
	seller, _ := m.GetWallet(context.Background(), "seller")
	if buyer.Total != 300 {
		t.Fatalf("buyer total = %d, want 300", buyer.Total)
	}
	if seller.Total != 200 {
		t.Fatalf("seller total = %d, want 200", seller.Total)
	}
}

func TestPurchaseInsufficientFunds(t *testing.T) {
	m, store := newMarket(t)
	store.SeedWallet("seller", 0)
	store.SeedWallet("buyer", 50)
	it := mustCreate(t, m, CreateItemInput{Name: "Potion", Rarity: domain.Rare, Seller: "seller", Price: 100, Quantity: 1})

	if _, err := m.Purchase(context.Background(), it.ID, "buyer", 1); err != domain.ErrInsufficientFunds {
		t.Fatalf("Purchase err = %v, want ErrInsufficientFunds", err)
	}
	// The failed purchase must not have moved funds or stock.
	buyer, _ := m.GetWallet(context.Background(), "buyer")
	if buyer.Total != 50 {
		t.Fatalf("buyer charged on failed purchase: total=%d", buyer.Total)
	}
}

func TestPurchaseRejectsLegendary(t *testing.T) {
	m, store := newMarket(t)
	store.SeedWallet("seller", 0)
	store.SeedWallet("buyer", 10_000)
	// Listing a Legendary opens its auction; Price is the starting bid.
	it := mustCreate(t, m, CreateItemInput{Name: "Soul Reaver", Rarity: domain.Legendary, Seller: "seller", Price: 100})

	if _, err := m.Purchase(context.Background(), it.ID, "buyer", 1); err != domain.ErrLegendaryNotLimit {
		t.Fatalf("Purchase legendary err = %v, want ErrLegendaryNotLimit", err)
	}
}

// TestConcurrentPurchasesNoOversell is the money-safety guarantee under load
// (R1/R16/R17): many buyers race for a 1-unit listing; exactly one wins, the
// rest get a clean conflict, and no extra gold is created or destroyed.
func TestConcurrentPurchasesNoOversell(t *testing.T) {
	m, store := newMarket(t)
	store.SeedWallet("seller", 0)

	const buyers = 50
	for i := range buyers {
		store.SeedWallet(buyerName(i), 1_000)
	}
	it := mustCreate(t, m, CreateItemInput{Name: "Last Potion", Rarity: domain.Rare, Seller: "seller", Price: 100, Quantity: 1})

	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := range buyers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if _, err := m.Purchase(context.Background(), it.ID, buyerName(idx), 1); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if wins != 1 {
		t.Fatalf("winners = %d, want exactly 1", wins)
	}
	seller, _ := m.GetWallet(context.Background(), "seller")
	if seller.Total != 100 {
		t.Fatalf("seller total = %d, want exactly 100 (no oversell)", seller.Total)
	}
	final, _ := m.GetItem(context.Background(), it.ID)
	if final.Status != domain.ItemSoldOut {
		t.Fatalf("item status = %s, want sold_out", final.Status)
	}
}

func buyerName(i int) string {
	return "buyer-" + string(rune('A'+i%26)) + string(rune('0'+i/26))
}
