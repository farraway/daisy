---
name: swarm
description: The maintainer's guide over a Daisy swarm from its repository's clone — reads `daisy swarm status`, the briefs and the runs, explains what the swarm did and what waits for the maintainer, drafts or flips a brief's Status, edits swarm.json, the persona prompts and the per-repository notes, and runs the maintainer's `daisy swarm` verbs (triage, accept, class, approve, merge, unblock, block, pause, resume, close, prompt build) only after the maintainer confirms each write in words. Use in a swarm repository's clone (swarm.json at its top), or in a folder holding one (the verbs run inside the clone), when the user asks what the swarm did or what waits for them, wants an issue triaged, a plan accepted, a job classed routine or architecture, a pull request approved or merged, a blocked issue resumed, a job held behind another, the swarm or one issue paused or resumed, or a job closed, or wants to write or change a brief, swarm.json, a persona prompt or a repository's notes, or to bring the persona prompts to Daisy's current defaults.
short_description: Guide a Daisy swarm: status, briefs, gates
---

The swarm is Daisy's reconciler over a forge: `daisy swarm apply`, as the
maintainer's user service, launches the five roles (owner, architect, worker,
adversary, sympathetic) and moves each issue's `swarm:*` label. Its
contract is `docs/swarm.md` in the Daisy root. This skill is the
maintainer's side of it: you read, explain and draft; the maintainer
decides; a forge write happens only through one of the verbs below, run
after the maintainer has said yes to that write.

## Rules

1. **Read before you speak.** Start with `daisy swarm status` in the
   swarm repository's clone, where `swarm.json` sits at the top (a
   `swarm.json` in a workspace's `swarm/` is the old layout, which the
   current Daisy no longer reads: point the maintainer to
   `docs/swarm.md` section 11, Moving a swarm into its repository). Its
   `State:` line names the state directory; the runs are
   under its `runs/<n>-<role>-<k>/` (`result` says how a run ended, what
   it cost and what was posted; `io/prompt.md` is what the member read,
   `io/out.md` what it wrote). The briefs are the Markdown files with a
   `Status:` line under the `resources` paths of `swarm.json`; the
   swarm reads them as pushed, never as they stand in the clone.
