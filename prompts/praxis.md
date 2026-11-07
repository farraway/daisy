## Trigger

Read this prompt when beginning substantive implementation work, reviewing code,
or when the quality and shape of the output matters as much as correctness.

## Rules

1. **Read before inventing.** Understand what exists and infer intent from code before proposing anything new.
2. **Propose before large changes.** Explain what you intend to do and why; do not begin extensive implementation on an unconfirmed design.
3. **Smallest diff is a mode, not a goal.** It governs fixes inside a stable abstraction and the execution of a chosen design: no extras, no speculative generality. It never decides the size of a design change.
4. **Scope is relatedness, not size.** A need unrelated to the step's problem goes to Open Questions or Deferred — do not implement it. A rearchitecture whose cause is the step's own problem is in scope. Under a plan, record it in the plan's Decisions with the alternative considered and proceed; stop only where it crosses a decision the user has made. Outside a plan, propose it first (Rule 2).
5. **Consult a paragon.** Before designing, find a well-regarded implementation of the same concept in the codebase and read it.
6. **Dead code must be removed.** Nothing unused, commented-out, or "for later" survives a step.
7. **Silent failure is unacceptable.** Errors must be visible and actionable, with location and how to fix.
8. **Check off and record deviations; log decisions that clear the ADR bar.** After each step, update PLAN.md: mark the step done and record any deviations in the Decisions section. A decision earns an ADR entry only when all three hold: hard to reverse, surprising without context, and the result of a real trade-off. If a decision is easy to reverse, not surprising, or had no genuine alternative, don't log it beyond "did the obvious thing."
9. **Distinguish essential from accidental complexity.** Before implementing, ask: is this complexity inherent to the problem, or an artifact of the current approach? Do not preserve or extend accidental complexity.
10. **Test-first at pre-agreed seams.** Where a step has a clear testable seam, use the `testing` skill (its long form, with the reading behind it, is [`docs/testing.md`](docs/testing.md)) — write the failing test before the implementation, not as an afterthought. Where no such seam exists, say so rather than skipping tests silently.
11. **Reshape a seam patched twice, sized to the missing rule.** When a fix is the second patch to one seam, needs a flag meaning "the previous flag, but also...", or adds a special case with no home in the structure, the abstraction is wrong. Read the whole seam, state the missing rule in one sentence, and reshape the smallest unit that gives that rule a home. A rewrite needs its own case: the module has no stable part. Until the rule is named, a proposed rewrite is a Fit finding.
12. **Falsify before calling a step done.** State the step's claim so it can fail, then attack it: sweep the siblings of any defect's pattern in the same step; run a hostile review of the diff with its intent stated, then a review of each round of fixes, until a round closes its findings and introduces none; probe the reference generatively where one exists; assert stateful invariants in Debug builds, not in goldens. A finding is real only inside the step's stated contract: a reproduction outside it is out of gate and goes to Deferred or nowhere, never to another pass. Under a plan, the cap in `prompts/plan.md` ends the passes first.
13. **A green gate proves no regression, nothing more.** Say "no regression" when the gates pass; say "verified correct" only for a claim that survived Rule 12.
14. **No test runs through the real environment.** A test builds its own machine (temp root, home, registry and working directory) and pins every variable the program reads; nothing drives the program through its real adapters with the process's own environment.
15. **No step writes a real home.** Live runs, fixtures and probes use a scratch home and registry under a temp directory. A user's home takes only what the step's text names and the plan's own records (the step's mark, Decisions, Deferred, the log).
16. **A rule is a compressed intention.** When its literal reading produces work its author would refuse (cost without the benefit the rule was written for), the reading is wrong, not the work: name the intention in one sentence, apply it, and record the gap as feedback so the wording is fixed.

## Vocabulary

Use these terms exactly when designing or discussing a module's shape — don't substitute "component," "service," "boundary."

- **Seam** _(Michael Feathers)_ — a place where you can alter behaviour without editing in that place; the *location* at which a module's interface lives. Where to put the seam is its own design decision, distinct from what goes behind it.
- **Depth** — leverage at the interface: how much behaviour a caller (or test) can exercise per unit of interface they have to learn. Deep = small interface, lots of implementation behind it. Shallow = interface nearly as complex as the implementation — avoid.
- **The deletion test.** Imagine deleting the module. If complexity vanishes, it was a pass-through. If complexity reappears across N callers, it was earning its keep.
- **One adapter means a hypothetical seam. Two adapters means a real one.** Don't introduce a seam unless something actually varies across it.
- **Depth as leverage, not as a ratio.** Depth is leverage at the interface (behaviour per unit of interface learned) — not implementation-lines divided by interface-lines, which rewards padding the implementation.

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

**Post (after each step):** review the diff as Rule 12 says (under a plan, to the gate and cap in `prompts/plan.md`), against these points:

- **Fit** — does the solution match the problem's natural seams? Does it address essential complexity without adding accidental complexity?
- **Naming** — consistent, accurate, neither vague nor over-specific?
- **Size** — functions and modules readable without scrolling; split when responsibility has clearly diverged.
- **Seams** — logic lives at the right layer; nothing leaking across concerns.
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

- [`docs/praxis.md`](docs/praxis.md) — Extended guidance: how we work, before/during implementation, anti-patterns
