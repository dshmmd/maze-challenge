package domain

import "time"

// minIncrementPercent is the minimum a new bid must exceed the current highest
// by (R11): at least 5%.
const minIncrementPercent = 5

// AuctionStatus is the lifecycle state of an auction.
type AuctionStatus string

const (
	// AuctionActive — open for bids and before its end time.
	AuctionActive AuctionStatus = "active"
	// AuctionSettled — closed; winner (if any) has been finalized.
	AuctionSettled AuctionStatus = "settled"
)

// Bid is a single offer in an auction. Only the current highest *active* bid
// holds a wallet reservation; lower bids were released the moment they were
// outbid (R7).
type Bid struct {
	ID       string
	GuildID  string
	Amount   Gold
	Active   bool
	PlacedAt time.Time
}

// Auction sells a single Legendary item to the highest bidder over a time
// window. At most one active auction exists per item (R3), enforced by the
// store; this type owns the bidding rules.
type Auction struct {
	ID            string
	ItemID        string
	SellerGuildID string
	StartPrice    Gold // minimum acceptable first bid
	EndsAt        time.Time
	Extension     time.Duration // anti-snipe extension (e.g. 5m)
	Status        AuctionStatus
	Bids          []Bid
	CreatedAt     time.Time
}

// HighestBid returns the current winning bid, or nil if there are none.
func (a *Auction) HighestBid() *Bid {
	var best *Bid
	for i := range a.Bids {
		b := &a.Bids[i]
		if !b.Active {
			continue
		}
		if best == nil || b.Amount > best.Amount {
			best = b
		}
	}
	return best
}

// MinNextBid is the smallest amount a new bid may be: the start price when there
// are no bids yet, otherwise the highest plus a ceil(5%) increment (R11).
func (a *Auction) MinNextBid() Gold {
	high := a.HighestBid()
	if high == nil {
		return a.StartPrice
	}
	inc := (high.Amount*minIncrementPercent + 99) / 100 // ceil of 5%
	inc = max(inc, 1)
	return high.Amount + inc
}

// PlaceBid validates and appends a new bid. It returns the bid that was just
// unseated (the previous leader, whose reservation the caller must release) or
// nil. A bid placed within the final Extension window pushes EndsAt out by
// Extension (anti-snipe, R6).
//
// It enforces: auction open and not past end (R5), not the seller's own item
// (R10), and the 5% minimum increment (R11). Fund movement is the caller's job.
func (a *Auction) PlaceBid(bidID, guild string, amount Gold, now time.Time) (unseated *Bid, err error) {
	if a.Status != AuctionActive || !now.Before(a.EndsAt) {
		return nil, ErrAuctionClosed
	}
	if guild == a.SellerGuildID {
		return nil, ErrCannotBidOwnItem
	}
	if amount < a.MinNextBid() {
		return nil, ErrBidTooLow
	}

	prev := a.HighestBid() // becomes the unseated leader (if any)
	a.Bids = append(a.Bids, Bid{
		ID: bidID, GuildID: guild, Amount: amount, Active: true, PlacedAt: now,
	})

	// Anti-snipe: guarantee at least Extension remains after a late bid.
	if a.Extension > 0 && a.EndsAt.Sub(now) < a.Extension {
		a.EndsAt = now.Add(a.Extension)
	}
	return prev, nil
}

// CancelBid withdraws a guild's bid. Only the bid's owner may cancel it, and the
// current highest bidder may not (R12). The cancelled bid is returned; since a
// non-leading bid holds no reservation (it was released when outbid), no funds
// move on cancel.
func (a *Auction) CancelBid(bidID, guild string) (*Bid, error) {
	if a.Status != AuctionActive {
		return nil, ErrAuctionClosed
	}
	high := a.HighestBid()
	if high != nil && high.ID == bidID {
		return nil, ErrCannotCancelLeader
	}
	for i := range a.Bids {
		b := &a.Bids[i]
		if b.ID == bidID && b.Active {
			if b.GuildID != guild {
				return nil, ErrBidNotFound
			}
			b.Active = false
			return b, nil
		}
	}
	return nil, ErrBidNotFound
}

// Due reports whether the auction has reached its end time and should settle.
func (a *Auction) Due(now time.Time) bool {
	return a.Status == AuctionActive && !now.Before(a.EndsAt)
}

// Settle closes the auction and returns the winning bid (nil if there were no
// bids). The caller finalizes funds (winner settles, seller is credited) or, on
// nil, returns the item to available. Settling is idempotent-friendly: a
// non-active auction returns ErrAuctionClosed.
func (a *Auction) Settle() (*Bid, error) {
	if a.Status != AuctionActive {
		return nil, ErrAuctionClosed
	}
	a.Status = AuctionSettled
	return a.HighestBid(), nil
}
