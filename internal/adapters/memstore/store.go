// Package memstore is an in-memory implementation of the storage ports.
//
// It exists so the core can run and be tested with zero infrastructure, and so
// concurrency behavior can be exercised in unit tests. Atomicity is provided by
// a single store-wide mutex held for the duration of each unit of work: coarse,
// but it gives the same all-or-nothing, no-interleaving guarantee that a
// Postgres serializable transaction would. The Postgres adapter (next slice)
// implements the same ports with real row locking.
package memstore

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/dshmmd/maze-challenge/internal/domain"
	"github.com/dshmmd/maze-challenge/internal/ports"
)

// Store holds all state and serializes units of work.
type Store struct {
	mu       sync.Mutex
	items    map[string]domain.Item
	wallets  map[string]domain.Wallet
	auctions map[string]domain.Auction
	ledger   []domain.LedgerEntry
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		items:    map[string]domain.Item{},
		wallets:  map[string]domain.Wallet{},
		auctions: map[string]domain.Auction{},
	}
}

// SeedWallet creates or replaces a guild wallet with no daily cap (helper).
func (s *Store) SeedWallet(guildID string, total domain.Gold) {
	s.SeedGuild(guildID, total, 0)
}

// SeedGuild creates or replaces a guild wallet with a daily spend cap (0 =
// unlimited). Used for setup/demo/tests.
func (s *Store) SeedGuild(guildID string, total, dailyCap domain.Gold) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wallets[guildID] = domain.Wallet{GuildID: guildID, Total: total, DailyCap: dailyCap}
}

// ListItemsForTest returns a snapshot of all items. It is an inspection helper
// for tests and demos; production reads go through WithinTx/Repo.
func (s *Store) ListItemsForTest() ([]domain.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	return out, nil
}

// WithinTx runs fn while holding the store lock, giving it an atomic,
// non-interleaving view. A returned error discards staged changes.
func (s *Store) WithinTx(ctx context.Context, fn func(ports.Repo) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// The transaction stages changes on copies; they are committed to the
	// store only if fn succeeds, so a mid-way error leaves no partial writes.
	tx := &txRepo{
		store:    s,
		items:    map[string]domain.Item{},
		wallets:  map[string]domain.Wallet{},
		auctions: map[string]domain.Auction{},
	}
	if err := fn(tx); err != nil {
		return err
	}
	tx.commit()
	return nil
}

// txRepo is a Repo bound to one unit of work. It reads through to the store and
// buffers writes until commit.
type txRepo struct {
	store    *Store
	items    map[string]domain.Item    // staged item writes
	wallets  map[string]domain.Wallet  // staged wallet writes
	auctions map[string]domain.Auction // staged auction writes
	ledger   []domain.LedgerEntry      // staged ledger appends
}

func (t *txRepo) commit() {
	maps.Copy(t.store.items, t.items)
	maps.Copy(t.store.wallets, t.wallets)
	maps.Copy(t.store.auctions, t.auctions)
	t.store.ledger = append(t.store.ledger, t.ledger...)
}

func (t *txRepo) CreateItem(_ context.Context, item *domain.Item) error {
	if _, ok := t.store.items[item.ID]; ok {
		return domain.ErrItemNotFound // ID collision; treated as a programming error
	}
	t.items[item.ID] = *item
	return nil
}

func (t *txRepo) GetItem(_ context.Context, id string) (*domain.Item, error) {
	if it, ok := t.items[id]; ok {
		cp := it
		return &cp, nil
	}
	it, ok := t.store.items[id]
	if !ok {
		return nil, domain.ErrItemNotFound
	}
	cp := it
	return &cp, nil
}

// GetItemForUpdate is identical to GetItem here: the store-wide lock already
// serializes units of work, so the "lock" is implicit.
func (t *txRepo) GetItemForUpdate(ctx context.Context, id string) (*domain.Item, error) {
	return t.GetItem(ctx, id)
}

func (t *txRepo) UpdateItem(_ context.Context, item *domain.Item) error {
	t.items[item.ID] = *item
	return nil
}

func (t *txRepo) ListItems(_ context.Context) ([]domain.Item, error) {
	out := make([]domain.Item, 0, len(t.store.items))
	for _, it := range t.store.items {
		if staged, ok := t.items[it.ID]; ok {
			out = append(out, staged)
			continue
		}
		out = append(out, it)
	}
	return out, nil
}

