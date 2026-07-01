# Phases 1b–5 — Postgres, auctions, resilience, cap, idempotency, UI

**Date:** 2026-06-30
**Status:** shipped (green `make check`; verified end-to-end against real Postgres)

## What was done

- **Phase 1b — Postgres (D1):** `internal/adapters/postgres` implements
  `TxManager`/`Repo` over pgx with `SELECT … FOR UPDATE` locking. Schema is
  embedded (`schema.sql`) and applied on boot; a **partial unique index**
  `(item_id) WHERE status='active'` enforces one active auction per item (R3) at
  the DB. `memstore` is now the **test double** only — run model is Postgres-only.
- **Phase 2 — Auctions/bids:** `domain/auction.go` (Auction, Bid, 5% increment,
  anti-snipe extend, cancel rules, settle) + service `BidOnItem` / `CancelBidOnItem`
  / `SettleDueAuctions`. Reserve→release(outbid)→settle through one locked tx.
  Listing a **Legendary auto-opens its auction** (price = starting bid). Background
  `RunSettlementWorker` on the injectable clock.
- **Phase 3 — Oracle (D13):** `Market.OraclePrice` — 500ms timeout, reject
  non-positive, cache last-good per item; attached to item views as advisory only.
- **Phase 4 — Cap + idempotency + audit:** daily cap (D12) from ledger sum of
  debit+settle since UTC midnight, checked in Purchase and at bid time; in-process
  `Idempotency-Key` middleware (D14); `GET /guilds/{id}/ledger`.
- **Phase 5 — Delivery:** reviewer **console at `/`** (`internal/adapters/webui`,
  Go template + fetch); docker-compose (app + Postgres) with short
  `AUCTION_WINDOW` for observable settlement; ADR-0002; demo seed (guilds
  ironband/stormforge/shadowveil + starter items).

## Verified (real Postgres, docker)

Limit buy + stock decrement; **idempotent replay charged once**; bid reserve;
5%-too-low → 409; outbid → previous leader released; own-item bid → 409;
window-expiry settlement → winner debited, seller credited, item `sold_out`,
ledger shows `reserve`→`settle`. UI renders with guild switcher.

## Black-box QA harness

`scripts/qa.mjs` (`make qa`) drives a *running* server through 41 assertions
across 11 scenarios — limit orders, rejections, daily cap (on buys *and* bids),
idempotent replay, a 12-way concurrency race (no oversell), a self-contained
auction (bid/reserve hand-off/cancel/settlement), no-bid return, and a
gold-conservation invariant. Run it against a **fresh DB** with a **short
`AUCTION_WINDOW`** (e.g. 5s) — it polls for settlement. Verified 41/41 green, and
the UI was screenshot-checked end to end.

Design gotcha it caught: the daily cap is checked at **bid time**, so a capped
guild (e.g. shadowveil near its limit) can be legitimately rejected mid-auction —
keep that in mind when writing scenarios (use uncapped bidders + self-contained
auctions so background settlement doesn't perturb later balance assertions).

## Gotchas / notes for the next agent

- **Run model is Postgres-only.** `make run` needs a reachable DB; use `make up`.
  `make check` stays DB-free (unit tests use `memstore`).
- **No Postgres race test in CI yet** — concurrency was verified manually + the
  in-memory `-race` limit-order test. Adding parallel-bidder DB tests is the top
  follow-up (ADR-0002).
- **Daily-cap edge:** two simultaneous winning bids could each pass the pre-settle
  check; documented in D12/ADR-0002, not closed.
- **Idempotency store is in-process** (not durable across restarts) — D14.
- `UpdateAuction` re-materializes bids (delete+insert) within the tx — fine at
  this scale.
- Auction starting bid comes from the item's `price` field at creation
  (Item.Price stays 0 for Legendary; the auction holds `StartPrice`).
