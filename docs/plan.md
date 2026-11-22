# Plan Workflow — Reference

Detailed algorithms for the `/daisy plan`, `/daisy execute`, and `/daisy archive` workflows. For the AI agent's operational rules, see `prompts/plan.md`.

## Overview

Plans are durable implementation artifacts. A plan is developed collaboratively before any code is written, executed in batches of steps, one fresh sub-agent per batch, with `praxis.md` as a quality guide, and archived when complete. The plan file is the contract between planning and execution.

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
   review agents use, recommending a Sonnet-class model for the
   reviewers; record the answer in Decisions.

7. Present the numbered Open Questions list via AskUserQuestion
   (the client's native Q&A UI), including a final question
   equivalent to "Ready to execute?"; fall back to prose ending in
   "Ready to execute? (yes/no)" only when AskUserQuestion is
   unavailable.
```

Collaborate on the plan until the user confirms it is ready. Use the plan phase to surface risks, open questions, and scope boundaries — especially distinguishing essential from accidental complexity before any implementation begins. Do not implement anything.

### Models and step size

`prompts/plan.md` rules 21 and 13 state the two choices; this is why they
read as they do.

**A Sonnet-class reviewer by default (rule 21).** Plan 261111115545 ran
its reviewers on Opus 5.5 at high effort and plan 261114125457 on Sonnet
5.5 at high effort. The second spent 62% fewer review tokens (2.24M
against 5.88M) for more code (4,839 committed code lines against 2,702),
about 2.5 times the code per token; none of its steps took more than 2
review passes, where seven of the first plan's took 3 to 5, and its
reviewers still caught real defects (design review of 2026-11-15,
effectiveness G2 and R2). The metrics show no loss; the trade-off is a
subtle defect a weaker reviewer may miss, which would show in later fixes.
So the rule recommends a Sonnet-class model and records another when
Decisions names a reason (a claim that is an invariant across packages,
or simply the user's choice). It names the class, not a version, so it
does not date.

**One claim, about one screen of diff (rule 13).** Plan 261114125457's
steps averaged about 230 code and 210 test lines and none passed 2 review
passes; plan 261111115545's two largest, Step 16a (+827) and Step 16b
(+544), took 5 and 1 (effectiveness R10). The size is a claim and a
screen, with no file count (the maintainer's choice, 2026-11-15): a rename across many files
is still one claim. The "and" that splits a step is the one joining two
changes, two things the step does; an "and" between checks of one change
does not, and the testing skill's two model claims ("Cases 54 and 55
pass"; "`init` then `clean` leaves `git status` and the three config keys
as before") are each one claim.

---

## /daisy execute PLAN.md

```
0. Resolve the plan's real path with `daisy files plan` and write
   every change below there (prompts/plan.md rule 17: PLAN.md is a
   link, and prompts/daisy.md's Trigger block says why no Daisy file
   is reached through one). Every write to the plan is the
   orchestrator's; a step agent never writes it.

1. Read PLAN.md; confirm at least one unchecked "- [ ]" step exists,
   and that Decisions records the model and effort the step agents
   and the review agents use (ask it if not, recommending a
   Sonnet-class model for the reviewers).

2. Update "**Status:** executing" in the frontmatter block.

3. Group the steps, in order, into batches, and work through the
   batches, each in a fresh sub-agent, the step agent (not a fork).
   The main agent orchestrates and does not implement. Brief the
   step agent with the batch's steps written in, not the plan's
   path:
   - the plan's Goal in a sentence
   - the Non-goals
   - each step's text
   - the Decisions that name a step or bear on it
   - the Constraints
   - $DAISY_ROOT/prompts/praxis.md and the testing skill
   - the review and the gate
   - the hand-back's shape
   - its reviewers' model and effort (from Decisions)
   - that it neither reads nor writes the plan file, and leaves
     the journal to the orchestrator

4. The step agent, which neither reads nor writes the plan
   file nor writes the journal, reads praxis.md and, for each
   step in order, states its claim, does the step (its test red
   first, then its probes) and commits it after a green cached
   gate (none for a step it stops on). It then runs the batch's
   review, runs the gate uncached and hands back, with, for each
   step:
   - the decisions it made, each with the alternative considered
   - its Deferred candidates (scope that surfaced)
   - its open questions, and any rule it proposes, marked as a
     proposed rule

5. The orchestrator verifies the hand-back (each claim, the
   gate uncached, daisy healthcheck --force, hygiene, commits;
   for a hand-back stopped by the review or at another stop,
   that nothing past the stop is committed), then writes the
   plan at its real path:
   - marks each step "- [x]" (not a step the batch stopped on)
   - records the decisions in Decisions
   - adds the Deferred candidates to Deferred, with the review's
     leftovers that do not change behaviour and each out-of-gate
     remark the hand-back reports that names a defect at a file
     and line
   - adds the open questions to Open Questions, a proposed rule
     marked "Rule question", to be asked alone at the next stop
     or at the run's end (it is not itself a stop)
   It records Decisions and Deferred for a stopped batch too.
   It then records the metrics with daisy metrics add: one
   --role step row per step (the step's commit range and
   repository, the step agent's model, effort and tokens), and
   one --role review row per batch for each reviewer model and
   effort its passes used (--step the batch's steps joined by
   "+", that model and effort, --review-rounds, --review-effect
   the in-gate findings fixed, the passes' tokens summed; no
   range, so no lines). A step the batch stopped on keeps the
   same two rows as a finished step, its step row (no range when
   it has no commit) and its batch's review row, each with
   --outcome "stopped: <stop>" ("stopped: review" at the
   review's stop). Last it logs each step via daisy log, before
   launching the next batch. It does not ask the user between
   batches.

6. Stop and report at the stops below, or if blocked; do not
   skip steps or work around blockers silently. At the run's
   end, record one --role orchestrator row (--step the last step
   the run verified), with the orchestrator's own tokens when it
   can count them, else empty.

7. On all steps complete:
   - Append " *Built*" to the H1 title
   - Update "**Status:** built"

8. Log execution summary to today.md via:
   daisy log
```

**Batches, review and gate:** `prompts/plan.md` rule 22 is the batches,
the claim and nothing beyond, the gate, the review's two passes and its
stop; [The review gate](#the-review-gate) below gives the reasons and the
history.

**Stops:** the run stops when it needs something only the user can give
and the plan does not record: a decision of theirs (the model and effort
of the step and review agents when Decisions has none; a rearchitecture
that crosses a decision they made), an approval (a hard-to-reverse or
outward-facing action the plan does not call for, such as a force-push,
a deletion or a publication; a step the plan gives the user), or a way
past a block (a batch stopped by its review; the same mistake twice; a
permission refusal, which the orchestrator reports verbatim and never
re-runs for the sub-agent). The plan's own checkpoints are stops too. The
orchestrator logs each stop with its reason (`prompts/plan.md` rule 9).
A step stopped at one (the same mistake twice; a rearchitecture that
crosses a decision) is not committed: the step agent hands back with the
stop and its reason named and the step's work uncommitted, and its
metrics rows carry `--outcome "stopped: <stop>"`.

**Hand-back:** for each step, the claim and its probes; lines added and
deleted; its commit; the decisions made, each with the alternative
considered; the Deferred candidates and the open questions, a proposed
rule marked as one, which the orchestrator writes into the plan. For the
batch, each review pass with its findings, each in-gate one with its fix
and the fix's commit; the out-of-gate remarks, each with its file and
line; the leftovers bound for Deferred; at a stop, the stop and its
reason, the step or open finding it stopped on, and the proposed fix left
uncommitted; the verification lines; the batch's commit range and its
repository; the tokens used, the step agent's and each reviewer's, each
with its model and effort.

**The brief, not the plan.** Until 2026-11-15 each step agent was given
the plan's real path and read the whole plan (90–104 KB, about 25k
tokens, every step and every past decision), then wrote its own mark,
Decisions and Deferred there, which made rule 17's symlink trap one of
its duties (design review of 2026-11-15, effectiveness R3, P20). Now the
orchestrator writes the batch's steps into the brief: the plan's Goal in
a sentence, the Non-goals, each step's text, the Decisions that name a
step or bear on it, and the Constraints. Rule 13 already makes the step's text the
contract (files named, code shown, a claim), so that is what the agent
needs. Everything the agent would have written into the plan comes back
in the hand-back, and the orchestrator writes it after verifying the
step (the maintainer's choice, 2026-11-15). The cost is the plan's
history: a Decision the orchestrator does not judge to bear on the step
is one the step agent does not see.

**Metrics:** `daisy metrics add` appends each row to
`<home>/metrics/metrics.csv`, the one record across plans, and writes
the same row into the plan's `## Metrics` table. The lines added and
removed per class (code, tests, prose) are measured from the step's
commit range by git, never reported by an agent. `daisy metrics` summarizes the CSV by kind and model: the
record is there to show what a change of documentation, model or
strategy does to a step's cost and shape.

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

2. Rehome the plan's Deferred list:
   - Verify each item against the code
   - Live: move it to the project file's ## Deferred
   - Resolved (the plan's, or the project file's that the plan
     closed): strike through, date, and move it, its text kept,
     to .daisy/projects/_deferred-closed/<project>.md under a
     new heading at the top naming the date and the plan by its
     id (its file name's timestamp, not a link: daisy plan
     archive repoints links in projects/ and projects/_archive/
     only)

3. Call daisy plan archive

4. Log the archive event to today.md via:
   daisy log
```

**Why closed items leave the project file.** Until 2026-11-15 a resolved
item was struck and kept in place, and Daisy's own project file's Deferred
section reached 561 bullets, 417 of them struck, which every research
phase and every triage read again (design review of 2026-11-15, P23).
The project file now holds the open work only. The closed items go to one
file per project, not one per archive, so a single path answers "was this
done?"; the newest heading sits at the top, where that question usually
lands. The file lives in a directory, `_deferred-closed/` beside
`_archive/` in the home's `projects/`, because `daisy projects` lists every
`*.md` at the top of `projects/` as an active project.

**Note:** `/daisy archive` only applies to plans created via `/daisy plan` — it requires a `PLAN.md` symlink in the workspace. Plans created by Claude's `/plan` mode are stored in `~/.claude/plans/` and are not Daisy plans; there is nothing to archive.

---

## The review gate

`prompts/plan.md` rule 22 is how a batch of steps is built, gated and
reviewed. This section holds why each part of it reads as it does and
where it came from, so the rule can stay short; where the two differ, the
rule wins. Most of it comes from an audit of the review loop on
2026-11-06 (one session's three step agents and twelve reviewers), from
the design review of 2026-11-15 and from the maintainer's change of
2026-11-21.

**Batches, the claim and nothing beyond, fix then check.** Until
2026-11-21 each step ran in its own agent and was reviewed alone, up to
five passes, the step agent fixing each pass's findings and a fresh
reviewer reading each round of fixes. Two steps of one plan each
used all five passes, the late passes finding test gaps and wording, not
behaviour, and a step took about an hour and a half. So the maintainer
chose a shorter loop. One step agent builds a batch of steps, one commit
per step. A step builds what its claim names and nothing beyond: an edge
case the claim does not name is a Deferred candidate, not code, and where
the spec is silent the simplest reading consistent with it is taken and
the alternative named as an open question. The batch is reviewed once:
the first pass reviews and fixes, committing its fixes; a fresh second
pass checks only those fix commits and starts no new hunt. Then the
review closes, and what is left goes to Deferred unless it changes
behaviour (a claim fails, a test is red). A behaviour finding earns one
more fix and one more check, and then the batch stops for the user,
since a fix that keeps failing is the signal of a seam patched twice
(praxis rule 11). The gate runs cached between commits and uncached once
per batch, before the hand-back, and the orchestrator runs it uncached
again: one uncached run per batch keeps a stale test cache from hiding a
red or a slow package, which cached runs had once done for a package that
took 604 s uncached, without paying an uncached run per commit.

**A pass tests the claim.** A review pass is an attempt to refute the
steps' `Claim:` lines (rule 13; the testing skill, The claim), not a search
for whatever can be found in the diff. The four classes are the ways a
claim fails to hold: it fails outright, a required item is missing or
wrong, a test is red, a document contradicts another. From 2026-11-15 to
2026-11-21 the claim also set the number of passes: a prose-only claim,
one whose diff is documents, prompts or plan text, has only two tests, a
document contradicts another and a required item is missing or wrong, so
one pass ran both and its fixes were committed without a second, while a
code claim's fixes went to another pass, up to the cap. The batch's two
passes replaced that for every claim. Before it a prose step paid 4 to
5 passes: in plan 261111115545, Step 25 (prose +26/-15) took 5 passes and
306,203 review tokens and Step 18 (prose +155/-80) 4 passes and 341,000,
and the late passes were the reviewer and the step agent negotiating
wording, the loop the cap exists to stop (design review of 2026-11-15,
effectiveness F1).

**The reproduction.** A finding counts only when the reviewer quotes the
code or passage and shows how it fails the step's text or a rule. Without
that, a judgement call is a finding: in the audited session a reviewer's "This is a judgement call, but it is a new field by the
gate's own definition" and another's finding marked "(low)" were both
fixed as in gate. The testing skill's Review says the same from the
reviewer's side: a finding it would not stake the commit on is out of
gate.

**Later passes review only the fixes.** Until 2026-11-06 every pass was
handed the whole diff. A fresh reviewer has no memory of what the last
one read, so each pass drained a different part of a large text: of
one step's 21 later findings that the audit traced, 10 cited text that was
in the first diff and that earlier passes had not read closely and 7 cited
text a fix had added, while the reviewers worked as hard at pass 5 as at
pass 1 (30 to 49 tool calls and 169k to 217k tokens of context each). The
count tracked the text's size, not the fixes. So pass 1 reviews the diff,
and each later pass gets the previous findings, the diff of the fixes alone
(`git diff` between the tree the last reviewer saw and the tree now) and
the files those hunks touch; it reports whether each finding is closed
and whether a fix introduced an in-gate defect in the lines it changed,
and a finding in text no fix touched is out of gate after pass 1. Praxis
rule 12's "and again over the fixes" read either way and was rewritten
with it. A batch's second pass keeps this: it reads only the first pass's
fix commits.

**Four classes, and none added.** The gate widened in every brief, and
the widening was cumulative. "A test is red" became "or a test cannot
fail: show the mutation", which with the testing skill's probe rule made
every untested line in gate: 6 of one step's 16 in-gate findings, including
both that held it at the cap, which the reviewer said did not break the
claim. Another brief named six classes for a 355-line change to a
2,650-line spec, and the step agent's later briefs added each pass's
finding types to the next one's hunt list (its brief grew from 7,796 to
10,133 characters). So every brief names the same four and adds none, not
even a past finding's type. A green test is not red; a test the reviewer
thinks is missing is in gate only when the step's text names it, as a
required item.

**Contradictions the diff introduced.** Briefs that admitted a
contradiction "introduced or left by the diff" let pre-existing
inconsistencies in, and the reviewers said so of their own findings ("The
numbering and the skip rule are pre-existing"). Earlier briefs that said
"do not report as in gate a gap that existed equally before this change"
converged (8, 6, 6, 3, 1 findings). So a contradiction is in gate only
when the diff introduced it: a passage the diff did not touch, or an
inconsistency that holds equally at the range's base, is out of gate
(`git show <base>:<file>` shows which). A hole two passages leave open in a design is
not a contradiction: a reviewer building a scenario the text does not
cover is reviewing the design by simulation, which is valuable and has no
end, so the hole goes to Open Questions, where the user decides it.

**Classify before fixing.** Whoever fixes judges the gate. With no way to
dispute, a step agent disputed none of 46 in-gate findings across two
steps, and fixed out-of-gate ones as well; each such fix was new text the
next reviewer had not asked for. So the step agent classified every
finding before fixing any: in gate (a named class, the reproduction
holds), disputed (the reproduction does not hold or the class is not
named: one sentence why, not fixed, reported in the hand-back) or out of
gate, and fixed only the in-gate ones. Since 2026-11-21 the first pass
fixes what it finds, so it classifies its own findings, in gate or out
of gate, before fixing any, and fixes only the in-gate ones.

**Arrangements the contract does not support.** Rule 22's first version
(2026-11-02) said Daisy's root/home contract is about paths, not the file
system: a write into the root counts only when reached through Daisy's own
resolution of home, workspace and root paths with no user-made link in
the way. A link a user makes into the root is the user's, and a write
through it is lost at the next root update. A reviewer inventing such
arrangements is the loop the cap exists to stop. The 2026-11-06 rewrite
stated it for any project: a finding that needs an arrangement the
project's own contract does not support is out of gate, with Daisy's root
as the example.

**Out-of-gate remarks go to the hand-back.** Until 2026-11-15 an
out-of-gate remark went "to Deferred or nowhere", and agents chose
Deferred: plan 261111115545 added 69 Deferred items (41 of them out of
gate) and plan 261114125457 92 in 21 steps; the project file's Deferred
section reached 561 bullets, 417 of them struck, and triaging it cost two
steps and 1.4M review tokens (design review of 2026-11-15, effectiveness
F2). Now the step agent reports the remarks in its hand-back, which the
orchestrator reads once; the orchestrator adds one to Deferred only when
it names a defect at a file and line (a test missing for a named case, a
wrong passage), and a style, wording or line-width remark ends there. The
cost is that some good remarks die; before, they died after the triage.
Since 2026-11-21 an in-gate leftover that does not change behaviour goes
to Deferred too, where it once took another pass.

**The cap, and the stop that replaced it.** The first cap (2026-11-02) was 6 passes for a step that
changes code and 3 for one that changes only documents and prompts, and
at the cap the orchestrator chose between committing with the open
findings in Deferred and stopping. On 2026-11-06 it became 5 passes for
every step, and a step not clean after pass 5 stops uncommitted: the step
agent hands back the open findings with its proposed fixes uncommitted,
and the orchestrator takes them to the user, does not commit at the cap
and never runs another pass; the same change removed the proportion rule.
On 2026-11-14 the stopped step was also left unmarked. The cap
exists because the search for a refutation has no natural end
(`docs/testing.md`, Part 4); with later passes scoped to the fixes,
reaching it means the fixes themselves keep failing, the signal of a seam
patched twice (praxis rule 11). The prose-only pass reached by the claim
what the first cap's 3 passes for documents reached by the kind of file.
On 2026-11-21 the batch's review replaced the cap: after its check, only
a behaviour finding still open earns one more fix and check, and if it is
still open the batch stops, the finding and its proposed fix going to the
user, its steps unmarked and no further pass run.

**History.**

| Date | Commit | Change |
|---|---|---|
| 2026-11-02 | 0c2c04a | Rule 22: a separate reviewer, the reproduction, the four classes, the root/home link contract, caps of 6 (code) and 3 (documents) |
| 2026-11-06 | 61bc5de | Cap of 5 for every step; a capped step hands back uncommitted; the proportion rule removed |
| 2026-11-06 | 78d46b1 | The review-loop audit applied: later passes review the fixes only; the four classes fixed, with the green-test, HEAD and design-hole exclusions; no class added from a past pass; classify and dispute; arrangements the contract does not support |
| 2026-11-14 | 166d0d0 | A step stopped at the cap is neither committed nor marked |
| 2026-11-15 | 5a93a91 | Rule 22 shortened, its reasons moved here; a pass tests the claim, so a prose-only claim takes one pass; out-of-gate remarks to the hand-back, kept in Deferred only when they name a defect at a file and line |
| 2026-11-21 | (this change) | Batches, one commit per step; the claim and nothing beyond; review and fix, then a check of the fix commits only; a leftover that does not change behaviour to Deferred, a behaviour finding one more fix and check, then a stop; the cap and the prose-only pass removed; the gate cached between commits, uncached once per batch |

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

- [ ] Step 1: Description
  Claim: one sentence the reviewer can refute by a command, a test name or two quoted passages
- [ ] Step 2: Description
  Claim: ...

## Decisions

(recorded during execution)

## Metrics

(a step row per step and a review row per batch, written by daisy metrics add)

## Deferred

(scope discovered during execution, not part of this plan)
```
