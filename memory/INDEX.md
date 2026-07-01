# Memory Index — Dragon Market

One line per shipped milestone. Newest first. Read the linked file for the
"why" and the gotchas before building on that area.

- [0002-phase2-5-auctions-resilience-ui.md](0002-phase2-5-auctions-resilience-ui.md)
  — Phases 1b–5: Postgres adapter (pgx, `FOR UPDATE`, partial unique index),
  Legendary auctions (reserve/release/settle, 5%, anti-snipe, worker), advisory
  oracle, daily cap + `Idempotency-Key` + ledger audit, reviewer console at `/`.
  Verified end-to-end on real Postgres. All R1–R17 done; run model = Postgres-only.
- [0001-phase1-limit-orders.md](0001-phase1-limit-orders.md) — Phase 1: items +
  rarity, limit-order purchase (atomic buy, ledger, single-fulfilment), market
  service over `TxManager`/`Repo` ports, in-memory store, HTTP endpoints, 50-buyer
  no-oversell race test. Postgres adapter is the next slice (Phase 1b).
- [0000-phase0-foundation.md](0000-phase0-foundation.md) — Phase 0 scaffold:
  hexagonal layout, pure `Wallet` domain (reserve/release/settle) with tests,
  ports + mock oracle + clock, chi `/healthz`, `make check` gate, full doc
  harness (README as company channel, ROADMAP, ADR-0001).
