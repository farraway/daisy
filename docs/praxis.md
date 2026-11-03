# Praxis — Implementation Quality Reference

Full elaboration of the principles in `prompts/praxis.md`. The prompt contains the operational core (Rules + Review checklists). This file is the "why" and extended guidance.

---

## How We Work

Work in short cycles with the human in the loop (under `/daisy execute`, the orchestrating agent stands in the loop between steps, and the user is asked only at the stops in `prompts/plan.md`):

1. **Understand** — read what exists; infer intent from code, not assumptions.
2. **Propose** — explain what you intend to do and why, before large changes.
3. **Implement** — execute the chosen design with the smallest diff: no extras, no speculative generality. The smallest diff never decides the size of a design change.
4. **Review** — check the result against the standards below.
5. **Discuss** — present what changed and why; adjust from feedback.
6. **Stop** — scope is relatedness, not size. A need unrelated to the step's problem goes to Open Questions or Deferred — do not implement it. A rearchitecture whose cause is the step's own problem is in scope: propose it first.

Prefer one good step over a batch of unreviewed steps.

---

## Before This Step

- What is the actual problem? Distinguish symptoms from causes.
- What already solves part of it? Read what exists before inventing.
- **Consult a paragon.** Find one or two well-regarded implementations of the same concept in the workspace's reference codebases. Read them — not to copy, but to know what the problem looks like when it has been solved well.
- Where do responsibilities belong? Sketch module or layer boundaries first.
- What is explicitly out of scope for this step? What should be deferred?
- For non-trivial work: state the approach in plain language before coding.

---

## While Implementing

- **Minimize scope** — smallest diff within the chosen design; no drive-by refactors.
- **Match conventions** — naming, layout, error style, patterns already in use.
- **Resist over-engineering** — no premature generalisation, no abstraction layers with only one caller, no helpers that merely rename one line.
- **Comments sparingly** — code should mostly explain itself; comment only non-obvious constraints, invariants, or trade-offs.
- **Tests when they earn their keep** — add tests that protect real behaviour or document contracts; skip tests that only assert the obvious.

---

## Maintaining the Plan

If working from a PLAN.md:

- Check off the completed step.
- If you deviated from the plan, record why in the Decisions section.
- If something cannot be done correctly within this step's scope, move it to Deferred — do not implement it partially or silently drop it.
- If a new question arose, add it to Open Questions.

---

## What Good Output Feels Like

- A reader thinks: "yes, of course it works that way."
- Removing the change would leave an obvious gap; adding it does not create parallel concepts doing the same job.
- The next similar problem has a clear place to go — because the structure reflects the domain, not the session's convenience.

---

## Anti-Patterns

- Generic frameworks inside a specific feature.
- Stubs or placeholders presented as done.
- Errors swallowed, logged-only, or returned without context.
- "While I'm here" refactors unrelated to the task.
- Abstractions named after patterns (Manager, Handler, Util) that do not correspond to a real responsibility.
- Implementing three steps ahead of the agreed one.
- Partial implementation of something that should have been deferred.
