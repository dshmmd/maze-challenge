# Phase 0 — Foundation scaffold

**Date:** 2026-06-27
**Status:** shipped (pending green gate + initial commit)

## What was done

- Established the **hexagonal** layout: `internal/domain` (pure), `internal/ports`
  (interfaces), `internal/adapters/{httpapi,oracle,clock}`, `cmd/server`.
- **Domain `Wallet`** with the reserve-not-deduct model: `Available = Total −
  Reserved`, plus `Reserve / Release / Settle / Debit / Credit`, and a sentinel
  error catalog (`internal/domain/errors.go`). Fully unit-tested
  (`wallet_test.go`) — no DB needed.
- `Gold` as `int64` whole-unit money (no floats).
- Running HTTP slice: chi server with `GET /healthz`.
- Mock flaky **Price Oracle** and real + fake **Clock** behind ports.
- `make check` gate (fmt + vet + build + test), Dockerfile, docker-compose
  (app + Postgres), `.env.example`.
- Full **doc harness**: README (the company-facing decision record), ROADMAP
  (R#/D#/Q# ledgers + phased plan), ADR-0001, slash-commands.

## Why / decisions

- See `docs/adr/0001-foundation.md` for D1–D6 (Postgres, hexagonal, chi,
  injectable clock, integer gold, reserve-not-deduct). All human-made.

## Gotchas / notes for the next agent

- **README is the only channel to the reviewing company.** Foundational design
  questions must go to the human and be recorded there — do not silently default.
- Phase 0 tests are pure unit tests; **no Postgres yet**. The DB-backed repos,
  migrations, and concurrency/race tests start in Phase 1.
- Open questions Q1–Q5 (auth/guild identity, daily-cap semantics, oracle's
  gating role, idempotency mechanism, Rare stock model) are **unresolved** —
  resolve the relevant one before its phase.
- Layering: `domain` imports nothing outward; wiring lives only in `cmd/`.
