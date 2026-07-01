// Package ports declares the interfaces the application core depends on.
//
// The domain and use-cases are written against these abstractions; concrete
// implementations live in internal/adapters. This is the "ports" side of the
// hexagonal architecture (D2): it lets external services be mocked, exactly as
// the challenge requires.
package ports

import (
	"context"
	"time"

	"github.com/dshmmd/maze-challenge/internal/domain"
)

// Clock abstracts the current time so time-dependent rules (auction windows,
// the 5-minute anti-snipe extension) are deterministic in tests. (D4)
type Clock interface {
	Now() time.Time
}

// PriceOracle is the unreliable external price feed. Implementations may be
// slow or return invalid prices (zero/negative); callers must treat the result
// defensively and never trust it blindly. (R15)
type PriceOracle interface {
	// BasePrice returns the current advertised base price for an item kind.
	// Implementations should respect ctx cancellation/timeout.
	BasePrice(ctx context.Context, itemID string) (domain.Gold, error)
}

// Repo is the transactional data access the application core uses. The
// "ForUpdate" reads lock the row for the duration of the surrounding
// transaction (in Postgres, SELECT … FOR UPDATE; in the in-memory store, a
// global lock), which is how the money-safety and single-fulfilment guarantees
// (R1, R13, R16, R17) are enforced without races.
type Repo interface {
	// Items
	CreateItem(ctx context.Context, item *domain.Item) error
	GetItem(ctx context.Context, id string) (*domain.Item, error)
	GetItemForUpdate(ctx context.Context, id string) (*domain.Item, error)
	UpdateItem(ctx context.Context, item *domain.Item) error
	ListItems(ctx context.Context) ([]domain.Item, error)

	// Wallets
	GetWalletForUpdate(ctx context.Context, guildID string) (*domain.Wallet, error)
	UpdateWallet(ctx context.Context, w *domain.Wallet) error

	// Auctions
	CreateAuction(ctx context.Context, a *domain.Auction) error
	GetAuction(ctx context.Context, id string) (*domain.Auction, error)
	GetAuctionForUpdate(ctx context.Context, id string) (*domain.Auction, error)
	GetActiveAuctionByItem(ctx context.Context, itemID string) (*domain.Auction, error)
	UpdateAuction(ctx context.Context, a *domain.Auction) error
	ListAuctions(ctx context.Context, activeOnly bool) ([]domain.Auction, error)
	DueAuctions(ctx context.Context, now time.Time) ([]domain.Auction, error)

	// Audit / limits
	AppendLedger(ctx context.Context, entry domain.LedgerEntry) error
	ListLedger(ctx context.Context, guildID string) ([]domain.LedgerEntry, error)
	// SpentSince sums a guild's committed outflows (debits + settlements) at or
	// after t — used to enforce the daily purchase cap (R14).
	SpentSince(ctx context.Context, guildID string, t time.Time) (domain.Gold, error)
}

// TxManager runs a unit of work atomically. The function receives a Repo bound
// to the transaction; if it returns an error the whole unit is rolled back.
// This keeps business logic in the service while the atomic boundary lives in
// the adapter (Postgres tx or in-memory lock). (D2/D11)
type TxManager interface {
	WithinTx(ctx context.Context, fn func(Repo) error) error
}

// IDGenerator produces unique identifiers for new entities.
type IDGenerator interface {
	NewID() string
}
