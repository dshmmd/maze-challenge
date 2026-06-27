Add a feature to **Dragon Market**: $ARGUMENTS

1. **Load context** (`CLAUDE.md` → `ROADMAP.md` → `memory/INDEX.md`).
2. **Scope it.** Does this fit the active phase? If it's a foundational choice or
   changes the API contract / data model / a Q# item, **surface it to the human**
   and record the decision in `README.md` (+ ADR) before building — the README is
   the only channel to the reviewing company.
3. **Place it correctly.** Business rules in `internal/domain` (pure); external
   I/O behind `internal/ports` with an adapter; HTTP in `internal/adapters/httpapi`;
   wiring only in `cmd/`. Match existing style.
4. **Test it.** Add unit tests for the rule; add integration/concurrency tests if
   it touches money, uniqueness, or auctions. Map it to an R# in `ROADMAP.md`.
5. **Verify:** `make check` green.
6. **Document:** update `ROADMAP.md` (R#/Decisions), `README.md` if reviewer-facing,
   and the relevant `memory/*.md`.
7. **Summarize** outcome, decisions, and any new open questions. Commit only when
   green, with the attribution trailer.
