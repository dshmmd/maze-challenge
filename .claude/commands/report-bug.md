Report and fix a bug in **Dragon Market**: $ARGUMENTS

1. **Load context** (`CLAUDE.md` → `ROADMAP.md` → latest `memory/*.md`).
2. **Reproduce first.** Write a failing test that captures the bug — ideally at
   the domain layer if it's a rule violation (money, uniqueness, auction state),
   or an HTTP/integration test otherwise.
3. **Fix at the root cause**, not the symptom. Make the minimal change; keep the
   domain pure and the layering intact. If the bug reveals a flawed foundational
   decision, flag it to the human and update `README.md`/ADR.
4. **Verify:** `make check` green, and confirm the new test now passes.
5. **Lock it in.** Keep the regression test. Note the gotcha in the relevant
   `memory/*.md` so the next agent doesn't reintroduce it.
6. **Summarize** the root cause, the fix, and the test added. Commit only when
   green, with the attribution trailer.
