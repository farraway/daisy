## Trigger

Read the full `$DAISY_ROOT/prompts/plan.md` when:
- User invokes `/daisy plan`, `/daisy execute`, `/daisy archive`, `/daisy resume`, `/daisy plan spec`, or `/daisy plan pickup`
- User says "Daisy, plan ...", "Daisy, execute ...", "Daisy, archive ...", "Daisy, spec out ...", or "Daisy, pick up ..."
- `PLAN.md` exists at the workspace root and any plan-related work is in progress
- A `{slug}_PLAN.md` draft exists at the workspace root

**Plan workflow:**
- `plan <description> [+project]` → create PLAN.md, enter collaborative planning mode
- `plan spec <description> [+project]` → create an unregistered draft (`{slug}_PLAN.md`) directly at the workspace root — no `$DAISY_HOME/plans/` copy, no `PLAN.md` symlink. Multiple drafts can coexist while deciding which (if any) to pursue.
- `plan pickup <path>` → promote a local spec draft into a tracked plan: copies it into `$DAISY_HOME/plans/`, creates the `PLAN.md` symlink
- `execute PLAN.md` → implement the plan one step per fresh sub-agent, each reviewed and verified before the next
- `archive PLAN.md` → move completed plan to archive
- `resume PLAN.md` → verify state from a previous session, confirm resumption point, continue execution

---

## Rules

