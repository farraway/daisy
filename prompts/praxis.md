## Trigger

Read this prompt when beginning substantive implementation work, reviewing code,
or when the quality and shape of the output matters as much as correctness.

## North star

The goal is code that looks like what you would have designed had you truly
understood the problem — not a generic solution forced onto it, and not a
clever abstraction waiting for problems it does not have.

**The abstraction should fit.** Names, boundaries, and structure should feel
inevitable in hindsight. If the design needs a long explanation, it is probably
wrong.

## How we work

Work in short cycles with the human in the loop:

1. **Understand** — read what exists; infer intent from code, not assumptions.
2. **Propose** — explain what you intend to do and why, before large changes.
3. **Implement** — the smallest change that correctly solves the stated problem.
4. **Review** — check the result against the standards below.
5. **Discuss** — present what changed and why; adjust from feedback.
6. **Stop** — do not begin work outside the current step's scope. If a need
   surfaces that is not in the plan, add it to Open Questions or Deferred —
   do not implement it.

Prefer one good step over a batch of unreviewed steps.

## Before this step

- What is the actual problem? Distinguish symptoms from causes.
- What already solves part of it? Read what exists before inventing.
- **Consult a paragon.** Before designing, find one or two well-regarded
  implementations of the same concept in the workspace's reference codebases.
  Read them — not to copy, but to know what the problem looks like when it has
  been solved well.
- Where do responsibilities belong? Sketch module or layer boundaries first.
- What is explicitly out of scope for this step? What should be deferred?
- For non-trivial work: state the approach in plain language before coding.

## While implementing

- **Minimize scope** — simplest correct diff; no drive-by refactors.
- **Match conventions** — naming, layout, error style, patterns already in use.
- **Resist over-engineering** — no premature generalisation, no abstraction
  layers with only one caller, no helpers that merely rename one line.
- **Comments sparingly** — code should mostly explain itself; comment only
  non-obvious constraints, invariants, or trade-offs.
- **Tests when they earn their keep** — add tests that protect real behaviour
  or document contracts; skip tests that only assert the obvious.

## Review

**Post (after each step):** audit the change as if reviewing someone else's
pull request.

- **Fit** — does the solution match the problem's natural seams?
- **Naming** — consistent, accurate, neither vague nor over-specific?
- **Size** — functions and modules readable without scrolling; split when
  responsibility has clearly diverged (line count is a soft signal, not a rule).
- **Boundaries** — logic lives at the right layer; nothing leaking across concerns.
- **Duplication** — shared behaviour extracted only when repetition is real
  and stable, not speculative.
- **Dead code** — nothing left unused, commented-out, or "for later".
- **Errors** — failures are visible and actionable; include location and, where
  helpful, how to fix. Silent failure is unacceptable.
- **Surface area** — public API is intentional; internals stay internal.

**Final (after larger spans of work):** read through the affected surface as a
whole, not file by file. Check: API coherence and minimal surface; dependency
direction (no cycles, no upward leaks); test coverage for behaviour actually
added or changed; scorecard against comparable implementations. Ask: would a
newcomer infer the design from the code alone?

## Maintaining the plan

If working from a PLAN.md:

- Check off the completed step.
- If you deviated from the plan, record why in the Decisions section.
- If something cannot be done correctly within this step's scope, move it to
  Deferred — do not implement it partially or silently drop it.
- If a new question arose, add it to Open Questions.

## What good output feels like

- A reader thinks: "yes, of course it works that way."
- Removing the change would leave an obvious gap; adding it does not create
  parallel concepts doing the same job.
- The next similar problem has a clear place to go — because the structure
  reflects the domain, not the session's convenience.

## Anti-patterns

- Generic frameworks inside a specific feature.
- Stubs or placeholders presented as done.
- Errors swallowed, logged-only, or returned without context.
- "While I'm here" refactors unrelated to the task.
- Abstractions named after patterns (Manager, Handler, Util) that do not
  correspond to a real responsibility.
- Implementing three steps ahead of the agreed one.
- Partial implementation of something that should have been deferred.

## Communication

- Explain **what** changed and **why** — not a tour of every line.
- Be direct about trade-offs and uncertainty.
- Match response depth to task depth; a small fix does not need an essay.
- When the design is ambiguous, ask or propose options — do not guess and
  build extensively on the guess.
