// Package postgres is the production data adapter (D1). It implements
// ports.TxManager and ports.Repo over a pgx connection pool, using
// SELECT … FOR UPDATE row locks so the money-safety and single-fulfilment
// guarantees hold under concurrency without application-level races.
package postgres

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dshmmd/maze-challenge/internal/domain"
	"github.com/dshmmd/maze-challenge/internal/ports"
)

//go:embed schema.sql
var schemaSQL string

// Store is the Postgres-backed TxManager.
type Store struct {
	pool *pgxpool.Pool
}

// Connect opens a pool, verifies connectivity, and applies the schema.
func Connect(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// SeedGuild inserts or resets a guild wallet (demo/setup). Idempotent.
func (s *Store) SeedGuild(ctx context.Context, guildID string, total, dailyCap domain.Gold) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO guilds (id, total, reserved, daily_cap)
		VALUES ($1, $2, 0, $3)
		ON CONFLICT (id) DO UPDATE SET total = EXCLUDED.total, daily_cap = EXCLUDED.daily_cap`,
		guildID, int64(total), int64(dailyCap))
	return err
}

// WithinTx runs fn inside a single transaction, committing on success and
// rolling back on error (D11). The bound Repo issues row locks for "ForUpdate"
// reads.
func (s *Store) WithinTx(ctx context.Context, fn func(ports.Repo) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit

	if err := fn(&txRepo{tx: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// txRepo is a Repo bound to one pgx transaction.
type txRepo struct {
	tx pgx.Tx
}

// --- Items ---

func (t *txRepo) CreateItem(ctx context.Context, i *domain.Item) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO items (id, name, rarity, seller_guild_id, price, quantity, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		i.ID, i.Name, string(i.Rarity), i.SellerGuildID, int64(i.Price), i.Quantity, string(i.Status), i.CreatedAt)
	return err
}

func scanItem(row pgx.Row) (*domain.Item, error) {
	var i domain.Item
	var rarity, status string
	var price int64
	err := row.Scan(&i.ID, &i.Name, &rarity, &i.SellerGuildID, &price, &i.Quantity, &status, &i.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrItemNotFound
	}
	if err != nil {
		return nil, err
	}
	i.Rarity = domain.Rarity(rarity)
	i.Status = domain.ItemStatus(status)
	i.Price = domain.Gold(price)
	return &i, nil
}

const itemCols = `id, name, rarity, seller_guild_id, price, quantity, status, created_at`

func (t *txRepo) GetItem(ctx context.Context, id string) (*domain.Item, error) {
	return scanItem(t.tx.QueryRow(ctx, `SELECT `+itemCols+` FROM items WHERE id=$1`, id))
}

func (t *txRepo) GetItemForUpdate(ctx context.Context, id string) (*domain.Item, error) {
	return scanItem(t.tx.QueryRow(ctx, `SELECT `+itemCols+` FROM items WHERE id=$1 FOR UPDATE`, id))
}

func (t *txRepo) UpdateItem(ctx context.Context, i *domain.Item) error {
	_, err := t.tx.Exec(ctx, `
		UPDATE items SET name=$2, rarity=$3, seller_guild_id=$4, price=$5, quantity=$6, status=$7
		WHERE id=$1`,
		i.ID, i.Name, string(i.Rarity), i.SellerGuildID, int64(i.Price), i.Quantity, string(i.Status))
	return err
}

func (t *txRepo) ListItems(ctx context.Context) ([]domain.Item, error) {
	rows, err := t.tx.Query(ctx, `SELECT `+itemCols+` FROM items ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Item, 0)
	for rows.Next() {
		i, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}

// --- Wallets ---

func (t *txRepo) GetWalletForUpdate(ctx context.Context, guildID string) (*domain.Wallet, error) {
	var w domain.Wallet
	var total, reserved, dailyCap int64
	err := t.tx.QueryRow(ctx, `SELECT id, total, reserved, daily_cap FROM guilds WHERE id=$1 FOR UPDATE`, guildID).
		Scan(&w.GuildID, &total, &reserved, &dailyCap)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrWalletNotFound
	}
	if err != nil {
		return nil, err
	}
	w.Total, w.Reserved, w.DailyCap = domain.Gold(total), domain.Gold(reserved), domain.Gold(dailyCap)
	return &w, nil
}

func (t *txRepo) UpdateWallet(ctx context.Context, w *domain.Wallet) error {
	_, err := t.tx.Exec(ctx, `UPDATE guilds SET total=$2, reserved=$3, daily_cap=$4 WHERE id=$1`,
		w.GuildID, int64(w.Total), int64(w.Reserved), int64(w.DailyCap))
	return err
}

// --- Auctions ---

const auctionCols = `id, item_id, seller_guild_id, start_price, ends_at, extension_seconds, status, created_at`

