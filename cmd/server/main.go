// Command server is the Dragon Market entrypoint: load config, connect Postgres,
// wire adapters, seed demo data, and run the HTTP server + settlement worker.
// Wiring lives here and nowhere else (hexagonal).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dshmmd/maze-challenge/internal/adapters/clock"
	"github.com/dshmmd/maze-challenge/internal/adapters/httpapi"
	"github.com/dshmmd/maze-challenge/internal/adapters/idgen"
	"github.com/dshmmd/maze-challenge/internal/adapters/oracle"
	"github.com/dshmmd/maze-challenge/internal/adapters/postgres"
	"github.com/dshmmd/maze-challenge/internal/domain"
	"github.com/dshmmd/maze-challenge/internal/service"
)

func main() {
	addr := getenv("HTTP_ADDR", ":8080")
	dsn := getenv("DATABASE_URL", "postgres://dragon:dragon@localhost:5432/dragon_market?sslmode=disable")

	cfg := service.Config{
		// Demo-friendly defaults so the anti-snipe rule is easy to see: a bid in
		// the final 5 minutes of the 6-minute window pushes the close out. Both
		// are configurable (the spec suggests 24h; override via env for that).
		AuctionWindow:    getdur("AUCTION_WINDOW", 6*time.Minute),
		AuctionExtension: getdur("AUCTION_EXTENSION", 5*time.Minute),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Connect Postgres (D1). Retry briefly so we tolerate the DB still booting
	// under docker-compose.
	store, err := connectWithRetry(ctx, dsn, 30, time.Second)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer store.Close()

	priceOracle := oracle.NewMock(nil)
	market := service.NewMarket(store, idgen.Hex{}, clock.Real{}, priceOracle, cfg)

	guilds := seed(ctx, store, market, priceOracle)

	// Background settlement of expired auctions (D4).
	go market.RunSettlementWorker(ctx, 5*time.Second)

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(market, guilds...).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("dragon-market listening on %s (console at http://localhost%s/)", addr, addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Print("dragon-market stopped")
}

func connectWithRetry(ctx context.Context, dsn string, attempts int, wait time.Duration) (*postgres.Store, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		store, err := postgres.Connect(ctx, dsn)
		if err == nil {
			return store, nil
		}
		lastErr = err
		log.Printf("waiting for database (%d/%d): %v", i+1, attempts, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, lastErr
}

// seed inserts demo guilds and a starter set of listings on first boot, and
// returns the guild IDs for the console switcher. Guilds upsert (idempotent);
// items are only seeded when none exist, so restarts don't duplicate them.
func seed(ctx context.Context, store *postgres.Store, market *service.Market, oc *oracle.Mock) []string {
	type g struct {
		id   string
		gold domain.Gold
		cap  domain.Gold
	}
	demo := []g{
		{"ironband", 10_000, 0},      // unlimited daily cap
		{"stormforge", 10_000, 0},    //
		{"shadowveil", 5_000, 2_000}, // capped at 2,000/day
	}
	ids := make([]string, 0, len(demo))
	for _, d := range demo {
		if err := store.SeedGuild(ctx, d.id, d.gold, d.cap); err != nil {
			log.Printf("seed guild %s: %v", d.id, err)
		}
		ids = append(ids, d.id)
	}

	existing, err := market.ListItems(ctx)
	if err == nil && len(existing) == 0 {
		starters := []service.CreateItemInput{
			{Name: "Health Potion", Rarity: domain.Common, Seller: "ironband", Price: 50, Quantity: 20},
			{Name: "Mithril Shield", Rarity: domain.Rare, Seller: "stormforge", Price: 800, Quantity: 3},
			{Name: "Soul Reaver", Rarity: domain.Legendary, Seller: "shadowveil", Price: 1_000},
		}
		for _, in := range starters {
			item, err := market.CreateItem(ctx, in)
			if err != nil {
				log.Printf("seed item %s: %v", in.Name, err)
				continue
			}
			// Advisory oracle price near the listed price (D13).
			oc.SetPrice(item.ID, in.Price+25)
		}
	} else {
		// Re-attach advisory prices for already-persisted items.
		for _, it := range existing {
			base := it.Price
			if base == 0 {
				base = 1_000
			}
			oc.SetPrice(it.ID, base+25)
		}
	}
	return ids
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getdur(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Printf("invalid duration %s=%q, using %s", key, v, fallback)
	}
	return fallback
}
