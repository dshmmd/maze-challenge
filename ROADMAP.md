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
| R1 | Legendary item is unique; never sold/duplicated twice | ✅ single-fulfilment + partial unique index (one active auction/item) |
| R2 | Limit order (Common/Rare): fixed price at listing; buyer with funds buys | ✅ done (service + HTTP + tests) |
| R3 | One active auction per Legendary item at a time | ✅ enforced (service check + DB partial unique index) |
| R4 | Auction window configurable (default 24h) | ✅ `AUCTION_WINDOW` config |
| R5 | Highest bidder wins | ✅ settlement tested |
| R6 | Bid in final 5 min extends window by 5 min (anti-snipe) | ✅ domain + test (`AUCTION_EXTENSION`) |
| R7 | Bidding reserves funds (not deducted); losing releases immediately | ✅ reserve/release/settle through tx + tests |
| R8 | Auction with no bids returns item to *available* | ✅ tested |
| R9 | Wallet: available = total − reserved; every txn traceable | ✅ full ledger (reserve/release/settle/debit/credit) + `/ledger` |
| R10 | Guild cannot bid on / buy its own item | ✅ both paths tested |
| R11 | New bid ≥ 5% above current highest | ✅ domain + tests |
| R12 | Cancel bid only if not the current highest | ✅ domain + HTTP |
| R13 | Available balance (incl. reservations) checked before any commitment | ✅ enforced in Purchase + Bid |
| R14 | Guild daily purchase cap enforced | ✅ buys + wins, UTC reset (D12) |
| R15 | Price Oracle mocked behind interface; zero/negative/slow handled | ✅ advisory, timeout + last-good cache (D13) |
| R16 | Idempotency / no double-effect under duplicate requests | ✅ tx invariants + `Idempotency-Key` (D14), verified |
| R17 | Concurrency-safe (concurrent bids produce a defensible result) | ✅ Postgres `FOR UPDATE` locking; limit-order race test under `-race` |

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
| D8 | Guild identity via `X-Guild-Id` header, no real auth | Keeps focus on market correctness (what's graded); explicit off-trade | human |
| D9 | Limit-order listing carries price + quantity | Models "Rare = limited stock"; one listing serves many buyers; Legendary fixed to qty 1 | human |
| D10 | `POST /items/{id}/purchase` for limit-order buys | Proposed API only listed `/bid` (auctions); limit orders need their own buy endpoint | agent |
| D11 | Unit-of-Work `TxManager.WithinTx`; in-memory store (tests) + Postgres adapter (prod) | Purchase/bid logic stays in the service; atomic boundary is a tx (Postgres `FOR UPDATE`) or lock (memory), so the backend is swappable | agent |
| D12 | Daily cap = realized buys + auction wins, UTC-midnight reset | "Prevent monopoly on daily spend"; enforced from the ledger; checked at bid time so a winning bid can't breach it | human |
| D13 | Price Oracle advisory/display-only; defensive (timeout + reject non-positive + last-good cache) | Don't punish users for a flaky external service; never gate trades on it | human |
| D14 | Idempotency via `Idempotency-Key` header (in-process store) | Safe client retries on top of data-layer invariants; off-trade: not durable across restarts | human |
| D15 | Reviewer console at `/` (Go templates + fetch over the JSON API) | Lets a reviewer QA every feature in a browser with zero curl | human |

---

## Open questions (Q#)

All resolved. Kept for traceability.

| Q# | Question | Resolution |
|----|----------|-----------|
| Q1 | Guild identity / auth | ✅ D8 — `X-Guild-Id` header, no real auth |
| Q2 | Daily purchase cap semantics | ✅ D12 — buys + auction wins, UTC reset |
| Q3 | Oracle price role | ✅ D13 — advisory/display-only, handled defensively |
| Q4 | Idempotency mechanism | ✅ D14 — `Idempotency-Key` header |
| Q5 | Rare stock model | ✅ D9 — listing carries price + quantity |

---

## Phased plan

Each phase ends at an **owner-review gate**: finish, run `make check`, stop, show it.

### Phase 0 — Foundation scaffold ✅ done
- [x] Go module, repo layout, `.gitignore`
- [x] Domain: `Gold`, `Wallet` (reserve/release/settle/debit) + unit tests
- [x] Domain error catalog
- [x] Ports: `PriceOracle`, `Clock`, repository interfaces
- [x] Adapters: mock flaky `PriceOracle`, real + fake `Clock`
- [x] HTTP slice: chi server + `GET /healthz`
- [x] `cmd/server/main.go` wiring + config from env
- [x] `Makefile` (`check`/`test`/`run`/`up`), `Dockerfile`, `docker-compose.yml`, `.env.example`
- [x] Docs: README, ROADMAP, ADR-0001, memory seed, slash-commands
- [x] Green `make check`

### Phase 1 — Items & limit orders (Common/Rare) 🟢 core done (in-memory)
Q1/Q5 resolved (D8/D9). Shipped: item domain + rarity; `POST/GET /items`,
`GET /items/{id}`, `POST /items/{id}/purchase`, `GET /guilds/{id}/wallet`;
atomic buy with wallet debit/credit + ledger; single-fulfilment guarantee
(R1, R2, R10, R13); concurrency test under `-race` (R17). Backed by the
in-memory store behind the `TxManager`/`Repo` ports.

### Phase 1b — Postgres persistence ✅ done
`Repo`/`TxManager` implemented against Postgres (pgx) with `SELECT … FOR UPDATE`
locking; embedded schema applied on boot (partial unique index for R3); wired via
`DATABASE_URL`; verified end-to-end against a real DB (limit buy, bids, settlement).

### Phase 2 — Legendary auctions & bids ✅ done
Auction lifecycle, bid reserve/release, 5% increment, anti-snipe extension,
single-active-auction, cancel rules, no-bid expiry (R3–R8, R11, R12). Background
settlement worker on the injectable clock; domain + service tests with a fake clock.

### Phase 3 — Oracle integration & resilience ✅ done
Mock oracle wired into item views as advisory price; timeout, reject non-positive,
last-good cache (R15, D13).

### Phase 4 — Durability & audit ✅ done
Daily cap (R14, D12), idempotency via `Idempotency-Key` (R16, D14), full ledger +
`GET /guilds/{id}/ledger` audit view (R9).

### Phase 5 — Delivery polish ✅ done
docker-compose end-to-end (short auction window for observability), reviewer
console at `/` (D15), ADR-0002, seed data. **Remaining (human):** README prose
in the author's voice.

> **All R1–R17 satisfied; all Q# resolved.** Possible follow-ups live in
> ADR-0002 "What I'd add with more time" (Postgres race tests in CI, durable
> idempotency, oracle circuit-breaker, auth).

---

## Working agreement (short)

- Read this file + `memory/INDEX.md` first, every session.
- Foundational design questions go to the human → recorded in README.
- Green `make check` before any commit; never commit a red tree.
- One coherent phase, then stop at the owner-review gate.
- Each shipped phase appends a `memory/` entry.
- Commits are brief and AI-attributed (see `CLAUDE.md`).
