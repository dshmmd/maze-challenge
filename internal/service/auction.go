package service

import (
	"context"
	"time"

	"github.com/dshmmd/maze-challenge/internal/domain"
	"github.com/dshmmd/maze-challenge/internal/ports"
)

// openAuction creates an active auction for a Legendary item inside an existing
// transaction. It enforces a single active auction per item (R3) and moves the
// item to in_auction. startPrice is the minimum first bid.
func (m *Market) openAuction(ctx context.Context, r ports.Repo, item *domain.Item, startPrice domain.Gold, now time.Time) error {
	if !startPrice.IsPositive() {
		return domain.ErrNonPositiveAmount
	}
	if _, err := r.GetActiveAuctionByItem(ctx, item.ID); err == nil {
		return domain.ErrAuctionExists
	} else if err != domain.ErrAuctionNotFound {
		return err
	}

	a := &domain.Auction{
		ID:            m.ids.NewID(),
		ItemID:        item.ID,
		SellerGuildID: item.SellerGuildID,
		StartPrice:    startPrice,
		EndsAt:        now.Add(m.cfg.AuctionWindow),
		Extension:     m.cfg.AuctionExtension,
		Status:        domain.AuctionActive,
		CreatedAt:     now,
	}
	if err := r.CreateAuction(ctx, a); err != nil {
		return err
	}
	item.Status = domain.ItemInAuction
	return r.UpdateItem(ctx, item)
}

// BidOnItem records a bid by `bidder` on the active auction of `itemID`. Funds
// are reserved (not debited, R7); the previous leader's reservation is released
// immediately as they are now outbid. The 5% increment, own-item, and window
// rules live in the domain. The whole unit runs under lock, so concurrent bids
// are linearized (R17).
func (m *Market) BidOnItem(ctx context.Context, itemID, bidder string, amount domain.Gold) (*domain.Auction, error) {
	if !amount.IsPositive() {
		return nil, domain.ErrNonPositiveAmount
	}

	var result *domain.Auction
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		active, err := r.GetActiveAuctionByItem(ctx, itemID)
		if err != nil {
			return err
		}
		a, err := r.GetAuctionForUpdate(ctx, active.ID)
		if err != nil {
			return err
		}

		bidderWallet, err := r.GetWalletForUpdate(ctx, bidder)
		if err != nil {
			return err
		}

		now := m.clock.Now()
		unseated, err := a.PlaceBid(m.ids.NewID(), bidder, amount, now)
		if err != nil {
			return err
		}

		// Daily cap (R14, D12): a bid that could win must keep the guild's
		// realized daily spend within its cap. Reservations do not count; only
		// realized buys and auction wins do.
		spent, err := r.SpentSince(ctx, bidder, startOfUTCDay(now))
		if err != nil {
			return err
		}
		if !bidderWallet.WithinDailyCap(spent, amount) {
			return domain.ErrDailyCapExceeded
		}

		// Release the previous leader's reservation (R7). If the same guild is
		// raising its own bid, operate on the one wallet instance to avoid a
		// stale overwrite.
		if unseated != nil {
			if unseated.GuildID == bidder {
				if err := bidderWallet.Release(unseated.Amount); err != nil {
					return err
				}
			} else {
				prevWallet, err := r.GetWalletForUpdate(ctx, unseated.GuildID)
				if err != nil {
					return err
				}
				if err := prevWallet.Release(unseated.Amount); err != nil {
					return err
				}
				if err := r.UpdateWallet(ctx, prevWallet); err != nil {
					return err
				}
				if err := r.AppendLedger(ctx, domain.LedgerEntry{
					ID: m.ids.NewID(), GuildID: unseated.GuildID, Type: domain.LedgerRelease,
					Amount: unseated.Amount, ItemID: a.ItemID, Memo: "outbid release", At: now,
				}); err != nil {
					return err
				}
			}
		}

		// Reserve the new bid (R7, R13 — checks available funds).
		if err := bidderWallet.Reserve(amount); err != nil {
			return err
		}
		if err := r.UpdateWallet(ctx, bidderWallet); err != nil {
			return err
		}
		if err := r.AppendLedger(ctx, domain.LedgerEntry{
			ID: m.ids.NewID(), GuildID: bidder, Type: domain.LedgerReserve,
			Amount: amount, ItemID: a.ItemID, Memo: "bid reserve", At: now,
		}); err != nil {
			return err
		}

		if err := r.UpdateAuction(ctx, a); err != nil {
			return err
		}
		result = a
		return nil
	})
	return result, err
}