func (t *txRepo) scanAuction(ctx context.Context, row pgx.Row) (*domain.Auction, error) {
	var a domain.Auction
	var startPrice, extSecs int64
	var status string
	err := row.Scan(&a.ID, &a.ItemID, &a.SellerGuildID, &startPrice, &a.EndsAt, &extSecs, &status, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAuctionNotFound
	}
	if err != nil {
		return nil, err
	}
	a.StartPrice = domain.Gold(startPrice)
	a.Extension = time.Duration(extSecs) * time.Second
	a.Status = domain.AuctionStatus(status)
	bids, err := t.loadBids(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	a.Bids = bids
	return &a, nil
}

func (t *txRepo) loadBids(ctx context.Context, auctionID string) ([]domain.Bid, error) {
	rows, err := t.tx.Query(ctx, `SELECT id, guild_id, amount, active, placed_at FROM bids WHERE auction_id=$1 ORDER BY placed_at`, auctionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Bid, 0)
	for rows.Next() {
		var b domain.Bid
		var amount int64
		if err := rows.Scan(&b.ID, &b.GuildID, &amount, &b.Active, &b.PlacedAt); err != nil {
			return nil, err
		}
		b.Amount = domain.Gold(amount)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (t *txRepo) CreateAuction(ctx context.Context, a *domain.Auction) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO auctions (`+auctionCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		a.ID, a.ItemID, a.SellerGuildID, int64(a.StartPrice), a.EndsAt,
		int64(a.Extension/time.Second), string(a.Status), a.CreatedAt)
	return err
}

func (t *txRepo) GetAuction(ctx context.Context, id string) (*domain.Auction, error) {
	return t.scanAuction(ctx, t.tx.QueryRow(ctx, `SELECT `+auctionCols+` FROM auctions WHERE id=$1`, id))
}

func (t *txRepo) GetAuctionForUpdate(ctx context.Context, id string) (*domain.Auction, error) {
	return t.scanAuction(ctx, t.tx.QueryRow(ctx, `SELECT `+auctionCols+` FROM auctions WHERE id=$1 FOR UPDATE`, id))
}

func (t *txRepo) GetActiveAuctionByItem(ctx context.Context, itemID string) (*domain.Auction, error) {
	return t.scanAuction(ctx, t.tx.QueryRow(ctx,
		`SELECT `+auctionCols+` FROM auctions WHERE item_id=$1 AND status='active' FOR UPDATE`, itemID))
}

func (t *txRepo) UpdateAuction(ctx context.Context, a *domain.Auction) error {
	if _, err := t.tx.Exec(ctx, `UPDATE auctions SET ends_at=$2, status=$3 WHERE id=$1`,
		a.ID, a.EndsAt, string(a.Status)); err != nil {
		return err
	}
	// Re-materialize the bid set for this auction (small N, within the tx).
	if _, err := t.tx.Exec(ctx, `DELETE FROM bids WHERE auction_id=$1`, a.ID); err != nil {
		return err
	}
	for _, b := range a.Bids {
		if _, err := t.tx.Exec(ctx, `
			INSERT INTO bids (id, auction_id, guild_id, amount, active, placed_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			b.ID, a.ID, b.GuildID, int64(b.Amount), b.Active, b.PlacedAt); err != nil {
			return err
		}
	}
	return nil
}

func (t *txRepo) ListAuctions(ctx context.Context, activeOnly bool) ([]domain.Auction, error) {
	q := `SELECT ` + auctionCols + ` FROM auctions`
	if activeOnly {
		q += ` WHERE status='active'`
	}
	q += ` ORDER BY created_at`
	return t.queryAuctions(ctx, q)
}

func (t *txRepo) DueAuctions(ctx context.Context, now time.Time) ([]domain.Auction, error) {
	return t.queryAuctions(ctx,
		`SELECT `+auctionCols+` FROM auctions WHERE status='active' AND ends_at<=$1 FOR UPDATE`, now)
}

func (t *txRepo) queryAuctions(ctx context.Context, q string, args ...any) ([]domain.Auction, error) {
	rows, err := t.tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	// Collect IDs first; loadBids issues its own queries, so the outer rows must
	// be closed before reusing the connection.
	type row struct {
		a       domain.Auction
		extSecs int64
		status  string
	}
	var staged []row
	for rows.Next() {
		var r row
		var startPrice int64
		if err := rows.Scan(&r.a.ID, &r.a.ItemID, &r.a.SellerGuildID, &startPrice, &r.a.EndsAt, &r.extSecs, &r.status, &r.a.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		r.a.StartPrice = domain.Gold(startPrice)
		staged = append(staged, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]domain.Auction, 0, len(staged))
	for _, r := range staged {
		r.a.Extension = time.Duration(r.extSecs) * time.Second
		r.a.Status = domain.AuctionStatus(r.status)
		bids, err := t.loadBids(ctx, r.a.ID)
		if err != nil {
			return nil, err
		}
		r.a.Bids = bids
		out = append(out, r.a)
	}
	return out, nil
}

// --- Ledger / limits ---

func (t *txRepo) AppendLedger(ctx context.Context, e domain.LedgerEntry) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO ledger (id, guild_id, type, amount, item_id, memo, at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		e.ID, e.GuildID, string(e.Type), int64(e.Amount), e.ItemID, e.Memo, e.At)
	return err
}

func (t *txRepo) ListLedger(ctx context.Context, guildID string) ([]domain.LedgerEntry, error) {
	rows, err := t.tx.Query(ctx, `
		SELECT id, guild_id, type, amount, item_id, memo, at FROM ledger
		WHERE guild_id=$1 ORDER BY at`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.LedgerEntry, 0)
	for rows.Next() {
		var e domain.LedgerEntry
		var typ string
		var amount int64
		if err := rows.Scan(&e.ID, &e.GuildID, &typ, &amount, &e.ItemID, &e.Memo, &e.At); err != nil {
			return nil, err
		}
		e.Type = domain.LedgerEntryType(typ)
		e.Amount = domain.Gold(amount)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (t *txRepo) SpentSince(ctx context.Context, guildID string, since time.Time) (domain.Gold, error) {
	var total int64
	err := t.tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount),0) FROM ledger
		WHERE guild_id=$1 AND at>=$2 AND type IN ('debit','settle')`, guildID, since).Scan(&total)
	if err != nil {
		return 0, err
	}
	return domain.Gold(total), nil
}
