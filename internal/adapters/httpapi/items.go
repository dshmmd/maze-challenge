package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dshmmd/maze-challenge/internal/domain"
	"github.com/dshmmd/maze-challenge/internal/service"
)

// itemView is the JSON representation of an item returned to clients.
type itemView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Rarity      string `json:"rarity"`
	Seller      string `json:"seller_guild_id"`
	Price       int64  `json:"price"`
	Quantity    int    `json:"quantity"`
	Status      string `json:"status"`
	OraclePrice *int64 `json:"oracle_price,omitempty"` // advisory, display-only (D13)
}

func toItemView(i domain.Item) itemView {
	return itemView{
		ID: i.ID, Name: i.Name, Rarity: string(i.Rarity), Seller: i.SellerGuildID,
		Price: int64(i.Price), Quantity: i.Quantity, Status: string(i.Status),
	}
}

// withOraclePrice attaches the advisory oracle price to a view, if available.
func (s *Server) withOraclePrice(r *http.Request, v itemView) itemView {
	if p, ok := s.market.OraclePrice(r.Context(), v.ID); ok {
		pp := int64(p)
		v.OraclePrice = &pp
	}
	return v
}

type createItemRequest struct {
	Name     string `json:"name"`
	Rarity   string `json:"rarity"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	seller := guildID(r)
	if seller == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing X-Guild-Id header"})
		return
	}
	var req createItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	item, err := s.market.CreateItem(r.Context(), service.CreateItemInput{
		Name:     req.Name,
		Rarity:   domain.Rarity(req.Rarity),
		Seller:   seller,
		Price:    domain.Gold(req.Price),
		Quantity: req.Quantity,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toItemView(*item))
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	items, err := s.market.ListItems(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	views := make([]itemView, 0, len(items))
	for _, it := range items {
		views = append(views, s.withOraclePrice(r, toItemView(it)))
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleGetItem(w http.ResponseWriter, r *http.Request) {
	item, err := s.market.GetItem(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.withOraclePrice(r, toItemView(*item)))
}

type purchaseRequest struct {
	Quantity int `json:"quantity"`
}

func (s *Server) handlePurchase(w http.ResponseWriter, r *http.Request) {
	buyer := guildID(r)
	if buyer == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing X-Guild-Id header"})
		return
	}
	// Quantity defaults to 1 when the body is empty or omits it.
	req := purchaseRequest{Quantity: 1}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Quantity == 0 {
		req.Quantity = 1
	}

	res, err := s.market.Purchase(r.Context(), chi.URLParam(r, "id"), buyer, req.Quantity)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"item": toItemView(*res.Item),
		"cost": int64(res.Cost),
	})
}

type walletView struct {
	GuildID   string `json:"guild_id"`
	Total     int64  `json:"total"`
	Reserved  int64  `json:"reserved"`
	Available int64  `json:"available"`
}

func (s *Server) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	wallet, err := s.market.GetWallet(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, walletView{
		GuildID:   wallet.GuildID,
		Total:     int64(wallet.Total),
		Reserved:  int64(wallet.Reserved),
		Available: int64(wallet.Available()),
	})
}
