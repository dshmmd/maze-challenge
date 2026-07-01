package domain

import (
	"testing"
	"time"
)

func newAuction() *Auction {
	start := mockTime()
	return &Auction{
		ID: "a1", ItemID: "i1", SellerGuildID: "seller",
		StartPrice: 100, EndsAt: start.Add(24 * time.Hour),
		Extension: 5 * time.Minute, Status: AuctionActive, CreatedAt: start,
	}
}

func TestMinNextBid(t *testing.T) {
	a := newAuction()
	if got := a.MinNextBid(); got != 100 {
		t.Fatalf("first MinNextBid = %d, want 100 (start price)", got)
	}
	if _, err := a.PlaceBid("b1", "g1", 100, mockTime()); err != nil {
		t.Fatalf("PlaceBid 100: %v", err)
	}
	// 5% of 100 = 5 → next must be ≥ 105.
	if got := a.MinNextBid(); got != 105 {
		t.Fatalf("MinNextBid after 100 = %d, want 105", got)
	}
}

func TestPlaceBidRejectsTooLow(t *testing.T) {
	a := newAuction()
	a.PlaceBid("b1", "g1", 100, mockTime())
	if _, err := a.PlaceBid("b2", "g2", 104, mockTime()); err != ErrBidTooLow {
		t.Fatalf("bid 104 err = %v, want ErrBidTooLow", err)
	}
	if _, err := a.PlaceBid("b3", "g2", 105, mockTime()); err != nil {
		t.Fatalf("bid 105: %v", err)
	}
}

func TestPlaceBidRejectsOwnItem(t *testing.T) {
	a := newAuction()
	if _, err := a.PlaceBid("b1", "seller", 100, mockTime()); err != ErrCannotBidOwnItem {
		t.Fatalf("seller bid err = %v, want ErrCannotBidOwnItem", err)
	}
}

func TestPlaceBidUnseatsPreviousLeader(t *testing.T) {
	a := newAuction()
	a.PlaceBid("b1", "g1", 100, mockTime())
	unseated, err := a.PlaceBid("b2", "g2", 105, mockTime())
	if err != nil {
		t.Fatalf("PlaceBid: %v", err)
	}
	if unseated == nil || unseated.GuildID != "g1" {
		t.Fatalf("unseated = %+v, want g1's bid", unseated)
	}
	if h := a.HighestBid(); h.GuildID != "g2" {
		t.Fatalf("highest = %s, want g2", h.GuildID)
	}
}

func TestAntiSnipeExtension(t *testing.T) {
	a := newAuction()
	// A bid 2 minutes before close must push the end out to now+5m.
	now := a.EndsAt.Add(-2 * time.Minute)
	if _, err := a.PlaceBid("b1", "g1", 100, now); err != nil {
		t.Fatalf("PlaceBid: %v", err)
	}
	want := now.Add(5 * time.Minute)
	if !a.EndsAt.Equal(want) {
		t.Fatalf("EndsAt = %v, want %v (extended)", a.EndsAt, want)
	}
}

func TestNoExtensionWhenAmpleTime(t *testing.T) {
	a := newAuction()
	orig := a.EndsAt
	if _, err := a.PlaceBid("b1", "g1", 100, mockTime()); err != nil {
		t.Fatalf("PlaceBid: %v", err)
	}
	if !a.EndsAt.Equal(orig) {
		t.Fatalf("EndsAt changed to %v with ample time left", a.EndsAt)
	}
}

func TestCancelLeaderRejected(t *testing.T) {
	a := newAuction()
	a.PlaceBid("b1", "g1", 100, mockTime())
	if _, err := a.CancelBid("b1", "g1"); err != ErrCannotCancelLeader {
		t.Fatalf("cancel leader err = %v, want ErrCannotCancelLeader", err)
	}
}

func TestCancelNonLeader(t *testing.T) {
	a := newAuction()
	a.PlaceBid("b1", "g1", 100, mockTime())
	a.PlaceBid("b2", "g2", 200, mockTime())
	// g3 (not the owner) cannot cancel g1's non-leading bid.
	if _, err := a.CancelBid("b1", "g3"); err != ErrBidNotFound {
		t.Fatalf("non-owner cancel err = %v, want ErrBidNotFound", err)
	}
	// g1 is no longer leader and owns b1 → may cancel.
	if _, err := a.CancelBid("b1", "g1"); err != nil {
		t.Fatalf("cancel non-leader: %v", err)
	}
	// After cancelling, g1's bid is inactive and g2 remains the leader.
	if h := a.HighestBid(); h == nil || h.GuildID != "g2" {
		t.Fatalf("leader after cancel = %+v, want g2", h)
	}
}

func TestSettleReturnsWinner(t *testing.T) {
	a := newAuction()
	a.PlaceBid("b1", "g1", 100, mockTime())
	a.PlaceBid("b2", "g2", 200, mockTime())
	winner, err := a.Settle()
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if winner == nil || winner.GuildID != "g2" {
		t.Fatalf("winner = %+v, want g2", winner)
	}
	if a.Status != AuctionSettled {
		t.Fatalf("status = %s, want settled", a.Status)
	}
	// Second settle is rejected (no double-settle).
	if _, err := a.Settle(); err != ErrAuctionClosed {
		t.Fatalf("re-settle err = %v, want ErrAuctionClosed", err)
	}
}

func TestSettleNoBids(t *testing.T) {
	a := newAuction()
	winner, err := a.Settle()
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if winner != nil {
		t.Fatalf("winner = %+v, want nil (no bids)", winner)
	}
}
