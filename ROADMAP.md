# Dragon Market — Roadmap

The persistent product plan. Survives context loss between sessions. Read this
(and `memory/INDEX.md`) at the start of every session.

> **Communication channel rule:** This is a take-home challenge. The
> **README is the human's channel to the reviewing company** — short, written in
> the human's own voice. Every *foundational* design decision is the author's
> (the human's) to make — an agent surfaces the question, the human decides, and
> the decision is recorded here (D#) and in an ADR with rationale. Agents keep
> the README skeleton terse and do not write narrative into it; the human fills
> it. Implementation-level choices may be made by the agent and noted below.

---

## Vision

Build the **core** of a secure marketplace for Aethoria: guilds list Common/Rare
items at fixed prices (limit orders) and auction one-of-a-kind Legendary items.
The system must uphold three non-negotiable guarantees under duplicate requests,
flaky external services, and concurrent bids:

1. No invalid or duplicate sale of an asset.
2. No guild over-commits beyond available gold or its daily cap.
3. Reliable and auditable behavior even with unreliable external services.

Target: a defensible, well-tested core — not a feature-complete product. Expected
effort 1–3 working days.

---

## Requirements ledger (R#)

Traceability from spec → code. Each rule should have a test that locks it in.

| R# | Requirement | Status |
|----|-------------|--------|
| R1 | Legendary item is unique; never sold/duplicated twice | ⏳ planned |
| R2 | Limit order (Common/Rare): fixed price at listing; buyer with funds buys | ⏳ planned |
| R3 | One active auction per Legendary item at a time | ⏳ planned |
| R4 | Auction window configurable (default 24h) | ⏳ planned |
| R5 | Highest bidder wins | ⏳ planned |
| R6 | Bid in final 5 min extends window by 5 min (anti-snipe) | ⏳ planned |
| R7 | Bidding reserves funds (not deducted); losing releases immediately | 🟡 wallet logic done |
| R8 | Auction with no bids returns item to *available* | ⏳ planned |
| R9 | Wallet: available = total − reserved; every txn traceable | 🟡 arithmetic done |
| R10 | Guild cannot bid on / buy its own item | ⏳ planned |
| R11 | New bid ≥ 5% above current highest | ⏳ planned |
| R12 | Cancel bid only if not the current highest | ⏳ planned |
| R13 | Available balance (incl. reservations) checked before any commitment | 🟡 wallet logic done |
| R14 | Guild daily purchase cap enforced | ⏳ planned |
| R15 | Price Oracle mocked behind interface; zero/negative/slow handled | ⏳ planned |
| R16 | Idempotency / no double-effect under duplicate requests | ⏳ planned |
| R17 | Concurrency-safe (concurrent bids produce a defensible result) | ⏳ planned |

---

## Decisions (D#)

Foundational decisions (D1–D6) are human-made and mirrored in `README.md`.

| D# | Decision | Rationale | By |
|----|----------|-----------|----|
| D1 | PostgreSQL datastore | ACID + row locking enforce money/uniqueness invariants in the DB | human |
| D2 | Hexagonal architecture | Pure, testable domain; external deps behind interfaces (mockable) | human |
| D3 | chi on net/http | Idiomatic, light routing; handlers are a thin adapter | human |
| D4 | Injectable Clock + background sweeper | Deterministic time tests; auto-settle expired auctions | human |
| D5 | Integer gold (int64), no floats | Exact money; reserves net to zero | human |
| D6 | Reserve-not-deduct for bids | Honest spendable balance; models spec directly | human |
| D7 | `make check` = fmt + vet + build + test | One green gate before any commit | agent |

---

## Open questions (Q#)

Surface these to the human before locking the relevant slice; record answers in
README + a Decisions row.

| Q# | Question | Notes |
|----|----------|-------|
| Q1 | **Guild identity / auth** — how is the acting guild identified per request? | Default proposal: `X-Guild-Id` header, no real auth (out of scope for a core challenge). Needs human sign-off. |
| Q2 | **Daily purchase cap semantics** — does "purchase" count limit-order buys only, or also winning auction settlements? Reset at UTC midnight? | Affects R14 design. |
| Q3 | **Oracle price role** — is the oracle price advisory (display only) or does it gate listings/bids (e.g. reject far-off prices)? | Spec lists items "with current price"; needs clarity. |
| Q4 | **Idempotency mechanism** — `Idempotency-Key` header on POST, or natural keys only? | Drives R16 approach. |
| Q5 | **Rare stock model** — does Rare have a finite countable stock, or just "limited & regenerates over time"? | Affects item/listing schema. |

---

## Phased plan

Each phase ends at an **owner-review gate**: finish, run `make check`, stop, show it.

### Phase 0 — Foundation scaffold ✅ (in progress)
- [x] Go module, repo layout, `.gitignore`
- [x] Domain: `Gold`, `Wallet` (reserve/release/settle/debit) + unit tests
- [x] Domain error catalog
- [ ] Ports: `PriceOracle`, `Clock`, repository interfaces
- [ ] Adapters: mock flaky `PriceOracle`, real + fake `Clock`
- [ ] HTTP slice: chi server + `GET /healthz`
- [ ] `cmd/server/main.go` wiring + config from env
- [ ] `Makefile` (`check`/`test`/`run`/`up`), `Dockerfile`, `docker-compose.yml`, `.env.example`
- [ ] Docs: README, ROADMAP, ADR-0001, memory seed, slash-commands
- [ ] Green `make check` → initial commit

### Phase 1 — Items & limit orders (Common/Rare)
Resolve Q1, Q5. Item domain + rarity; create listing; buy at fixed price with
atomic wallet debit + single-fulfilment guarantee (R1, R2, R10, R13). Postgres
repo + migrations. Tests for double-buy and insufficient-funds races.

### Phase 2 — Legendary auctions & bids
Resolve Q2. Auction lifecycle, bid reserve/release, 5% increment, anti-snipe
extension, single-active-auction, cancel rules, no-bid expiry (R3–R8, R11, R12).
Background settlement worker on injectable clock. Concurrency tests (R17).

### Phase 3 — Oracle integration & resilience
Resolve Q3. Wire mock oracle into pricing; reject zero/negative/stale; timeouts
and fallbacks (R15). Surface current price in `GET /items`.

### Phase 4 — Durability & audit
Resolve Q2, Q4. Daily cap enforcement (R14), idempotency (R16), transaction
ledger / audit trail, observability. Harden concurrency guarantees.

### Phase 5 — Delivery polish
docker-compose end-to-end, README walkthrough, final ADR pass ("where I cut
corners / what I'd add with more time"), seed data, example requests.

---

## Working agreement (short)

- Read this file + `memory/INDEX.md` first, every session.
- Foundational design questions go to the human → recorded in README.
- Green `make check` before any commit; never commit a red tree.
- One coherent phase, then stop at the owner-review gate.
- Each shipped phase appends a `memory/` entry.
- Commits are brief and AI-attributed (see `CLAUDE.md`).
