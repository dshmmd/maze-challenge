package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dshmmd/maze-challenge/internal/domain"
)

// bidView is the JSON representation of a single bid.
type bidView struct {
	ID      string `json:"id"`
	GuildID string `json:"guild_id"`
	Amount  int64  `json:"amount"`
	Active  bool   `json:"active"`
}

// auctionView is the JSON representation of an auction.
type auctionView struct {
	ID         string    `json:"id"`
	ItemID     string    `json:"item_id"`
	Seller     string    `json:"seller_guild_id"`
	StartPrice int64     `json:"start_price"`
	MinNextBid int64     `json:"min_next_bid"`
	EndsAt     string    `json:"ends_at"`
	Status     string    `json:"status"`
	HighestBid *bidView  `json:"highest_bid"`
	Bids       []bidView `json:"bids"`
}

func toAuctionView(a domain.Auction) auctionView {
	bids := make([]bidView, 0, len(a.Bids))
	for _, b := range a.Bids {
		bids = append(bids, bidView{ID: b.ID, GuildID: b.GuildID, Amount: int64(b.Amount), Active: b.Active})
	}
	v := auctionView{
		ID: a.ID, ItemID: a.ItemID, Seller: a.SellerGuildID,
		StartPrice: int64(a.StartPrice), MinNextBid: int64(a.MinNextBid()),
		EndsAt: a.EndsAt.UTC().Format("2006-01-02T15:04:05Z"), Status: string(a.Status),
		Bids: bids,
	}
	if h := a.HighestBid(); h != nil {
		v.HighestBid = &bidView{ID: h.ID, GuildID: h.GuildID, Amount: int64(h.Amount), Active: h.Active}
	}
	return v
}

func (s *Server) handleListAuctions(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("all") != "true"
	auctions, err := s.market.ListAuctions(r.Context(), activeOnly)
	if err != nil {
		writeError(w, err)
		return
	}
	views := make([]auctionView, 0, len(auctions))
	for _, a := range auctions {
		views = append(views, toAuctionView(a))
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleGetAuction(w http.ResponseWriter, r *http.Request) {
	a, err := s.market.GetAuction(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuctionView(*a))
}

type bidRequest struct {
	Amount int64 `json:"amount"`
}

func (s *Server) handlePlaceBid(w http.ResponseWriter, r *http.Request) {
	bidder := guildID(r)
	if bidder == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing X-Guild-Id header"})
		return
	}
	var req bidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	a, err := s.market.BidOnItem(r.Context(), chi.URLParam(r, "id"), bidder, domain.Gold(req.Amount))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAuctionView(*a))
}

func (s *Server) handleCancelBid(w http.ResponseWriter, r *http.Request) {
	guild := guildID(r)
	if guild == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing X-Guild-Id header"})
		return
	}
	a, err := s.market.CancelBidOnItem(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "bid_id"), guild)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuctionView(*a))
}

type ledgerView struct {
	ID      string `json:"id"`
	GuildID string `json:"guild_id"`
	Type    string `json:"type"`
	Amount  int64  `json:"amount"`
	ItemID  string `json:"item_id,omitempty"`
	Memo    string `json:"memo,omitempty"`
	At      string `json:"at"`
}

func (s *Server) handleGetLedger(w http.ResponseWriter, r *http.Request) {
	entries, err := s.market.Ledger(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	views := make([]ledgerView, 0, len(entries))
	for _, e := range entries {
		views = append(views, ledgerView{
			ID: e.ID, GuildID: e.GuildID, Type: string(e.Type), Amount: int64(e.Amount),
			ItemID: e.ItemID, Memo: e.Memo, At: e.At.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, views)
}
