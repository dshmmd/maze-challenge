Continue work on **Dragon Market** (Go take-home challenge).

1. **Re-derive context.** Read `CLAUDE.md`, then `ROADMAP.md` (R#/D#/Q# + phased
   plan) and `memory/INDEX.md` + the latest `memory/*.md` entry.
2. **Identify the active phase** and the smallest correct next slice within it.
   Do not jump ahead to later phases.
3. **Check for foundational decisions.** If the next slice depends on an open
   question (Q#) or any foundational choice, **ask the human first** — that
   answer goes in `README.md` (+ ADR) as a Decision, since the README is the
   only channel to the reviewing company. Do not silently default.
4. **Implement** at the right layer (domain pure; external deps behind ports;
   wiring only in `cmd/`). Match the surrounding code. Add/extend tests that lock
   in the relevant R# rule.
5. **Verify:** run `make check`. Fix until green. Never leave a red tree.
6. **Document:** update `ROADMAP.md` (R# statuses, Decisions, Q#), append/refresh
   the `memory/*.md` entry, and keep `README.md` current.
7. **Stop at the owner-review gate.** Summarize: what now exists and that it's
   green, decisions captured, open questions for the human, recommended next phase.
8. **Commit only if complete and green**, with the AI-attribution trailer from
   `CLAUDE.md`. Don't push unless asked.
