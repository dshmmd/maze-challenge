# Phase 1 — Items & limit orders (Common/Rare)

**Date:** 2026-06-30
**Status:** core shipped in-memory (Postgres adapter = Phase 1b, next)

## What was done

- **Item domain** (`internal/domain/item.go`): `Rarity` (common/rare/legendary),
  `ItemStatus`, `NewItem` validation, and `SellTo` which applies the limit-order
  rules (own-item R10, stock, availability) and returns the cost. Legendary is
  forced to qty 1 / no price and rejected from limit-order buys.
- **Ledger** (`domain/ledger.go`): `LedgerEntry` + types for full traceability (R9).
- **Ports** (`internal/ports`): added `Repo` (item/wallet/ledger access with
  `…ForUpdate` locking reads), `TxManager.WithinTx` unit-of-work, `IDGenerator`.
- **Market service** (`internal/service/market.go`): `CreateItem`, `ListItems`,
  `GetItem`, `GetWallet`, `Purchase`. `Purchase` runs the whole read-modify-write
  in one `WithinTx`, locking the item + both wallets, so it is atomic and
  race-free (R1, R13, R16-partial, R17).
- **In-memory store** (`internal/adapters/memstore`): `TxManager`+`Repo` with a
  store-wide mutex per unit of work and staged writes committed only on success.
- **HTTP** (`internal/adapters/httpapi`): `POST/GET /items`, `GET /items/{id}`,
  `POST /items/{id}/purchase`, `GET /guilds/{id}/wallet`; domain-error→status
  mapping; `X-Guild-Id` header for identity (D8).
- **Wiring** (`cmd/server`): memstore + idgen + real clock + service + HTTP, with
  two demo guilds seeded.
- **Tests** (22 total, pass under `-race`): domain `SellTo`/`Wallet`, service
  purchase happy/insufficient-funds/legendary, **concurrent 50-buyer no-oversell**
  test, and HTTP flow + auth + conflict tests.

## Why / decisions

- D8 (X-Guild-Id), D9 (price+quantity listing), D10 (`/purchase` endpoint),
  D11 (UoW `TxManager`, in-memory first then Postgres). See `ROADMAP.md`.

## Gotchas / notes for the next agent

- **Persistence is in-memory.** The guarantees are proven against `memstore`'s
  coarse lock; the real concurrency story needs the **Postgres adapter with
  `SELECT … FOR UPDATE`** — that is Phase 1b and should land before Phase 2.
  The service/HTTP layers won't change: just add a `postgres` package
  implementing `ports.Repo`/`TxManager` and swap the wiring in `cmd/`.
- `memstore.Store.ListItemsForTest` / `SeedWallet` are inspection/seed helpers,
  not part of the `Repo` port.
- Demo guilds (`ember-wardens`, `frost-covenant`, 10k gold each) are seeded only
  in the in-memory build; with Postgres this becomes a seed migration.
- Open questions still to resolve before their phases: Q2 (daily-cap), Q3
  (oracle role), Q4 (idempotency).