func (t *txRepo) GetWalletForUpdate(_ context.Context, guildID string) (*domain.Wallet, error) {
	if w, ok := t.wallets[guildID]; ok {
		cp := w
		return &cp, nil
	}
	w, ok := t.store.wallets[guildID]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	cp := w
	return &cp, nil
}

func (t *txRepo) UpdateWallet(_ context.Context, w *domain.Wallet) error {
	t.wallets[w.GuildID] = *w
	return nil
}

func (t *txRepo) AppendLedger(_ context.Context, entry domain.LedgerEntry) error {
	t.ledger = append(t.ledger, entry)
	return nil
}

func (t *txRepo) getAuction(id string) (domain.Auction, bool) {
	if a, ok := t.auctions[id]; ok {
		return a, true
	}
	a, ok := t.store.auctions[id]
	return a, ok
}

func (t *txRepo) CreateAuction(_ context.Context, a *domain.Auction) error {
	t.auctions[a.ID] = cloneAuction(*a)
	return nil
}

func (t *txRepo) GetAuction(_ context.Context, id string) (*domain.Auction, error) {
	a, ok := t.getAuction(id)
	if !ok {
		return nil, domain.ErrAuctionNotFound
	}
	cp := cloneAuction(a)
	return &cp, nil
}

func (t *txRepo) GetAuctionForUpdate(ctx context.Context, id string) (*domain.Auction, error) {
	return t.GetAuction(ctx, id)
}

func (t *txRepo) GetActiveAuctionByItem(_ context.Context, itemID string) (*domain.Auction, error) {
	// Staged writes take precedence over committed state per ID.
	seen := map[string]bool{}
	for id, a := range t.auctions {
		seen[id] = true
		if a.ItemID == itemID && a.Status == domain.AuctionActive {
			cp := cloneAuction(a)
			return &cp, nil
		}
	}
	for id, a := range t.store.auctions {
		if seen[id] {
			continue
		}
		if a.ItemID == itemID && a.Status == domain.AuctionActive {
			cp := cloneAuction(a)
			return &cp, nil
		}
	}
	return nil, domain.ErrAuctionNotFound
}

func (t *txRepo) UpdateAuction(_ context.Context, a *domain.Auction) error {
	t.auctions[a.ID] = cloneAuction(*a)
	return nil
}

func (t *txRepo) ListAuctions(_ context.Context, activeOnly bool) ([]domain.Auction, error) {
	merged := map[string]domain.Auction{}
	maps.Copy(merged, t.store.auctions)
	maps.Copy(merged, t.auctions)
	out := make([]domain.Auction, 0, len(merged))
	for _, a := range merged {
		if activeOnly && a.Status != domain.AuctionActive {
			continue
		}
		out = append(out, cloneAuction(a))
	}
	return out, nil
}

func (t *txRepo) DueAuctions(_ context.Context, now time.Time) ([]domain.Auction, error) {
	merged := map[string]domain.Auction{}
	maps.Copy(merged, t.store.auctions)
	maps.Copy(merged, t.auctions)
	out := make([]domain.Auction, 0)
	for _, a := range merged {
		if a.Due(now) {
			out = append(out, cloneAuction(a))
		}
	}
	return out, nil
}

func (t *txRepo) ListLedger(_ context.Context, guildID string) ([]domain.LedgerEntry, error) {
	out := make([]domain.LedgerEntry, 0)
	for _, e := range t.store.ledger {
		if e.GuildID == guildID {
			out = append(out, e)
		}
	}
	for _, e := range t.ledger {
		if e.GuildID == guildID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (t *txRepo) SpentSince(_ context.Context, guildID string, since time.Time) (domain.Gold, error) {
	var total domain.Gold
	for _, e := range t.store.ledger {
		if e.GuildID != guildID || e.At.Before(since) {
			continue
		}
		if e.Type == domain.LedgerDebit || e.Type == domain.LedgerSettle {
			total += e.Amount
		}
	}
	return total, nil
}

// cloneAuction deep-copies an auction so staged/returned values never alias the
// stored slice of bids.
func cloneAuction(a domain.Auction) domain.Auction {
	cp := a
	cp.Bids = append([]domain.Bid(nil), a.Bids...)
	return cp
}
