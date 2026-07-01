package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dshmmd/maze-challenge/internal/adapters/clock"
	"github.com/dshmmd/maze-challenge/internal/adapters/idgen"
	"github.com/dshmmd/maze-challenge/internal/adapters/memstore"
	"github.com/dshmmd/maze-challenge/internal/adapters/oracle"
	"github.com/dshmmd/maze-challenge/internal/service"
)

func newTestServer(t *testing.T) (*Server, *memstore.Store) {
	t.Helper()
	store := memstore.New()
	market := service.NewMarket(store, idgen.Hex{}, clock.Real{}, oracle.NewMock(nil), service.DefaultConfig())
	return New(market), store
}

func do(t *testing.T, srv *Server, method, path, guild, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rdr)
	if guild != "" {
		req.Header.Set("X-Guild-Id", guild)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := do(t, srv, http.MethodGet, "/healthz", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}

func TestCreateRequiresGuildHeader(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := do(t, srv, http.MethodPost, "/items", "", `{"name":"x","rarity":"common","price":10,"quantity":1}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /items without guild = %d, want 401", rec.Code)
	}
}

func TestListPurchaseFlow(t *testing.T) {
	srv, store := newTestServer(t)
	store.SeedWallet("seller", 0)
	store.SeedWallet("buyer", 1_000)

	// Seller lists a Rare item.
	rec := do(t, srv, http.MethodPost, "/items", "seller", `{"name":"Healing Potion","rarity":"rare","price":100,"quantity":3}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create item = %d (%s), want 201", rec.Code, rec.Body.String())
	}

	// Find the item id via the list endpoint.
	items, err := store.ListItemsForTest()
	if err != nil || len(items) != 1 {
		t.Fatalf("expected 1 item, got %d err=%v", len(items), err)
	}
	id := items[0].ID

	// Buyer purchases 2 units → cost 200.
	rec = do(t, srv, http.MethodPost, "/items/"+id+"/purchase", "buyer", `{"quantity":2}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("purchase = %d (%s), want 200", rec.Code, rec.Body.String())
	}

	// Buyer wallet should now read 800 available.
	rec = do(t, srv, http.MethodGet, "/guilds/buyer/wallet", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"available":800`) {
		t.Fatalf("wallet = %d (%s), want available 800", rec.Code, rec.Body.String())
	}
}

func TestPurchaseOwnItemConflict(t *testing.T) {
	srv, store := newTestServer(t)
	store.SeedWallet("seller", 1_000)
	do(t, srv, http.MethodPost, "/items", "seller", `{"name":"Sword","rarity":"common","price":10,"quantity":1}`)
	items, _ := store.ListItemsForTest()
	rec := do(t, srv, http.MethodPost, "/items/"+items[0].ID+"/purchase", "seller", `{"quantity":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("buy own item = %d, want 409", rec.Code)
	}
}
