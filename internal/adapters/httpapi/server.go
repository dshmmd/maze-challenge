// Package httpapi is the HTTP transport adapter: it wires routes to handlers
// and translates between HTTP and the domain. Business rules live in the core,
// not here.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/dshmmd/maze-challenge/internal/adapters/webui"
	"github.com/dshmmd/maze-challenge/internal/domain"
	"github.com/dshmmd/maze-challenge/internal/service"
)

// Server holds the HTTP router and its dependencies.
type Server struct {
	router chi.Router
	market *service.Market
	idem   *idempotencyStore
}

// New constructs the Server and registers routes. demoGuilds, if non-empty,
// populates the web console's guild switcher (served at "/").
func New(market *service.Market, demoGuilds ...string) *Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	s := &Server{router: r, market: market, idem: newIdempotencyStore()}
	s.routes()

	if ui, err := webui.New(demoGuilds); err == nil {
		s.router.Get("/", ui.ServeHTTP)
	}
	return s
}

// Handler exposes the router for the HTTP server (and tests).
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() {
	s.router.Get("/healthz", s.handleHealth)

	s.router.Route("/items", func(r chi.Router) {
		r.Get("/", s.handleListItems)
		r.Get("/{id}", s.handleGetItem)
		// Mutating routes get idempotent-replay protection (R16, D14).
		r.Group(func(r chi.Router) {
			r.Use(s.idem.middleware)
			r.Post("/", s.handleCreateItem)
			r.Post("/{id}/purchase", s.handlePurchase)
			r.Post("/{id}/bid", s.handlePlaceBid)
			r.Delete("/{id}/bid/{bid_id}", s.handleCancelBid)
		})
	})

	s.router.Route("/auctions", func(r chi.Router) {
		r.Get("/", s.handleListAuctions)
		r.Get("/{id}", s.handleGetAuction)
	})

	s.router.Get("/guilds/{id}/wallet", s.handleGetWallet)
	s.router.Get("/guilds/{id}/ledger", s.handleGetLedger)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeJSON is the shared helper for JSON responses.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

// writeError maps a domain error to an HTTP status and a JSON error body. This
// is the single place transport knows about domain error semantics.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrItemNotFound),
		errors.Is(err, domain.ErrWalletNotFound),
		errors.Is(err, domain.ErrGuildNotFound),
		errors.Is(err, domain.ErrAuctionNotFound),
		errors.Is(err, domain.ErrBidNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInsufficientFunds),
		errors.Is(err, domain.ErrInsufficientStock),
		errors.Is(err, domain.ErrItemNotAvailable),
		errors.Is(err, domain.ErrCannotBuyOwnItem),
		errors.Is(err, domain.ErrLegendaryNotLimit),
		errors.Is(err, domain.ErrAuctionClosed),
		errors.Is(err, domain.ErrAuctionExists),
		errors.Is(err, domain.ErrNotLegendary),
		errors.Is(err, domain.ErrBidTooLow),
		errors.Is(err, domain.ErrCannotBidOwnItem),
		errors.Is(err, domain.ErrCannotCancelLeader),
		errors.Is(err, domain.ErrDailyCapExceeded):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrNotItemOwner):
		status = http.StatusForbidden
	case errors.Is(err, domain.ErrInvalidRarity),
		errors.Is(err, domain.ErrInvalidQuantity),
		errors.Is(err, domain.ErrNonPositiveAmount):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// guildID extracts the acting guild from the X-Guild-Id header (D8). Empty is
// rejected by the caller.
func guildID(r *http.Request) string { return r.Header.Get("X-Guild-Id") }
