// Package service holds the application use-cases: it orchestrates the domain
// and the ports, but contains no transport or storage detail of its own.
package service

import (
	"context"
	"sync"
	"time"

	"github.com/dshmmd/maze-challenge/internal/domain"
	"github.com/dshmmd/maze-challenge/internal/ports"
)

// Config holds tunable market parameters (D4: auction timing is configurable).
type Config struct {
	AuctionWindow    time.Duration // default auction duration (e.g. 24h)
	AuctionExtension time.Duration // anti-snipe extension (e.g. 5m)
}

// DefaultConfig returns sensible defaults per the spec.
func DefaultConfig() Config {
	return Config{AuctionWindow: 24 * time.Hour, AuctionExtension: 5 * time.Minute}
}

// Market is the marketplace use-case service. It depends only on ports, so it
// is exercised in tests against the in-memory store and runs in production
// against Postgres without changing a line.
type Market struct {
	tx     ports.TxManager
	ids    ports.IDGenerator
	clock  ports.Clock
	oracle ports.PriceOracle
	cfg    Config

	// lastGoodPrice caches the most recent valid oracle reading per item so a
	// flaky reading (zero/negative/slow) degrades gracefully (R15, D13).
	priceMu       sync.RWMutex
	lastGoodPrice map[string]domain.Gold
}

// NewMarket wires the service with its dependencies.
func NewMarket(tx ports.TxManager, ids ports.IDGenerator, clock ports.Clock, oracle ports.PriceOracle, cfg Config) *Market {
	return &Market{
		tx: tx, ids: ids, clock: clock, oracle: oracle, cfg: cfg,
		lastGoodPrice: map[string]domain.Gold{},
	}
}

// startOfUTCDay returns midnight UTC for t, the daily-cap window boundary (D12).
func startOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// CreateItemInput is the request to list a new item.
type CreateItemInput struct {
	Name     string
	Rarity   domain.Rarity
	Seller   string
	Price    domain.Gold
	Quantity int
}

// CreateItem validates and persists a new listing. For Common/Rare this is a
// limit-order listing. For Legendary, listing the item for sale immediately
// opens its (single) auction (R3), using in.Price as the starting bid and the
// configured window — so a Legendary item appears under GET /auctions at once.
func (m *Market) CreateItem(ctx context.Context, in CreateItemInput) (*domain.Item, error) {
	now := m.clock.Now()
	item, err := domain.NewItem(m.ids.NewID(), in.Name, in.Rarity, in.Seller, in.Price, in.Quantity, now)
	if err != nil {
		return nil, err
	}
	// Legendary needs a positive starting bid even though Item.Price is unused.
	if item.Rarity.AuctionOnly() && !in.Price.IsPositive() {
		return nil, domain.ErrNonPositiveAmount
	}

	err = m.tx.WithinTx(ctx, func(r ports.Repo) error {
		if err := r.CreateItem(ctx, item); err != nil {
			return err
		}
		if item.Rarity.AuctionOnly() {
			return m.openAuction(ctx, r, item, in.Price, now)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return item, nil
}

// GetItem returns a single item by ID.
func (m *Market) GetItem(ctx context.Context, id string) (*domain.Item, error) {
	var item *domain.Item
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		got, err := r.GetItem(ctx, id)
		if err != nil {
			return err
		}
		item = got
		return nil
	})
	return item, err
}

// ListItems returns all items.
func (m *Market) ListItems(ctx context.Context) ([]domain.Item, error) {
	var items []domain.Item
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		got, err := r.ListItems(ctx)
		if err != nil {
			return err
		}
		items = got
		return nil
	})
	return items, err
}

// GetWallet returns a guild's wallet (total, reserved, and thus available).
func (m *Market) GetWallet(ctx context.Context, guildID string) (*domain.Wallet, error) {
	var w *domain.Wallet
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		got, err := r.GetWalletForUpdate(ctx, guildID)
		if err != nil {
			return err
		}
		w = got
		return nil
	})
	return w, err
}

// PurchaseResult reports the outcome of a limit-order purchase.
type PurchaseResult struct {
	Item *domain.Item
	Cost domain.Gold
}

// Purchase executes a limit-order buy of `qty` units of `itemID` by `buyer`.
//
// The entire read-modify-write runs in one transaction with the item and both
// wallets locked, so the guarantees hold even under concurrent buyers:
//   - the item can never be over-sold or sold twice (R1, R16, R17);
//   - the buyer can never over-commit (R13) — the debit checks available funds;
//   - a guild cannot buy its own item (R10);
//   - Legendary items are rejected here (auction-only).
//
// Funds move buyer → seller, and both movements are recorded in the ledger (R9).
func (m *Market) Purchase(ctx context.Context, itemID, buyer string, qty int) (*PurchaseResult, error) {
	if qty < 1 {
		return nil, domain.ErrInvalidQuantity
	}

	var result *PurchaseResult
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		item, err := r.GetItemForUpdate(ctx, itemID)
		if err != nil {
			return err
		}

		buyerWallet, err := r.GetWalletForUpdate(ctx, buyer)
		if err != nil {
			return err
		}
		sellerWallet, err := r.GetWalletForUpdate(ctx, item.SellerGuildID)
		if err != nil {
			return err
		}

		// Apply the sale rules (own-item, stock, availability) and get the cost.
		cost, err := item.SellTo(buyer, qty)
		if err != nil {
			return err
		}

		// Enforce the guild's daily purchase cap (R14, D12): purchases count
		// toward the cap alongside auction wins.
		now := m.clock.Now()
		spent, err := r.SpentSince(ctx, buyer, startOfUTCDay(now))
		if err != nil {
			return err
		}
		if !buyerWallet.WithinDailyCap(spent, cost) {
			return domain.ErrDailyCapExceeded
		}

		// Move funds. Debit checks available balance, so this fails closed.
		if err := buyerWallet.Debit(cost); err != nil {
			return err
		}
		if err := sellerWallet.Credit(cost); err != nil {
			return err
		}

		if err := r.UpdateItem(ctx, item); err != nil {
			return err
		}
		if err := r.UpdateWallet(ctx, buyerWallet); err != nil {
			return err
		}
		if err := r.UpdateWallet(ctx, sellerWallet); err != nil {
			return err
		}

		if err := r.AppendLedger(ctx, domain.LedgerEntry{
			ID: m.ids.NewID(), GuildID: buyer, Type: domain.LedgerDebit,
			Amount: cost, ItemID: itemID, Memo: "limit-order purchase", At: now,
		}); err != nil {
			return err
		}
		if err := r.AppendLedger(ctx, domain.LedgerEntry{
			ID: m.ids.NewID(), GuildID: item.SellerGuildID, Type: domain.LedgerCredit,
			Amount: cost, ItemID: itemID, Memo: "limit-order sale", At: now,
		}); err != nil {
			return err
		}

		result = &PurchaseResult{Item: item, Cost: cost}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
