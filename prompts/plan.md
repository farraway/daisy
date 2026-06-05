## Trigger

Read the full `$DAISY_ROOT/prompts/plan.md` when:
- User invokes `/daisy plan`, `/daisy execute`, or `/daisy archive`
- User says "Daisy, plan ...", "Daisy, execute ...", or "Daisy, archive ..."
- `PLAN.md` exists at the workspace root and any plan-related work is in progress

**Plan workflow:**
- `plan <description> [+project]` → create PLAN.md, enter collaborative planning mode
- `execute PLAN.md` → implement the plan step by step with praxis.md as quality guide
- `archive PLAN.md` → move completed plan to archive

---

# Daisy — Plan Workflow

Plans are durable implementation artifacts. A plan is developed collaboratively before any code is written, executed step by step with `praxis.md` as a quality guide, and archived when complete. The plan file is the contract between planning and execution.

## /daisy plan \<description\> [+project]

1. Call `$DAISY_ROOT/daisy/scripts/plan-new.sh "<description>" ["+project"]`
2. Enter plan mode — you may only edit `PLAN.md` during this phase; do not write implementation code.
3. Scaffold the plan sections from conversation context: fill in Goal, Non-goals, Constraints with what is known; leave Steps as placeholder checkboxes for collaborative development.
4. If a `+project` tag was given: update `.daisy/projects/{project}.md` — add a Plans section (or append to it) with a relative link `../plans/{filename}` and status `in progress`.
5. End every response with the numbered Open Questions list and: **"Ready to execute? (yes/no)"**

Collaborate on the plan until the user confirms it is ready. Do not implement anything. Use the plan phase to surface risks, open questions, and scope boundaries.

## /daisy execute PLAN.md

1. Read `PLAN.md`; confirm at least one unchecked `- [ ]` step exists.
2. Update `**Status:** executing` in the frontmatter block.
3. Before each step: read `$DAISY_ROOT/prompts/praxis.md` (as instructed by the bold line in the Steps section).
4. Work through steps in order. After completing each step, mark it `- [x]` in `PLAN.md`.
5. After each step: update PLAN.md — check off the step; record decisions or deviations in Decisions if any; move anything out of scope to Deferred.
6. Stop and report if blocked; do not skip steps or work around blockers silently.
7. On all steps complete:
   - Append ` *Built*` to the H1 title
   - Update `**Status:** built`
8. Log execution summary to `today.md` via `$DAISY_ROOT/daisy/scripts/log.sh`.

## /daisy archive PLAN.md

1. Warn if `**Status:**` is not `built` — confirm the user wants to archive anyway.
2. Call `$DAISY_ROOT/daisy/scripts/plan-archive.sh`
3. Log the archive event to `today.md` via `$DAISY_ROOT/daisy/scripts/log.sh`.
