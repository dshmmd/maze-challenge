# ADR-0001: Foundation — stack, architecture, and money model

- **Status:** Accepted
- **Date:** 2026-06-27
- **Decider:** Project author (human)
- **Context:** Dragon Market backend challenge (Go). The challenge's three
  guarantees are about correctness under concurrency, defensibility, and
  auditability — not feature breadth.

This ADR records the foundational choices. The challenge explicitly asks for an
architecture-decisions document answering: *why this structure? where did you cut
corners (off-trade)? what would you add with more time?*

## Decision

### D1 — PostgreSQL as the datastore
The hardest requirements are atomicity ("no duplicate sale") and isolation ("no
over-commitment under concurrent bids"). These are database problems. Postgres
gives ACID transactions and pessimistic row locking (`SELECT … FOR UPDATE`),
letting us enforce invariants — single-fulfilment of a listing, single active
auction per Legendary item, available-balance check — at the storage layer where
they cannot be raced. Alternatives considered: SQLite (single-writer
serialization is simpler but undersells a concurrency challenge) and in-memory +
mutexes (full control but persistence and audit must be hand-rolled and are
harder to defend).

### D2 — Hexagonal architecture (ports & adapters)
The domain layer (`internal/domain`) is pure: value types and business rules with
no I/O, unit-testable in isolation (see `wallet_test.go`). Everything external —
repositories, the Price Oracle, the clock — is an interface in `internal/ports`,
implemented by adapters. This directly satisfies the challenge's "define an
interface for external services and mock it" requirement and keeps the rules
greppable and testable.

### D3 — chi router on net/http
A light, idiomatic router. Handlers are a thin transport adapter that translate
HTTP ↔ domain and map domain sentinel errors to status codes. No heavy framework
obscuring the core logic.

### D4 — Injectable Clock + background settlement worker
Auctions have a 24h window with a 5-minute anti-snipe extension. Time-dependent
rules are untestable against the wall clock. A `Clock` interface (real in prod, a
fake that can be advanced in tests) makes extend/settle logic deterministic. A
background sweeper settles expired auctions in production.

### D5 — Integer gold (`int64`), never floating point
Money must be exact. The reserve → release/settle cycle must net to exactly zero;
floats cannot guarantee that. Gold is whole-unit `int64`.

### D6 — Reserve-not-deduct for bids
Per spec, a bid reserves funds rather than deducting them. The wallet models this
as `available = total − reserved`. Losing or cancelling releases immediately;
winning settles (release + debit) atomically. A guild's spendable balance is
therefore always honest, and over-commitment is impossible by construction.

## Off-trade (corners cut, intentionally — for now)

- **Auth is minimal.** Acting guild identity is expected to come from a header,
  not real authentication (Q1). Out of scope for a core challenge.
- **Phase 0 tests are pure unit tests.** No DB is required for `make check`;
  integration/concurrency tests against Postgres come in later phases.
- Several semantics are still open questions (daily-cap counting, oracle's
  gating role, idempotency mechanism, Rare stock model) — tracked as Q1–Q5 in
  `ROADMAP.md`, to be decided by the author and recorded in the README.

## What I'd add with more time

- Postgres-backed concurrency/race tests proving R1, R16, R17 under load.
- A full transaction/audit ledger and structured observability.
- Idempotency keys on mutating endpoints.
- A richer oracle-resilience strategy (circuit breaker, staleness windows).

## Consequences

- Running the full stack needs Docker (Postgres). The domain and its tests run
  with zero infrastructure.
- Strong invariant enforcement at the cost of a DB dependency — an acceptable and
  defensible trade for a money-handling system.
