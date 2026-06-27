# CLAUDE.md — Dragon Market

Single source of truth for any agent working on this repo. Read this first.

## ▶ START HERE (boot sequence, every session)

1. Read this file, then **`ROADMAP.md`** (plan, R# requirements, D# decisions,
   Q# open questions) and **`memory/INDEX.md`** (what shipped + gotchas).
2. Skim the most recent `memory/*.md` entry for the current state.
3. Pick up the active phase from `ROADMAP.md`. Do not sprawl across phases.

## What this is

A **take-home challenge**: a Go backend for a secure marketplace ("Dragon
Market"). The full brief is `REQUIREMENTS.pdf`. The three non-negotiable
guarantees and the business rules are in `README.md` and `ROADMAP.md`.

## Communication-channel rule (important)

The **`README.md` is the human's channel to the reviewing company — not an
agent doc.** It is short and the **human writes its prose** (their POV on how the
project was built). Therefore:

- **Do not bloat the README.** Don't add long agent-written narrative there. Keep
  the section skeleton intact; if a factual stub needs filling (e.g. run commands,
  API table), keep it terse. Substance and rationale go in `ROADMAP.md` + `docs/adr/`.
- **Foundational design decisions belong to the human.** When a foundational
  choice arises (datastore, architecture, security model, money model, API
  contract shape, anything in the Q# list), **ask the human, do not silently
  default.** Record the decision in `ROADMAP.md` Decisions (D#) and an ADR with
  rationale. The human may then summarize it in the README in their own words.
- Implementation-level choices may be made by the agent; note them in the
  `ROADMAP.md` Decisions table marked `agent`.

## Working agreement

- **One source of truth, read first** (the boot sequence above).
- **Phases end at owner-review gates.** Finish a coherent phase, run the gate,
  stop, summarize. Don't start the next phase unprompted.
- **Green before commit.** `make check` (fmt + vet + build + test) must pass.
  Never commit a red tree.
- **Decisions are durable.** Architectural choices → `ROADMAP.md` Decisions +
  README/ADR, not just chat.
- **Memory per milestone.** Each shipped phase appends a `memory/` entry and an
  `INDEX.md` line.
- **Match the surrounding code.** Minimal changes at the root cause; tests lock
  in behavior. The domain stays pure (no I/O); external deps stay behind ports.
- **Results, not play-by-play.** Report outcomes, blockers, decisions, questions.

## Commits

Brief subject + short body (what changed and why, no play-by-play). End every
commit body with:

```
AI-Assisted: true
AI-Model: Claude Opus 4.8
```

Commit only when complete and green. Don't push unless asked. If on the default
branch, branch first.

## Architecture (map)

```
cmd/server/        entrypoint: load config → wire adapters → run HTTP + worker
internal/
  domain/          pure business types & rules (no I/O): money, wallet, item,
                   auction, bid, errors. Unit-tested here.
  ports/           interfaces the core depends on: repositories, PriceOracle, Clock
  adapters/
    httpapi/       chi handlers; HTTP↔domain; maps domain errors → status codes
    oracle/        mock flaky Price Oracle (interface impl)
    clock/         real clock + fake (advanceable) clock for tests
docs/adr/          architecture decision records
memory/            per-milestone build log
```

Layering rule: `domain` imports nothing from `adapters`/`ports`. `ports` may
reference `domain` types. `adapters` implement `ports`. Wiring happens only in
`cmd/`.

## Verification gate

`make check` = `gofmt` check + `go vet` + `go build ./...` + `go test ./...`.
This must be green before any commit.

## Commands (in `.claude/commands/`)

- `/continue` — re-derive context and advance the active phase.
- `/add-feature` — add scoped functionality within the working agreement.
- `/report-bug` — reproduce, fix at root cause, lock with a test.