1. **Research before scaffolding.** Before filling in PLAN.md sections, read what already exists relevant to the task. Identify what's essential (inherent to the problem) vs. accidental (artifact of the current approach). Confirm the research summary with the user before scaffolding.
2. **No implementation during plan mode.** During `/daisy plan`, only edit `PLAN.md`; do not write any implementation code until the user confirms the plan is ready.
3. **Collaborate until confirmed.** Present the Open Questions list via `AskUserQuestion` (the client's native Q&A UI), including a final question equivalent to "Ready to execute?"; fall back to prose ending in "Ready to execute? (yes/no)" only when `AskUserQuestion` is unavailable. Precede the question with the plan summary required by rule 19. Do not proceed to execution without confirmation.
4. **Prefer spec-mode when the shape isn't settled yet.** Use `plan spec` instead of `plan` when exploring multiple candidate approaches before committing to one, or when it's not yet clear this deserves a tracked Daisy plan at all. Promote to a real plan with `plan pickup` once a direction is chosen.
5. **Read praxis.md before each step.** During `/daisy execute`, the step agent reads `$DAISY_ROOT/prompts/praxis.md` as the quality guide before implementing its step.
6. **Check off steps immediately.** The orchestrator marks each step `- [x]` in `PLAN.md` as soon as it has verified the step's hand-back (rule 23), before moving on; a step stopped at the review cap stays unmarked (rule 22).
7. **Stop and report when blocked.** Do not skip steps or work around blockers silently; surface them explicitly.
8. **Out-of-scope work goes to Deferred.** When a need surfaces outside the current step, add it to Deferred — do not implement it partially or silently drop it. A step agent names it in its hand-back as a Deferred candidate and the orchestrator adds it (rule 20). A review's out-of-gate remark is not a candidate: it goes to the hand-back too, and the orchestrator adds one to Deferred only when it names a defect at a file and line (rule 22).
9. **Log each verified step, and before the session closes.** `prompts/daisy.md` rule 6 says when to log; under `/daisy execute` and `/daisy resume` the orchestrator logs, via `daisy log`, each step as soon as it has verified the step's hand-back (rule 23), each stop of rule 24 with its reason, and, after the last step, a summary.
10. **Warn before archiving incomplete plans.** If `**Status:**` is not `built`, confirm with the user before archiving.
11. **Archive only applies to Daisy plans.** `/daisy archive` requires a `PLAN.md` symlink in the workspace. Plans created by Claude's `/plan` mode (stored in `~/.claude/plans/`) are not Daisy plans — there is nothing to archive.
12. **Calibrate process to task complexity.** Skip the plan workflow for simple, contained changes (a one-liner, a rename, a config tweak) — just do them directly. Use one research pass + plan for single-file features. Use full RPI with human checkpoints for cross-file or cross-repo work. Reserve multiple research iterations for complex refactors.
13. **Steps must name files; complex steps show code.** Every step should identify the specific files (and line ranges where known) that will change. For non-trivial changes, include a brief before/after code snippet. Vague steps ("refactor auth module") give the agent nothing to anchor on — and an error in the plan is an error in every line that follows from it. Every step ends with `Claim:` one sentence the reviewer can refute by a command, a test name or two quoted passages, naming its oracle (testing skill, The claim). A step is one claim, about one screen of diff: a claim whose "and" joins two changes is two claims, so split it into two steps; an "and" between checks of one change ("cases 54 and 55 pass") is not. "Consistent", "traceable" and "complete" are adjectives, not claims: write the check that would show them false (`` `grep -c "T11" docs/swarm.md` gives the same count in §5, §6, §13 and the diagram ``).
14. **Surface repeated corrections; do not compound them.** If you have made the same mistake twice on a step, stop. Tell the user what you've tried and why it keeps failing; ask whether to compact what's been learned and start the step fresh. Do not continue correcting in-place — a history of corrections degrades trajectory and primes further failure.
15. **Resume by verifying state, not assuming it.** When picking up an in-progress plan from a previous session, do not assume completed steps are correct — verify. Check git log, today.md, and spot-check key files. Summarize what is confirmed, what is uncertain, and where work was interrupted. Present the plan summary required by rule 19 alongside that verification, then confirm with the user before continuing.
16. **Ask before deleting a picked-up spec's original.** `daisy plan pickup` never deletes the original `{slug}_PLAN.md` itself — after a successful pickup, ask the user whether to delete it now that its content is tracked via the `PLAN.md` symlink, and only run `rm` if they confirm.
17. **Resolve the real path before writing `PLAN.md`.** `PLAN.md` is a link: write the plan at the path `daisy files plan` prints, as `prompts/daisy.md`'s Trigger block says of every Daisy file, for every edit during `execute`, `resume` and `archive`. The rule binds the main agent, the orchestrator during `execute` and `resume`: it alone writes the plan, and a step agent never does (rule 20).
18. **Live-test auto-committing commands only against a hermetic fixture.** Never exercise a command or script that auto-commits (or otherwise mutates) against a real, in-use home. Use `daisy test`'s fixture harness, or a scratch home, registry and workspace under a temp directory (`DAISY_CONFIG` and `DAISY_LOCAL` pointed there before `daisy home new <fixture-home>`, and `daisy init <fixture-home>` run in a scratch workspace; praxis rules 14–15), and tear it down afterward.
19. **Show the plan before asking for approval.** Every readiness question — `/daisy plan`'s "Ready to execute?" and `/daisy resume`'s "Ready to resume from Step N?" — must be immediately preceded, in the same response, by a summary the user can answer from without opening the file. In this order: (1) the plan name and its metadata (created date, status); (2) one line on what the plan accomplishes; (3) every step, one line each, optionally followed by a sentence or two on what it entails and any constraint on it — every step, not an abridged selection; (4) risks, each with its mitigation; (5) open questions. A question the user must open a file to answer defeats the point of asking it.
20. **Execute each step in a fresh sub-agent.** During `/daisy execute` and `/daisy resume`, the main agent orchestrates and does not implement. For each step it briefs a fresh sub-agent, not a fork, with the step written into the brief, not the plan's path to read: the plan's Goal in a sentence, the step's text, the Decisions that name the step or bear on it, and the Constraints; and with `$DAISY_ROOT/prompts/praxis.md`, the `testing` skill, the review gate and cap (rule 22), its reviewer's model and effort (rule 21), the hand-back's shape (rule 25) and that it neither reads nor writes the plan file. The step agent states the step's claim, does the step, reviews it, commits (not when it stops at the cap, rule 22) and hands back its decisions, Deferred candidates and open questions. The orchestrator writes the plan: after verifying the hand-back (rule 23) and before the metrics and the log, it marks the step `- [x]` (not at the cap), records the step's decisions in Decisions, its Deferred candidates and each out-of-gate remark that names a defect at a file and line in Deferred (rule 22), and its open questions in Open Questions.
21. **Ask which models run the steps and the reviews.** During `/daisy plan`, ask the user which model and effort level the step agents and the review agents use, and record the answer in Decisions. For the review agents, recommend a Sonnet-class model: it is the default, and another model reviews only when Decisions names a reason for it (the user's answer is reason enough). When `/daisy execute` or `/daisy resume` finds no answer in Decisions (a plan reached through `plan spec` and `pickup`), ask it before launching the first step, and record the answer in Decisions. Each step brief names its reviewer's model and effort. The reason for the default: `docs/plan.md`, Models and step size.
22. **Review to a stated gate, with a cap.** A separate review agent attacks the step's diff, with the step's intent and claim stated: a pass tests the claim. A finding is in gate only when the reviewer reproduces it (quotes the code or passage and shows the failure against the step's text or a rule) and it falls in one of four classes: the claim fails, a required item is missing or wrong, a test is red, a document contradicts another. Every pass's brief names these four and adds none, not even a past finding's type. A green test is not red, and a missing one is in gate only when the step's text names it; a contradiction is in gate only when the diff introduced it (compare with `git show HEAD:<file>`); a hole two passages leave open goes to Open Questions (from a step agent, through its hand-back, rule 25). The first pass reviews the diff; each later pass reviews only the fixes (the previous findings, the diff since the tree the last reviewer saw, and the files it touches) and reports whether each finding is closed and whether a fix introduced an in-gate defect in the lines it changed. A prose-only claim has two tests (a document contradicts another; a required item missing or wrong), so one pass exhausts them: its fixes are committed without a second pass and the residue goes to the hand-back. Before fixing anything the step agent classifies each finding: in gate, disputed (the reproduction does not hold or the class is not named: one sentence why, not fixed) or out of gate; it fixes only the in-gate ones. Out of gate are style, anything outside the step's stated contract, after pass 1 any text no fix touched, and a finding that needs an arrangement the project's own contract does not support (in Daisy's root, a link a user makes into it). Out-of-gate remarks go to the hand-back, not Deferred; the orchestrator keeps one in Deferred only when it names a defect at a file and line. The cap is 5 passes. A step not clean after pass 5 stops: the step agent does not commit and hands back the open findings with its proposed fixes uncommitted; the orchestrator takes them to the user, neither commits nor marks the step at the cap and never runs another pass. The reasons and the history: `docs/plan.md`, The review gate.
23. **Verify every hand-back before the next step.** The orchestrator checks the claim, runs the tests and `daisy healthcheck --force` itself, runs the project's hygiene check where it has one, and confirms the commits before it writes the plan (rule 20) and launches the next step; for a hand-back stopped at the review cap, it confirms that the step has no commit (rule 22). It never re-runs for a sub-agent an action the permission gate refused the sub-agent: it reports the refusal to the user verbatim and stops.
24. **Run unattended between the stops.** Do not ask the user before each step. Stop when the run needs something only the user can give and the plan does not record: a decision of theirs (rule 21's answer when Decisions has none; a rearchitecture that crosses a decision they made, praxis rule 4), an approval (a hard-to-reverse or outward-facing action the plan does not call for, such as a force-push, a deletion or a publication; a step the plan gives the user), or a way past a block (a step stopped at the review cap, rule 22; the same mistake twice, rule 14; a permission refusal, rule 23). The plan's own checkpoints are stops too.
25. **Hand back in one shape.** The step agent's report gives: the claim and the probes that showed its tests can fail; lines added and deleted; each review pass with its in-gate count, the findings disputed with the reason, the findings accepted as is, and its out-of-gate remarks, each with the file and line the reviewer gave (rule 22); at the cap, the open findings and the proposed fixes left uncommitted; the verification lines (tests, health check, hygiene); the commits, with the step's commit range and the repository it is in; the decisions it made, each with the alternative considered; its Deferred candidates (rule 8) and open questions; the tokens used, the step agent's and each reviewer's, each with its model and effort.

# Daisy — Plan Workflow

## /daisy plan \<description\> [+project]

1. Call `daisy plan new "<description>" ["+project"]`
2. Enter plan mode — only edit `PLAN.md`; do not write implementation code.
3. **Research phase:** Read what already exists relevant to the task. Identify: (a) what already solves part of this, (b) what's essential to the problem (inherent) vs. accidental (artifact of current approach). Write findings into the `## Research` section of `PLAN.md`. Ask: **"Research summary confirmed — proceed to planning? (yes/no)"**
4. Scaffold plan sections from conversation context: fill in Goal, Non-goals, Constraints; leave Steps as placeholder checkboxes.
5. If a `+project` tag was given: update `.daisy/projects/{project}.md` — add a Plans section with a relative link and status `in progress`.
6. Ask which model and effort level the step agents and the review agents use; record the answer in Decisions (rule 21).
7. Close each planning round with the plan summary (rule 19) and the Open Questions through `AskUserQuestion`, the last of them "Ready to execute?" (rule 3).

## /daisy plan spec \<description\> [+project]

1. Call `daisy plan new --spec "<description>" ["+project"]` — writes `{slug}_PLAN.md` directly to the workspace root. No `PLAN.md` symlink is created and nothing is registered in `$DAISY_HOME/plans/` yet.
2. Same collaborative planning discipline as `/daisy plan` (rules 1–3) — only edit the draft file, research before scaffolding, end with Open Questions.
3. Multiple specs can coexist in the same directory (each has its own `{slug}_PLAN.md`) — useful for sketching more than one candidate approach before choosing.
4. When a direction is chosen, promote it: `/daisy plan pickup <path-to-spec>`.

## /daisy plan pickup \<path\>

1. Call `daisy plan pickup "<path>"` — copies the spec into `$DAISY_HOME/plans/` with the standard timestamp+slug filename and creates the `PLAN.md` symlink. The original file at `<path>` is left untouched by the command.
2. Ask the user: **"Delete the original spec file now that it's tracked? (yes/no)"** Run `rm <path>` only if they confirm (rule 16).
3. From here the plan behaves like any other — proceed with `/daisy execute` when ready.

## /daisy execute PLAN.md

0. Resolve the plan's real path with `daisy files plan` and write every change below there (rule 17).
1. Read `PLAN.md`; confirm at least one unchecked `- [ ]` step exists, and that Decisions records rule 21's answer (ask it if not).
2. Update `**Status:** executing` in the frontmatter block (write to the resolved real path).
3. Work through the steps in order, each in a fresh sub-agent (rule 20) briefed with the step written in, not the plan's path (the plan's Goal in a sentence, the step's text, the Decisions that name the step or bear on it, the Constraints), and with `$DAISY_ROOT/prompts/praxis.md`, the `testing` skill, the review gate and cap (rule 22), its reviewer's model and effort (rule 21), the hand-back's shape (rule 25) and that it neither reads nor writes the plan file.
4. The step agent, which neither reads nor writes the plan file, reads `praxis.md`, states the step's claim, does the step, reviews it to the gate, commits (not when it stops at the cap, rule 22) and hands back, with its decisions, Deferred candidates and open questions in the hand-back (rule 25).
5. Verify the hand-back (rule 23), then write the plan (to the resolved real path): mark the step `- [x]` (not a step stopped at the cap), record its decisions in Decisions, its Deferred candidates in Deferred with each out-of-gate remark it reports that names a defect at a file and line (rule 22), and its open questions in Open Questions; then record the step's metrics with `daisy metrics add --step <n>`, two rows: `--role step` with `--range <its commits> --repo <its repository>` and the step agent's model, effort and tokens, and `--role review` with the reviewers' model and effort, `--review-rounds` and all review passes' tokens summed (no range, so no lines); a step stopped at the review cap is recorded too, with no range and `--outcome "held at cap"`, its tokens and rounds; then log the step via `daisy log` (rule 9), before launching the next step; do not ask the user between steps.
6. Stop and report at the stops of rule 24, or if blocked; do not skip steps or work around blockers silently.
7. On all steps complete: append ` *Built*` to the H1 title; update `**Status:** built` (write to the resolved real path).
8. Log execution summary to `today.md` via `daisy log`.

## /daisy archive PLAN.md

1. Warn if `**Status:**` is not `built` — confirm the user wants to archive anyway.
2. **Rehome the Deferred list.** For each item under the plan's `## Deferred`, verify it against the code before moving it — some will already be done. Move the live ones into the project file's `## Deferred` section (creating it if absent), with the context that makes them intelligible. Strike through and date each item found resolved, the plan's and any in the project file's `## Deferred` that the plan closed, and move it, its text kept, to `.daisy/projects/_deferred-closed/<project>.md` (creating it if absent) under a new heading at the top that names the date and the plan; the project file's `## Deferred` keeps live items only. Then the open items sit in one mutable place instead of being copied into the next plan, where a resolved item keeps looking overdue, and the closed ones stay findable without being re-read at every research phase. An item becomes a `todo.txt` task when it is scheduled, not before.
3. Call `daisy plan archive`
4. Log the archive event to `today.md` via `daisy log`.

## /daisy resume PLAN.md

0. Resolve the plan's real path with `daisy files plan` and write every change below there (rule 17).
1. Read `PLAN.md` fully — note `**Status:**`, completed steps (`[x]`), and the first unchecked step (`[ ]`).
2. **Verify completed work:**
   - Run `git log --oneline -10` to see recent commits.
   - Check today.md for log entries from the previous session.
   - Spot-check 1–2 key files that completed steps claim to have changed — confirm the changes are present.
3. **Read `## Decisions`** in PLAN.md for context recorded by the previous session; if Decisions has no answer to rule 21's question, ask it before step 7.
4. **Identify the interruption point:** Was the previous session stopped mid-step, between steps, or blocked? A step with a commit but no `- [x]` stopped before the orchestrator wrote it into the plan (rule 20), whether before or after its hand-back: verify it as rule 23 says before marking it, and list its decisions, Deferred candidates and open questions under Unknown unless its commits or today.md record them.
5. **Surface gaps:** What context was established during the previous session that is not recorded in PLAN.md?
6. Present a resumption summary to the user:
   - **Completed and verified:** steps done and confirmed in the workspace
   - **Uncertain:** steps marked done but unverified, or partially done
   - **Next:** the step to resume from, and any known sub-state
   - **Unknown:** context that cannot be reconstructed from artifacts
7. Ask: **"Ready to resume from Step N? (yes/no)"** — do not proceed without confirmation.
   If "Uncertain" items are significant (multiple unverifiable steps, or a prior session with a correction history), also offer: "Alternatively, I can compact what's confirmed into a fresh context — say 'compact' to replan from here instead of continuing in-place."
8. If the user says "compact": summarise confirmed work into a Research handoff (exact files changed, decisions made, constraints discovered), write it into the `## Research` section of PLAN.md, reset unchecked steps to reflect remaining work, update `**Status:**` to `planning`, and ask: **"Research summary confirmed — proceed to replan? (yes/no)"**
9. On confirmation to resume: update `**Status:** executing`, then continue from the identified step as `/daisy execute` does, one fresh sub-agent per step (rule 20).
