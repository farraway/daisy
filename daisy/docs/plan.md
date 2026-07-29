# Plan Workflow — Reference

Detailed algorithms for the `/daisy plan`, `/daisy execute`, and `/daisy archive` workflows. For the AI agent's operational rules, see `prompts/plan.md`.

## Overview

Plans are durable implementation artifacts. A plan is developed collaboratively before any code is written, executed step by step with `praxis.md` as a quality guide, and archived when complete. The plan file is the contract between planning and execution.

---

## /daisy plan \<description\> [+project]

```
1. Call daisy plan-new "<description>" ["+project"]

2. Enter plan mode — only edit PLAN.md during this phase;
   do not write implementation code.

3. Research phase: read what already exists relevant to the task.
   Identify:
   - What already solves part of this?
   - What's essential to the problem (inherent to what needs solving)?
   - What's accidental (an artifact of how it's currently done)?
   Write findings into the ## Research section of PLAN.md.
   Ask: "Research summary confirmed — proceed to planning? (yes/no)"

4. Scaffold plan sections from conversation context:
   - Fill in Goal, Non-goals, Constraints with what is known
   - Leave Steps as placeholder checkboxes for collaborative development

5. If a +project tag was given:
   - Update .daisy/projects/{project}.md
   - Add a Plans section (or append to it) with:
     ../plans/{filename} — status: in progress

6. Present the numbered Open Questions list via AskUserQuestion
   (the client's native Q&A UI), including a final question
   equivalent to "Ready to execute?"; fall back to prose ending in
   "Ready to execute? (yes/no)" only when AskUserQuestion is
   unavailable.
```

Collaborate on the plan until the user confirms it is ready. Use the plan phase to surface risks, open questions, and scope boundaries — especially distinguishing essential from accidental complexity before any implementation begins. Do not implement anything.

---

## /daisy execute PLAN.md

```
0. Run `daisy files` and use the resolved "active plan" real path
   for every write below — PLAN.md at the workspace root is a
   symlink, and writing through it fails with "Refusing to write
   through symlink".

1. Read PLAN.md; confirm at least one unchecked "- [ ]" step exists.

2. Update "**Status:** executing" in the frontmatter block.

3. Before each step: read $DAISY_ROOT/prompts/praxis.md
   (as instructed by the bold line in the Steps section).

4. Work through steps in order.
   After completing each step, mark it "- [x]" in PLAN.md.

5. After each step, update PLAN.md:
   - Check off the step
   - Record decisions or deviations in Decisions if any
   - Move anything out of scope to Deferred

6. Stop and report if blocked; do not skip steps or work
   around blockers silently.

7. On all steps complete:
   - Append " *Built*" to the H1 title
   - Update "**Status:** built"

8. Log execution summary to today.md via:
   daisy log
```

**Fixture isolation:** if a step live-tests a script that auto-commits (or
otherwise mutates) a home, never run it against a real, in-use home.
Use `daisy test`'s hermetic fixture, or an isolated
`daisy init --new <fixture-home>`, and tear it down afterward.

---

## /daisy archive PLAN.md

```
1. Warn if "**Status:**" is not "built" — confirm the user
   wants to archive anyway.

2. Call daisy plan-archive

3. Log the archive event to today.md via:
   daisy log
```

**Note:** `/daisy archive` only applies to plans created via `/daisy plan` — it requires a `PLAN.md` symlink in the workspace. Plans created by Claude's `/plan` mode are stored in `~/.claude/plans/` and are not Daisy plans; there is nothing to archive.

---

## Plan File Format

Plans live in `.daisy/plans/` and use this structure:

```markdown
# Plan Title

**Status:** planning | executing | built
**Project:** +project-name (optional)
**Created:** YYYY-MM-DD

## Goal

One paragraph describing what this plan achieves.

## Non-goals

- What this plan explicitly does not cover

## Constraints

- Technical or process constraints that shape the approach

## Steps

> Read $DAISY_ROOT/prompts/praxis.md before each step.

- [ ] Step 1: Description
- [ ] Step 2: Description

## Decisions

(recorded during execution)

## Deferred

(scope discovered during execution, not part of this plan)
```
