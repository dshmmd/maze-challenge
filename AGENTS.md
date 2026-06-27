# AGENTS.md — Dragon Market

This mirrors `CLAUDE.md` so Codex and other agents get the same context. When in
doubt, **`CLAUDE.md` is canonical** — keep the two in sync.

## Start here

1. Read `CLAUDE.md` (canonical), then `ROADMAP.md` and `memory/INDEX.md`.
2. Resume the active phase in `ROADMAP.md`. One phase at a time; stop at the gate.

## The rules that matter most

- **Take-home challenge.** Full brief in `REQUIREMENTS.pdf`; summary + business
  rules in `README.md`.
- **README is the human's channel to the company — keep it short, don't write
  prose into it.** Foundational design decisions are the **human's** — ask, don't
  default — and get recorded in `ROADMAP.md` (D#) + an ADR with rationale (the
  human summarizes in the README themselves). Implementation choices are the
  agent's; note them in `ROADMAP.md`.
- **Green before commit.** `make check` (fmt + vet + build + test) must pass.
- **Phases end at owner-review gates.** Finish, verify, stop, summarize.
- **Decisions durable; memory per milestone.** Update `ROADMAP.md` and append a
  `memory/*.md` entry + `INDEX.md` line per shipped phase.
- **Domain stays pure;** external deps behind `internal/ports`. Wiring only in
  `cmd/`.

## Commit trailer

End every commit body with:

```
AI-Assisted: true
AI-Model: <model that did the work, e.g. GPT-5-Codex / Claude Opus 4.8>
```

## Workflows (Codex equivalents of the slash-commands)

- **Continue** (`/continue`): read the boot docs, identify the active phase, do
  the smallest correct next slice, run `make check`, update ROADMAP/memory, stop
  at the gate and summarize.
- **Add a feature** (`/add-feature`): confirm it fits the current phase (or flag
  it as new scope to the human), implement at the right layer, add tests, run the
  gate, document.
- **Report a bug** (`/report-bug`): reproduce with a failing test, fix at the
  root cause, confirm the gate is green, note the gotcha in memory.

See `CLAUDE.md` for the full architecture map and layering rules.