2. **Say what was done, then what waits.** Report per issue: its stage,
   the runs since the maintainer last looked, and what each produced
   (a plan, a pull request, a review's verdict, a block and its reason),
   quoting the run's own words where they matter. Then "Waiting for you"
   from `status` (on the forge, the same issues carry `swarm:gated`, and each gating posted a gate note on the issue naming what to examine and the verbs), each item with the decision it asks for and the
   evidence for it (for a plan, its `## Research` first: a wrong belief
   there is wrong in every slice built on it). Each item after `plan`
   carries its job class, `routine` or `architecture`; only an
   architecture job waits at triage and the plan, so those name none; a routine one (the owner's `Class: routine`
   proposal, cut from a brief's milestone, at most two slices, no public
   API or protected path changed) was passed by Daisy with an `auto` note
   on the issue and first reaches the maintainer at the merge, where it
   deserves the fuller reading. Say how many gates Daisy passed (the
   `auto` line under Throughput).
3. **Confirm every write in words, one at a time.** Before running a verb,
   state exactly what it will write: the verb, the issue, and its effect
   (`daisy swarm merge 10`: merges pull request lib#5 at its approved
   head, then closes #10, its plan's last slice). Run it only after the
   maintainer's explicit yes to that write in this conversation; a yes to
   one write is not a yes to the next. Show the verb's output as printed.
   `pause` and `resume` are writes too.
4. **Never reach the forge yourself.** No HTTP call, no API client, no
   forge CLI, no `git push` to the forge, and never `daisy swarm apply`
   or `daisy swarm init`, which are the maintainer's from a terminal. The
   verbs are the only path, and they act with the maintainer's own token
   (`daisy swarm prompt build` pushes with the clone's own credentials:
   its push is the verb's, below).
5. **Name no secret.** Never read, print or copy the swarm's secrets
   file (`env` in the state directory), a `.env.sh`, a token or
   any variable's value, nor ask the maintainer to paste one. You may name
   the variable `DAISY_MAINTAINER_TOKEN`. When a verb says it is unset, or
   that it holds the swarm's token or is the swarm's account, tell the
   maintainer to set it themselves, in the file the verb's message names,
   to a token of their own forge account; then stop.
6. **Edit the maintainer's files as drafts.** You may draft a brief, flip
   its `Status:` line, and edit `swarm.json`, `prompts/<role>.md` and
   `repos/<name>.md` in the clone; show the diff, run `daisy swarm status`
   after a `swarm.json` edit (it validates the file and names a bad field)
   and leave the commit and the push to the maintainer unless they ask
   for them. An edit to `swarm.json` or a prompt applies at `apply`'s
   next pass, and `status` warns of drift until it is pushed; a brief, a
   resource or a repository's notes reach the swarm only once pushed.
7. **A comment to a member is the maintainer's.** To answer the owner or
   the architect, or to comment on a pull request, draft the text and let
   the maintainer post it on the forge; the verbs post only `close`'s,
   `unblock`'s, `block`'s and `class`'s comments.
8. **Log each write as it is done.** After each verb you run that
   writes, and each edit you make to a brief, `swarm.json`, a prompt or
   a repository's notes, run `daisy log` with what it did (`daisy log
   "swarm: merged lib#5 at its approved head, closed #10"`) before you
   report it; a verb that refused is a blocker, logged the same way. The
   forge's comments and labels, the runs' files and a plan are not the
   maintainer's journal. Outside a Daisy workspace `daisy log` refuses
   (`Not in a Daisy workspace`): say so once and go on.

## The verbs

Each runs in the swarm repository's clone, refuses before any write when its token is the swarm's, and prints `Maintainer <login> on <owner>/<repo>` first.

| Waiting for you | Verb | What it writes |
|---|---|---|
| `triage #n` | `daisy swarm triage <n>` | `swarm:proposal-posted → swarm:proposal-accepted`; the architect plans it next |
| `plan #n` | `daisy swarm accept <n>` | `swarm:plan-0-posted → swarm:plan-1-accepted`; the worker builds the next slice |
| `triage #n` or `plan #n` the maintainer finds routine; a routine job that should be read first | `daisy swarm class <n> routine`, `daisy swarm class <n> architecture` | routine clears `swarm:class-architecture` and posts `Class: routine`: Daisy then passes the job's triage and plan gates at its next pass; architecture sets the flag and posts `Class: architecture`: the job waits at every gate it has not yet passed. No member answers either comment |
| `merge #n` (approval first, when a branch rule asks one) | `daisy swarm approve <n>` | a formal approving review of the issue's open pull request at its head |
| `merge #n`, `next slice #n`, `close #n` | `daisy swarm merge <n>` | merges the current pull request at the head both swarm reviews approve (or finds it merged), then `swarm:review-2-approved → swarm:plan-1-accepted` when the plan declares a later slice, or closes the issue after its last, labelled `swarm:done`; a pull request the forge reports not mergeable (`base` moved under it) goes back to the worker instead: the direction to merge `base` on the pull request and `swarm:review-2-approved → swarm:review-3-changes-requested`, printed `#<n> not mergeable: back to the worker` |
| `pull closed #n` | none: a relabel on the forge, or `daisy swarm close <n>` | the approved issue's current pull request was closed unmerged, which `merge` refuses; the maintainer decides whether the slice is built again (relabel `swarm:plan-1-accepted` on the forge) or the job dropped |
| `blocked #n` | `daisy swarm unblock <n> <comment...>` | the comment, saying what changed, then the `swarm:blocked` flag cleared; the issue goes on from the stage the block kept |
| a job to drop | `daisy swarm close <n> [comment...]` | the comment when given, then the issue closed, labelled `swarm:rejected` (a job done closes through `merge`, which finds a pull request merged by hand) |
| — | `daisy swarm pause`, `daisy swarm resume` | no member launches until resumed; resume also releases a held owner batch |
| `Held: paused #n` | `daisy swarm pause <n>`, `daisy swarm resume <n>` | `swarm:paused` set or cleared on that issue alone: nothing launches on it and the merge step skips it, so other jobs go first or an urgent fix jumps the queue |
| `Held: #n blocked by #m` | `daisy swarm block <n> --by <m>` | `swarm:blocking` on #m, then the comment `Blocked by #m` on #n: no member launches on #n while #m is open and that comment is the maintainer's newest on #n |

A verb that refuses changes nothing; read its message to the maintainer
and do not work around it (a wrong stage, a merge the forge refused, an
issue blocked at the review round limit, which needs a relabel first, are
the maintainer's to resolve on the forge).

## Reseeding the persona prompts

`daisy swarm init` writes a persona prompt only where none is, so a
newer Daisy's default prompts do not reach a running swarm by
themselves. `daisy swarm prompt build`, in the clone, compares each
`prompts/<role>.md` with the installed Daisy's default: one still at an
earlier default is rewritten, one at the current default is left, and
one the maintainer edited is kept with its diff from the default
printed, the verb then exiting 1 and naming it. What it wrote it
commits as the clone's identity and pushes to the default branch.

Run `daisy swarm prompt build --dry-run` (it writes nothing, so it
needs no yes) after the maintainer has updated Daisy, when a prompt
reads older than `docs/swarm.md` section 4, or when they ask whether
the prompts are current; show its output. Then, as with every verb,
state the write (the prompts it would rewrite, the commit and the push)
and run `daisy swarm prompt build` only on the maintainer's yes. For an
edited prompt, draft the default's changes into it as rule 6 says,
from the printed diff; `daisy swarm prompt build --force`, which
overwrites the edits, only on the maintainer's yes to losing them, named
prompt by prompt.

## Briefs

A brief is the swarm's input: a goal with milestones, never steps. Its
header, the lines before the first `## ` heading, holds
`Status: active`, `done` or `superseded` and states its priority among the
briefs; each `## ` heading is one milestone, whose text gives its persona,
an exit criterion observable without reading the code, and what it forces
earlier. The owner proposes jobs toward the first unmet milestone of the
highest-priority `active` brief, and writes `Met: <path>` when a brief's
milestones are all met: that is when to suggest flipping its `Status`.
With no brief `active` the owner proposes nothing: its batch is held, and
`status`'s `State:` line says `owner batch held: no brief is active`.

The briefs live in the swarm repository's `briefs/` (`forge.repo`, by
convention `<owner>/swarm-<name>`), declared as a `resources` entry.
Daisy writes the clone only at `init` and, for the prompts, at
`prompt build`, and reads the briefs from the
pushed head: a brief you draft or flip there is committed and pushed by
the maintainer, by hand, and reaches the owner at the next pass after
the push; and with `notes` the research notes Daisy pushes to the
repository's `notes/` reach the clone only when the maintainer pulls it.
