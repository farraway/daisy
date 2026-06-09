## Trigger

Read this prompt when beginning substantive implementation work, reviewing code,
or when the quality and shape of the output matters as much as correctness.

## Rules

1. **Read before inventing.** Understand what exists and infer intent from code before proposing anything new.
2. **Propose before large changes.** Explain what you intend to do and why; do not begin extensive implementation on an unconfirmed design.
3. **Smallest correct diff.** Implement only what solves the stated problem; no drive-by refactors or "while I'm here" changes.
4. **Stop at scope boundary.** If a need surfaces outside the current step, add it to Open Questions or Deferred — do not implement it.
5. **Consult a paragon.** Before designing, find a well-regarded implementation of the same concept in the codebase and read it.
6. **Dead code must be removed.** Nothing unused, commented-out, or "for later" survives a step.
7. **Silent failure is unacceptable.** Errors must be visible and actionable, with location and how to fix.
8. **Check off and record deviations.** After each step, update PLAN.md: mark the step done and record any deviations in the Decisions section.
9. **Distinguish essential from accidental complexity.** Before implementing, ask: is this complexity inherent to the problem, or an artifact of the current approach? Do not preserve or extend accidental complexity.

## North Star

The goal is code that looks like what you would have designed had you truly
understood the problem — not a generic solution forced onto it, and not a
clever abstraction waiting for problems it does not have.

**The abstraction should fit.** Names, boundaries, and structure should feel
inevitable in hindsight. If the design needs a long explanation, it is probably
wrong.

## Implementation

- **Match conventions** — naming, layout, error style, patterns already in use.
- **Resist over-engineering** — no premature generalisation, no abstraction layers with only one caller.
- **Comments sparingly** — comment only non-obvious constraints, invariants, or trade-offs.
- **Tests when they earn their keep** — protect real behaviour or document contracts; skip tests that assert the obvious.

## Review

**Post (after each step):** audit the change as if reviewing someone else's pull request.

- **Fit** — does the solution match the problem's natural seams? Does it address essential complexity without adding accidental complexity?
- **Naming** — consistent, accurate, neither vague nor over-specific?
- **Size** — functions and modules readable without scrolling; split when responsibility has clearly diverged.
- **Boundaries** — logic lives at the right layer; nothing leaking across concerns.
- **Duplication** — shared behaviour extracted only when repetition is real and stable, not speculative.
- **Dead code** — nothing left unused, commented-out, or "for later".
- **Errors** — failures are visible and actionable; include location and how to fix. Silent failure is unacceptable.
- **Surface area** — public API is intentional; internals stay internal.

**Final (after larger spans of work):** read through the affected surface as a whole, not file by file. Check: API coherence and minimal surface; dependency direction (no cycles, no upward leaks); test coverage for behaviour actually added or changed. Ask: would a newcomer infer the design from the code alone?

## Communication

- Explain **what** changed and **why** — not a tour of every line.
- Be direct about trade-offs and uncertainty.
- Match response depth to task depth; a small fix does not need an essay.
- When the design is ambiguous, ask or propose options — do not guess and build extensively on the guess.

## See Also

- [`daisy/docs/praxis.md`](daisy/docs/praxis.md) — Extended guidance: how we work, before/during implementation, anti-patterns
