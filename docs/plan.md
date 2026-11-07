# Plan Workflow — Reference

Detailed algorithms for the `/daisy plan`, `/daisy execute`, and `/daisy archive` workflows. For the AI agent's operational rules, see `prompts/plan.md`.

## Overview

Plans are durable implementation artifacts. A plan is developed collaboratively before any code is written, executed one step per fresh sub-agent with `praxis.md` as a quality guide, and archived when complete. The plan file is the contract between planning and execution.

---

## /daisy plan \<description\> [+project]

```
1. Call daisy plan new "<description>" ["+project"]

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

6. Ask which model and effort level the step agents and the
   review agents use; record the answer in Decisions.

7. Present the numbered Open Questions list via AskUserQuestion
   (the client's native Q&A UI), including a final question
   equivalent to "Ready to execute?"; fall back to prose ending in
   "Ready to execute? (yes/no)" only when AskUserQuestion is
   unavailable.
```

Collaborate on the plan until the user confirms it is ready. Use the plan phase to surface risks, open questions, and scope boundaries — especially distinguishing essential from accidental complexity before any implementation begins. Do not implement anything.

---

## /daisy execute PLAN.md

```
0. Run `daisy files` and use the resolved "current plan" real path
   for every write below — PLAN.md at the workspace root is a
   symlink, and writing through it fails with "Refusing to write
   through symlink".

1. Read PLAN.md; confirm at least one unchecked "- [ ]" step exists.

2. Update "**Status:** executing" in the frontmatter block.

3. Work through steps in order, each in a fresh sub-agent (not
   a fork). The main agent orchestrates and does not implement.
   Brief the step agent with:
   - the plan's real path and the step's text
   - $DAISY_ROOT/prompts/praxis.md and the testing skill
   - the review gate and its cap
   - the hand-back's shape
   - its reviewer's model and effort (from Decisions)

4. The step agent reads praxis.md, states the step's claim,
   does the step, reviews it to the gate, commits and checks off
   the step (neither when it stops at the cap), and updates
   PLAN.md:
   - Record decisions or deviations in Decisions if any
   - Move anything out of scope to Deferred

5. The orchestrator verifies the hand-back (claim, tests,
   daisy healthcheck --force, hygiene, commits, plan marks)
   before launching the next step. It does not ask the user
   between steps.

6. Stop and report at the stops below, or if blocked; do not
   skip steps or work around blockers silently.

7. On all steps complete:
   - Append " *Built*" to the H1 title
   - Update "**Status:** built"

8. Log execution summary to today.md via:
   daisy log
```

**Review gate and cap:** the first pass reviews the diff; each later pass
reviews only the fixes (the previous findings, the diff of the fixes
alone and the files those hunks touch), and a finding in text no fix
touched is Deferred after pass 1. A review finding is in gate only when
the reviewer reproduces it and it falls in one of four classes: the claim
fails, a required item is missing or wrong, a test is red, a document
contradicts another. The brief names these classes and adds none, and
never adds the last pass's finding types. A green test is not red (a
missing test is a Deferred candidate unless the step's text names it); a
contradiction counts only when the diff introduced it (compare with `git
show HEAD:<file>`); a design hole two passages leave open is an Open
Question. Style and anything outside the step's stated contract are out
of gate: Deferred or nowhere, never another pass. So is a finding that
needs an arrangement the project's own contract does not support (in
Daisy's root, a user-made link into it). The step agent classifies each
finding before fixing anything (in gate, disputed with a one-sentence
reason and not fixed, or out of gate) and fixes only the in-gate ones. The
cap is 5 passes for every step, code or docs; a step not clean after pass
5 stops: the step agent hands back with the open findings and its
proposed fixes uncommitted, and the orchestrator takes them to the user.
The orchestrator does not commit at the cap and never runs another pass.

**Stops:** the plan's own checkpoints; a hard-to-reverse or outward-facing
action the plan does not itself call for; a step the plan gives the user;
a step stopped at the review cap; a permission refusal, which the
orchestrator reports verbatim and never re-runs for the sub-agent.

**Hand-back:** the claim and its probes; lines added and deleted; each
review pass with its in-gate count; findings disputed with the reason;
findings accepted as is; at the cap,
the open findings and the proposed fixes left uncommitted; the
verification lines; the commits; the decisions; the tokens used.

**Fixture isolation:** if a step live-tests a script that auto-commits (or
otherwise mutates) a home, never run it against a real, in-use home.
Use `daisy test`'s hermetic fixture, or a scratch home, registry and
workspace under a temp directory (`DAISY_CONFIG` and `DAISY_LOCAL` pointed
there before `daisy home new <fixture-home>`, and `daisy init <fixture-home>`
run in a scratch workspace), and tear it down afterward.

---

## /daisy archive PLAN.md

```
1. Warn if "**Status:**" is not "built" — confirm the user
   wants to archive anyway.

2. Call daisy plan archive

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
  Claim: one sentence the reviewer can refute by a command, a test name or two quoted passages
- [ ] Step 2: Description
  Claim: ...

## Decisions

(recorded during execution)

## Deferred

(scope discovered during execution, not part of this plan)
```
