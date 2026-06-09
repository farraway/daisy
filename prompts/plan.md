## Trigger

Read the full `$DAISY_ROOT/prompts/plan.md` when:
- User invokes `/daisy plan`, `/daisy execute`, `/daisy archive`, or `/daisy resume`
- User says "Daisy, plan ...", "Daisy, execute ...", or "Daisy, archive ..."
- `PLAN.md` exists at the workspace root and any plan-related work is in progress

**Plan workflow:**
- `plan <description> [+project]` → create PLAN.md, enter collaborative planning mode
- `execute PLAN.md` → implement the plan step by step with praxis.md as quality guide
- `archive PLAN.md` → move completed plan to archive
- `resume PLAN.md` → verify state from a previous session, confirm resumption point, continue execution

---

## Rules

1. **Research before scaffolding.** Before filling in PLAN.md sections, read what already exists relevant to the task. Identify what's essential (inherent to the problem) vs. accidental (artifact of the current approach). Confirm the research summary with the user before scaffolding.
2. **No implementation during plan mode.** During `/daisy plan`, only edit `PLAN.md`; do not write any implementation code until the user confirms the plan is ready.
3. **Collaborate until confirmed.** End every planning response with the Open Questions list and "Ready to execute? (yes/no)"; do not proceed to execution without confirmation.
3. **Read praxis.md before each step.** During `/daisy execute`, read `$DAISY_ROOT/prompts/praxis.md` as the quality guide before implementing each step.
4. **Check off steps immediately.** After completing each step, mark it `- [x]` in `PLAN.md` before moving on.
5. **Stop and report when blocked.** Do not skip steps or work around blockers silently; surface them explicitly.
6. **Out-of-scope work goes to Deferred.** When a need surfaces outside the current step, add it to Deferred — do not implement it partially or silently drop it.
7. **Log before the session closes.** After completing all steps, log a summary to `today.md` via `log.sh` before ending the session — not deferred to the next one.
8. **Warn before archiving incomplete plans.** If `**Status:**` is not `built`, confirm with the user before archiving.
9. **Archive only applies to Daisy plans.** `/daisy archive` requires a `PLAN.md` symlink in the workspace. Plans created by Claude's `/plan` mode (stored in `~/.claude/plans/`) are not Daisy plans — there is nothing to archive.
10. **Calibrate process to task complexity.** Skip the plan workflow for simple, contained changes (a one-liner, a rename, a config tweak) — just do them directly. Use one research pass + plan for single-file features. Use full RPI with human checkpoints for cross-file or cross-repo work. Reserve multiple research iterations for complex refactors.
11. **Steps must name files; complex steps show code.** Every step should identify the specific files (and line ranges where known) that will change. For non-trivial changes, include a brief before/after code snippet. Vague steps ("refactor auth module") give the agent nothing to anchor on — and an error in the plan is an error in every line that follows from it.
12. **Surface repeated corrections; do not compound them.** If you have made the same mistake twice on a step, stop. Tell the user what you've tried and why it keeps failing; ask whether to compact what's been learned and start the step fresh. Do not continue correcting in-place — a history of corrections degrades trajectory and primes further failure.
13. **Resume by verifying state, not assuming it.** When picking up an in-progress plan from a previous session, do not assume completed steps are correct — verify. Check git log, today.md, and spot-check key files. Summarize what is confirmed, what is uncertain, and where work was interrupted. Confirm with the user before continuing.

# Daisy — Plan Workflow

## /daisy plan \<description\> [+project]

1. Call `$DAISY_ROOT/daisy/scripts/plan-new.sh "<description>" ["+project"]`
2. Enter plan mode — only edit `PLAN.md`; do not write implementation code.
3. **Research phase:** Read what already exists relevant to the task. Identify: (a) what already solves part of this, (b) what's essential to the problem (inherent) vs. accidental (artifact of current approach). Write findings into the `## Research` section of `PLAN.md`. Ask: **"Research summary confirmed — proceed to planning? (yes/no)"**
4. Scaffold plan sections from conversation context: fill in Goal, Non-goals, Constraints; leave Steps as placeholder checkboxes.
5. If a `+project` tag was given: update `.daisy/projects/{project}.md` — add a Plans section with a relative link and status `in progress`.
6. End every response with the numbered Open Questions list and: **"Ready to execute? (yes/no)"**

## /daisy execute PLAN.md

1. Read `PLAN.md`; confirm at least one unchecked `- [ ]` step exists.
2. Update `**Status:** executing` in the frontmatter block.
3. Before each step: read `$DAISY_ROOT/prompts/praxis.md`.
4. Work through steps in order; mark each `- [x]` in `PLAN.md` after completion.
5. After each step: update Decisions (if any) and Deferred (if scope surfaced).
6. Stop and report if blocked; do not skip steps or work around blockers silently.
7. On all steps complete: append ` *Built*` to the H1 title; update `**Status:** built`.
8. Log execution summary to `today.md` via `$DAISY_ROOT/daisy/scripts/log.sh`.

## /daisy archive PLAN.md

1. Warn if `**Status:**` is not `built` — confirm the user wants to archive anyway.
2. Call `$DAISY_ROOT/daisy/scripts/plan-archive.sh`
3. Log the archive event to `today.md` via `$DAISY_ROOT/daisy/scripts/log.sh`.

## /daisy resume PLAN.md

1. Read `PLAN.md` fully — note `**Status:**`, completed steps (`[x]`), and the first unchecked step (`[ ]`).
2. **Verify completed work:**
   - Run `git log --oneline -10` to see recent commits.
   - Check today.md for log entries from the previous session.
   - Spot-check 1–2 key files that completed steps claim to have changed — confirm the changes are present.
3. **Read `## Decisions`** in PLAN.md for context recorded by the previous session.
4. **Identify the interruption point:** Was the previous session stopped mid-step, between steps, or blocked?
5. **Surface gaps:** What context was established during the previous session that is not recorded in PLAN.md?
6. Present a resumption summary to the user:
   - **Completed and verified:** steps done and confirmed in the workspace
   - **Uncertain:** steps marked done but unverified, or partially done
   - **Next:** the step to resume from, and any known sub-state
   - **Unknown:** context that cannot be reconstructed from artifacts
7. Ask: **"Ready to resume from Step N? (yes/no)"** — do not proceed without confirmation.
8. On confirmation: read `$DAISY_ROOT/prompts/praxis.md`, update `**Status:** executing`, then continue from the identified step.
