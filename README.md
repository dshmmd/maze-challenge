# Dragon Market — Backend Engineering Challenge (Go)

> A secure marketplace core for buying and auctioning the legendary items of
> Aethoria — correct, auditable, and over-commitment-proof under duplicate
> requests, flaky external services, and concurrent bids.

<!-- This README is my note to the reviewers. I'll fill each section below in my
     own words. Deeper rationale lives in docs/adr/ and ROADMAP.md. -->

## Overview

<!-- One short paragraph: what this service is and the 3 guarantees it upholds. -->

## How I approached it

<!-- My POV: how I worked on this, what I asked the AI agent to do, what I drove
     myself, and how decisions were made. -->

## Architecture & key decisions

<!-- Short bullets. Stack: Go, PostgreSQL, chi, hexagonal. Why each — or point to
     docs/adr/0001-foundation.md. -->

## Tech stack

<!-- Go 1.25 · PostgreSQL · chi · docker-compose. -->

## How to run

```bash
make up      # app + Postgres via docker-compose, then open http://localhost:8080/
make check   # fmt + vet + build + test (the verification gate)
```

The console at `/` lets you switch guilds and exercise every feature. Seeded
guilds: `ironband`, `stormforge`, `shadowveil`. Auction window/extension are
configurable (`AUCTION_WINDOW`, `AUCTION_EXTENSION`); compose uses short values
so settlement is observable.

## API

<!-- Terse reference; all ✅ implemented. A browser console at `/` exercises
     everything without curl. Identity: send `X-Guild-Id: <guild>` on mutating
     requests; optional `Idempotency-Key` makes a retried POST safe. -->

| Method | Path | Purpose | |
|--------|------|---------|--|
| `GET` | `/` | Web console (test every feature in a browser) | ✅ |
| `GET` | `/healthz` | Liveness | ✅ |
| `POST` | `/items` | List an item (Legendary auto-opens an auction) | ✅ |
| `GET` | `/items`, `/items/{id}` | List / details (+ advisory oracle price) | ✅ |
| `POST` | `/items/{id}/purchase` | Buy a limit-order item (Common/Rare) | ✅ |
| `POST` | `/items/{id}/bid` | Place a bid (Legendary auction) | ✅ |
| `DELETE` | `/items/{id}/bid/{bid_id}` | Cancel a non-leading bid | ✅ |
| `GET` | `/auctions`, `/auctions/{id}` | Active auctions / details | ✅ |
| `GET` | `/guilds/{id}/wallet` | Wallet total/reserved/available | ✅ |
| `GET` | `/guilds/{id}/ledger` | Full transaction history (audit) | ✅ |

## Testing

<!-- What's covered and how to run it. -->

## Design Q&A

<!-- Short answers to the challenge's ADR questions — edit in my own words.
     Full notes in docs/adr/0001-foundation.md. -->

**Why this structure?**
I used a hexagonal layout so the business rules (wallets, auctions, bids) stay
pure and unit-testable, with the external pieces — Postgres, the flaky Price
Oracle, the clock — behind interfaces I can mock. I picked PostgreSQL because the
hard guarantees (no duplicate sale, no over-commitment under concurrent bids) are
really about atomicity and isolation, which the database enforces with
transactions and row locks instead of hopeful application code.

**Where did I cut corners (off-trade)?**
<!-- e.g. minimal auth, semantics I simplified — fill in. -->

**What would I add with more time?**
<!-- e.g. richer oracle resilience, fuller audit ledger, more race tests — fill in. -->

## Notes

<!-- AI assistance, anything else for the reviewer. -->
