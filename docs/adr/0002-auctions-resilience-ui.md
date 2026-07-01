# ADR-0002: Auctions, resilience, daily cap, idempotency, and the review console

- **Status:** Accepted
- **Date:** 2026-06-30
- **Decider:** Project author (human), with agent implementation
- **Context:** Builds on ADR-0001. Covers the decisions taken while implementing
  the Legendary auction lifecycle, the flaky-oracle handling, the daily purchase
  cap, idempotency, and a reviewer-facing UI.

## Decisions

### D8 — Guild identity via `X-Guild-Id` header (no real auth)
The acting guild is taken from a request header and trusted. Real
authentication is out of scope for a market-correctness challenge; this keeps
the focus on the guarantees. **Off-trade**, documented below.

### D9 — Limit-order listings carry a price + quantity
A Common/Rare listing has a fixed unit price (set at listing time) and a
remaining quantity decremented atomically per purchase. Legendary is always a
single unit and is never sold by limit order.

### D10 — Listing a Legendary opens its auction immediately
`POST /items` for a Legendary item also opens its (single) auction, using the
submitted price as the starting bid and the configured window. This matches the
spec's API surface (no separate "start auction" endpoint) and the rule that a
Legendary item has at most one active auction (enforced in the DB by a partial
unique index).

### D11 — Reserve / release / settle through one transaction per action
Each bid action runs in a single `WithinTx` with the auction row and affected
wallets locked (`SELECT … FOR UPDATE`). Placing a bid releases the previous
leader's reservation and reserves the new one atomically; settlement turns the
winner's reservation into a debit and credits the seller. Concurrent bids are
therefore linearized and a guild can never over-commit.

### D12 — Daily cap counts realized spend (buys + auction wins), reset at UTC midnight
The cap is enforced from the ledger sum of `debit`+`settle` entries since UTC
midnight. Bids are checked at placement so that a *winning* bid cannot breach the
cap; reservations themselves don't count (only realized outflows do). **Known
edge:** a guild could in principle hold two simultaneous winning bids that each
passed the check before either settled — acceptable for the challenge scope and
noted here.

### D13 — Price Oracle is advisory/display-only, handled defensively
The oracle price is shown for reference but never gates a trade. Reads are bound
by a short timeout (a slow oracle never blocks a request), non-positive readings
are rejected, and the last good value is cached per item so display degrades
gracefully. The mock can inject latency and faults for tests.

### D14 — Idempotency via `Idempotency-Key` header
Mutating POSTs may carry an `Idempotency-Key`; a replay returns the original
response without re-applying. This complements the data-layer invariants (stock,
single-active-auction, balance checks) that already prevent double-effect.
**Off-trade:** the key store is in-process (not durable across restarts).

### D15 — Reviewer console served at `/` (Go templates + fetch)
A single-page console exercises every feature in a browser so a reviewer can QA
without curl. It is a thin client over the JSON API: the Go template only injects
the seeded guild list; all actions are `fetch` calls carrying `X-Guild-Id`.

## Off-trade (corners cut, intentionally)

- No real auth (D8); guild identity is a trusted header.
- Idempotency cache is in-memory, not persisted (D14).
- Daily-cap simultaneous-winning-bids edge case (D12) is not closed.
- `UpdateAuction` re-materializes a small bid set (delete+insert) rather than
  diffing — simple and correct within the transaction, fine at challenge scale.

## What I'd add with more time

- Postgres-level concurrency/race tests (parallel bidders) in CI with a real DB.
- Durable idempotency keys and a money double-entry invariant check job.
- Circuit-breaker / staleness windows around the oracle.
- AuthN/Z and per-guild rate limiting.

## Consequences

- The full stack needs Postgres (run model: Postgres-only); the domain and its
  unit tests still run with zero infrastructure.
- The guarantees are enforced at the storage layer (locks + a partial unique
  index), which is the defensible place for a money-handling system.
