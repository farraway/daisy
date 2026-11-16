# Praxis — Implementation Quality Reference

Full elaboration of the principles in `prompts/praxis.md`. The prompt contains the operational core (Rules + Review checklists). This file is the "why" and extended guidance.

---

## How We Work

Work in short cycles with the human in the loop (under `/daisy execute`, the orchestrating agent stands in the loop between steps, and the user is asked only when the run needs something only they can give that the plan does not record, and at the plan's own checkpoints (`prompts/plan.md` rule 24)):

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
- **Comments sparingly** — code should mostly explain itself; comment only non-obvious constraints, invariants, or trade-offs. A comment never argues for a workaround: a shortcut it justifies leaves its cause in place, unseen by the next reader. Fix the cause, or, when the cause lies outside the step's problem, the cause goes to Deferred (Maintaining the Plan).
- **Tests when they earn their keep** — add tests that protect real behaviour or document contracts; skip tests that only assert the obvious.

---

## Maintaining the Plan

If working from a PLAN.md (under `/daisy execute` or `/daisy resume`, the step agent hands back its deviations, Deferred candidates and questions, and the orchestrator checks off the step and writes them; `prompts/plan.md` rule 20):

- Check off the completed step.
- If you deviated from the plan, record why in the Decisions section.
- If something cannot be done correctly within this step's scope, move it to Deferred — do not implement it partially or silently drop it.
- If a new question arose, add it to Open Questions.

---

## Rules and Who Makes Them

A rule is a compressed intention: it protects something in fewer words than the intention behind it, so a literal reading can produce work its author would refuse, cost without the benefit the rule was written for. Then the reading is wrong, not the work: name the intention in one sentence, apply it, and record the gap as feedback so the wording is fixed (in Daisy, `daisy feedback`).

The intention is the user's, and so is the making of a new rule. A rule for how the work runs (a limit, a cap, a stop, a gate) binds every later step, and one an agent writes from a lesson of its own carries an inferred figure with the authority of a stated one. On 2026-11-01, after a step ran 20 review passes, a session wrote two such rules into its own project memory, review caps of 6 and 3 passes and a proportion stop at about 300 changed lines, a figure the model chose; a later step, approved to fold that memory into the plan prompt, did so, and the user saw the rules only as one clause of a long decision summary; the stop halted three steps before the user, not recognising it, dropped it (the caps and the removal of the proportion rule: `docs/plan.md`, The review gate). So an agent that thinks a lesson calls for such a rule puts it to the user as a separate question, asked alone, with the figure and where it came from (measured, taken from a source, or its own estimate), and records the rule where later work reads instructions (memory, a prompt, a plan) only once the user confirms it; an agent that cannot ask the user, such as a step agent, hands it back marked as a proposed rule. When memory or other guidance the user has not confirmed moves into a prompt or instruction file, each rule in it is listed for the user to confirm before it lands, not folded into a summary, where a rule reads as a detail of the move.

The approval draws the line, not the file. A rule is approved when the user approved it as such: in a step's text, or in answer to a question that put it, with its figure where it has one. Approving a move or a summary is not approving the rules it carries, so the fold above needed each rule put to the user. Wording, folding or applying a rule the user approved is the step's work; a limit, a cap, a stop or a gate that no such approval names is a new rule, whichever prompt it lands in.

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