// CancelBidOnItem withdraws a non-leading bid on the active auction of `itemID`
// (R12). Outbid reservations are already released, so no funds move; this only
// updates the auction record.
func (m *Market) CancelBidOnItem(ctx context.Context, itemID, bidID, guild string) (*domain.Auction, error) {
	var result *domain.Auction
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		active, err := r.GetActiveAuctionByItem(ctx, itemID)
		if err != nil {
			return err
		}
		a, err := r.GetAuctionForUpdate(ctx, active.ID)
		if err != nil {
			return err
		}
		if _, err := a.CancelBid(bidID, guild); err != nil {
			return err
		}
		if err := r.UpdateAuction(ctx, a); err != nil {
			return err
		}
		result = a
		return nil
	})
	return result, err
}

// ListAuctions returns auctions, optionally only active ones.
func (m *Market) ListAuctions(ctx context.Context, activeOnly bool) ([]domain.Auction, error) {
	var out []domain.Auction
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		got, err := r.ListAuctions(ctx, activeOnly)
		if err != nil {
			return err
		}
		out = got
		return nil
	})
	return out, err
}

// GetAuction returns a single auction by ID.
func (m *Market) GetAuction(ctx context.Context, id string) (*domain.Auction, error) {
	var a *domain.Auction
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		got, err := r.GetAuction(ctx, id)
		if err != nil {
			return err
		}
		a = got
		return nil
	})
	return a, err
}

// SettleDueAuctions closes every auction past its end time. For each: the winner
// settles (reservation → debit), the seller is credited, and the win accrues to
// the winner's daily-cap spend via the ledger (D12); with no bids, the item
// returns to available (R8). Returns how many were settled. Safe to call
// repeatedly (the worker does) — already-settled auctions are skipped.
func (m *Market) SettleDueAuctions(ctx context.Context) (int, error) {
	settled := 0
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		now := m.clock.Now()
		due, err := r.DueAuctions(ctx, now)
		if err != nil {
			return err
		}
		for i := range due {
			a := &due[i]
			if err := m.settleOne(ctx, r, a, now); err != nil {
				return err
			}
			settled++
		}
		return nil
	})
	return settled, err
}

// settleOne finalizes a single due auction within an existing transaction.
func (m *Market) settleOne(ctx context.Context, r ports.Repo, a *domain.Auction, now time.Time) error {
	winner, err := a.Settle()
	if err != nil {
		return err
	}

	item, err := r.GetItemForUpdate(ctx, a.ItemID)
	if err != nil {
		return err
	}

	if winner == nil {
		// No bids: the item returns to available (R8).
		item.Status = domain.ItemAvailable
		if err := r.UpdateItem(ctx, item); err != nil {
			return err
		}
		return r.UpdateAuction(ctx, a)
	}

	// Winner pays: the reservation becomes a real debit (R7), and the seller is
	// credited. Both movements are recorded (R9).
	winnerWallet, err := r.GetWalletForUpdate(ctx, winner.GuildID)
	if err != nil {
		return err
	}
	if err := winnerWallet.Settle(winner.Amount); err != nil {
		return err
	}
	if err := r.UpdateWallet(ctx, winnerWallet); err != nil {
		return err
	}

	sellerWallet, err := r.GetWalletForUpdate(ctx, a.SellerGuildID)
	if err != nil {
		return err
	}
	if err := sellerWallet.Credit(winner.Amount); err != nil {
		return err
	}
	if err := r.UpdateWallet(ctx, sellerWallet); err != nil {
		return err
	}

	if err := r.AppendLedger(ctx, domain.LedgerEntry{
		ID: m.ids.NewID(), GuildID: winner.GuildID, Type: domain.LedgerSettle,
		Amount: winner.Amount, ItemID: a.ItemID, Memo: "auction won", At: now,
	}); err != nil {
		return err
	}
	if err := r.AppendLedger(ctx, domain.LedgerEntry{
		ID: m.ids.NewID(), GuildID: a.SellerGuildID, Type: domain.LedgerCredit,
		Amount: winner.Amount, ItemID: a.ItemID, Memo: "auction sale", At: now,
	}); err != nil {
		return err
	}

	// The Legendary item is now sold.
	item.Status = domain.ItemSoldOut
	if err := r.UpdateItem(ctx, item); err != nil {
		return err
	}
	return r.UpdateAuction(ctx, a)
}

// Ledger returns a guild's full transaction history (R9, audit view).
func (m *Market) Ledger(ctx context.Context, guildID string) ([]domain.LedgerEntry, error) {
	var out []domain.LedgerEntry
	err := m.tx.WithinTx(ctx, func(r ports.Repo) error {
		got, err := r.ListLedger(ctx, guildID)
		if err != nil {
			return err
		}
		out = got
		return nil
	})
	return out, err
}
