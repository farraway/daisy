---
name: swarm
description: The maintainer's guide over a Daisy swarm from its repository's clone — reads `daisy swarm status`, the briefs and the runs, explains what the swarm did and what waits for the maintainer, drafts or flips a brief's Status, edits swarm.json, the persona prompts and the per-repository notes, and runs the maintainer's `daisy swarm` verbs, each write only on the maintainer's yes. Use in a swarm repository's clone (swarm.json at its top), or in a folder holding one (the verbs run inside the clone), when the user asks what the swarm did or what waits for them, wants an issue triaged, a plan accepted, a job classed routine or architecture, a pull request approved or merged, a blocked issue unblocked, a job blocked by another, a job re-planned, the swarm or one issue paused or resumed, the swarm restarted on a new Daisy build, or a job closed, or wants to write or change a brief, swarm.json, a persona prompt or a repository's notes, or to bring the persona prompts to Daisy's current defaults.
short_description: Guide a Daisy swarm: status, briefs, gates
---

The swarm is Daisy's reconciler over a forge: `daisy swarm apply`, as the
maintainer's user service, launches the five roles (owner, architect, worker,
adversary, sympathetic), and the QA when `swarm.json` declares it,
and moves each issue's `swarm:*` label. Its
contract is `docs/swarm.md` in the Daisy root. This skill is the
maintainer's side of it: you read, explain and draft; the maintainer
decides; a forge write happens only through one of the verbs below, run
as rule 3 says.

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
   quoting the run's own words where they matter (of the lines `daisy
   swarm journal` shows, `--issue <n>` prints one issue's). Then
   "Waiting for you" from `status`, at its end before Held (`daisy swarm
   status --waiting` prints that line alone; on the forge, the same issues carry `swarm:gated`, and each gating posted a gate note on the issue naming what to examine and the verbs), each item with the decision it asks for and the
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
   one write is not a yes to the next. Under `/daisy swarm connect`
   (`prompts/swarm-connect.md`), a write that lies wholly within an
   authority the clone's `AUTHORITIES.md` grants has the maintainer's
   yes standing, and you cite the authority by its label in the log line
   and in any comment the write posts; with no file, none is granted.
   Show the verb's output as printed. `pause` and `resume` are writes
   too.
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
   `unblock`'s, `block`'s, `class`'s and `replan`'s comments. A comment of the
   maintainer's holding a line `/swarm <verb> [<words>]` is a command:
   `apply`'s next pass runs that verb on the issue under the
   maintainer's token and answers in the thread (`docs/swarm.md`
   section 6, The comment commands), so a draft meant as conversation
   never has a line beginning `/swarm`; when the maintainer would rather
   act from the forge, say the line to type (`/swarm accept`, `/swarm
   block --by 9`) and that the answer below it, `Done:` or `Refused:`,
   is the verb's output. The reconciler's answers are in the thread
   (`role=reconciler kind=command`) and the swarm's journal.
8. **Log each write as it is done**, as rule 6 of the Daisy prompt
   (`prompts/daisy.md` in the Daisy root) says of all work: each verb you run that writes and each edit you make to
   a brief, `swarm.json`, a prompt or a repository's notes, before you
   report it, and a verb that refused, as a blocker. Run `daisy log`
   with what it did, and with `--co-author "<your line>"` before the
   message so its commit says you wrote it, the line being the one your
   model signs commits with, one line `Name <email>` (a Claude model's
   is `Claude <model> <noreply@anthropic.com>`; `daisy log` refuses
   another shape, and a flag after the message would land in the
   journal):
   `daisy log --co-author "<your line>" "swarm: merged lib#5, closed #10"`.
   Outside a Daisy workspace `daisy log` refuses
   (`Not in a Daisy workspace`): say so once and go on.

## The verbs

`daisy swarm prompt build`, which reseeds the persona prompts from
Daisy's current defaults, is under Reseeding the persona prompts, below.

Each verb in the table runs in the swarm repository's clone; one given an issue `<n>` refuses before any write when its token is the swarm's, and prints `Maintainer <login> on <owner>/<repo>` first, while the bare `pause`, `resume` and `restart` write only the state directory.

| Waiting for you | Verb | What it writes |
|---|---|---|
| `triage #n` | `daisy swarm triage <n>` | `swarm:proposal-posted → swarm:proposal-accepted`; the architect plans it next |
| `plan #n` | `daisy swarm accept <n>` | `swarm:plan-0-posted → swarm:plan-1-accepted`; the worker builds the next slice |
| `triage #n` or `plan #n` the maintainer finds routine; a routine job that should be read first | `daisy swarm class <n> routine`, `daisy swarm class <n> architecture` | routine clears `swarm:class-architecture` and posts `Class: routine`: Daisy then passes the job's triage and plan gates at its next pass; architecture sets the flag and posts `Class: architecture`: the job waits at every gate it has not yet passed. No member answers either comment |
| `merge #n` (approval first, when a branch rule asks one) | `daisy swarm approve <n>` | a formal approving review of the issue's open pull request at its head |
| `merge #n`, `next slice #n`, `close #n` | `daisy swarm merge <n>` | merges the current pull request at the head both swarm reviews approve (or finds it merged), then `swarm:review-2-approved → swarm:plan-1-accepted` when the plan declares a later slice, or closes the issue after its last, labelled `swarm:done`; a pull request the forge reports not mergeable (`base` moved under it) goes back to the worker instead: the direction to merge `base` on the pull request and `swarm:review-2-approved → swarm:review-3-changes-requested`, printed `#<n> not mergeable: back to the worker` |
| `pull closed #n` | none: a relabel on the forge, or `daisy swarm close <n>` | the approved issue's current pull request was closed unmerged, which `merge` refuses; the maintainer decides whether the slice is built again (relabel `swarm:plan-1-accepted` on the forge) or the job dropped |
| `notes swarm/notes-<n>-<role>-<k>` | none: git in the clone | a run's research notes changed the same lines of a note as `main` (two runs re-dating one `Checked:` line, say), so Daisy pushed them to that branch; merge it into `main` by hand, keeping the newer of each pair of lines, push, and delete the branch on the forge: the next pass drops it from the list |
| `no check <name>` | none: an edit of `swarm.json` | the merge gate is off and the repository `<name>` declares no `check`, so the merge step merges its pull requests on the pair's approvals with no check of Daisy's run; declare its `check` (`["make", "check"]`, say), or turn `gates.merge` on, each an edit of `swarm.json` on the maintainer's yes: the line drops it at once |
| `question #n` | `daisy swarm unblock <n> <answer...>` | a member's question the architect and the owner passed on (the issue's newest reconciler note names it): the answer as the comment, then the `swarm:blocked` flag cleared; the member that asked runs again and reads it under `# Questions` |
| `blocked #n` | `daisy swarm unblock <n> [--cause <word>] [--start base] <comment...>` | the comment, saying what changed, then the `swarm:blocked` flag cleared; the issue goes on from the stage the block kept. `--cause <word>` retags the stop's cause in the metrics row of the run that set the block, when the maintainer reads it otherwise than the run recorded it: one of `plan`, `prerequisite`, `main-red`, `environment`, `forge-fact`, `kept-work`, `ruling`, `scope`, `capacity` (docs/swarm.md 7.5). The slice's next worker run starts from the commits the blocked run kept; when the maintainer wants them left out (the kept work is wrong, not unfinished), `--start base` writes the line `Start: base` first in the comment, and the run starts as with no kept run: from `base`, or from the open pull request's head (docs/swarm.md 7.8, The start); the kept commits stay in the mirror. From the forge, `/swarm unblock --start base <comment...>` |
| a plan to change: a direction, a ruling, a slice to split (not `blocked #n`: no block is needed) | `daisy swarm replan <n> [direction...]` | `Re-plan: <direction>` (`Re-plan.` with none) on the issue: the architect plans again at the next pass; between slices it revises the remaining slices, merged ones kept, or writes `STANDS:`, and the revision goes to the plan gate; a job with no plan moves to `swarm:proposal-accepted`, one in review whose pull request is closed or merged to `swarm:plan-1-accepted`. Refused while a pull request is open (close or merge it first), a member runs or a question waits; from the forge, `/swarm replan <direction>` |
| a job to drop | `daisy swarm close <n> [comment...]` | the comment when given, then the issue closed, labelled `swarm:rejected` (a job done closes through `merge`, which finds a pull request merged by hand) |
| — | `daisy swarm pause`, `daisy swarm resume` | no member launches until resumed; resume also releases a held owner batch (a hold waiting on issues, `released once #<n> … are closed` in status, releases itself when they close) |
| a new Daisy build to put in service, or `State: … — restart requested at <time>` | `daisy swarm restart` | the running `apply` loop re-executes itself at its first pass with no member running (it does so by itself when the installed binary changes); a swarm never idle waits: `daisy swarm pause`, then `resume` once the journal shows `0-restart-…` |
| `Held: paused #n` | `daisy swarm pause <n>`, `daisy swarm resume <n>` | `swarm:paused` set or cleared on that issue alone: nothing launches on it and the merge step skips it, so other jobs go first or an urgent fix jumps the queue |
| `Held: #n blocked by #m` | `daisy swarm block <n> --by <m>` | `swarm:blocking` on #m, then the comment `Blocked by #m` on #n: no member launches on #n while #m is open and that comment is the maintainer's newest on #n |
| a step-in no verb makes: a hand edit of `swarm.json`, a hand-published patch, a notes branch merged by hand, a check rerun | `daisy swarm intervene --type <type> [--issue <n>] <note...>` (`--issue`: the act was on issue `<n>`) | the intervention's record alone, a row of kind `intervention` in the home's metrics CSV and a line in the swarm's journal; no forge call |

**Interventions.** Every verb in the table but `intervene` also takes
`--type <type>` (right after `<n>`, or leading): when the maintainer
steps in where the swarm should have gone on alone, the verb's writes
are made and the intervention recorded with its type, one of the
catalogue's `I1` to `I14` (docs/swarm.md section 11, The interventions'
record, lists them), so the interventions a day are counted from the
record. Propose the type with the verb when you ask for the
maintainer's yes; a type outside `I1` to `I14` is refused, nothing
written.

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
the prompts are current; show its output. Then state its write as
rule 3 says (the prompts it would rewrite, the commit and the push)
and run `daisy swarm prompt build` only on that yes. For an
edited prompt, draft the default's changes into it as rule 6 says,
from the printed diff; `daisy swarm prompt build --force`, which
overwrites the edits, only on the maintainer's yes to losing them, named
prompt by prompt. A reading list written into a prompt (paths under
`/job/resources`) belongs in `swarm.json` instead: give each resource
a one-line `note` (`{"path": "<path>", "note": "<what it is, for whom>"}`),
which every run's `system.md` lists beside its path under
`# Resources`, and the prompt's own line can return to the default's.

Every path a member has is under `/job`, the run's folder
(`docs/swarm.md` section 7.2): its clones at `/job/work/<name>`, its
io at `/job/io`, the research notes at `/job/notes`, each resource
at `/job/resources/<repository>/<path>` and, for a run on an issue, the
job's workspace, read-only, at `/job/memory`. Before that layout a member's
clone was `/work`, holding `.swarm`, `.notes` and `.resources`. An
edited prompt, or a repository's notes (`repos/<name>.md`, which
`prompt build` never writes), that still names `/work` or a path under
it points the member at a path it no longer has: after a `prompt build`
that keeps an edited prompt, and whenever the maintainer asks whether
the swarm's text is current, look (`grep -n '/work' prompts/*.md
repos/*.md` in the clone) and draft each line's move to `/job` as rule
6 says.

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

The QA, when `swarm.json` declares `qa` and its persona,
runs after the set number of merges or a release tag, drives the
product from the page map the maintainer keeps in the first
repository's `repos/<name>.md`, and files each failure as a proposal,
which the gates handle as the owner's. When it fails or writes
`BLOCKED:` (no page map, no browser in the image), `status`'s `State:`
line says `qa held: <reason>`; `daisy swarm resume` releases it
once the maintainer has fixed the cause (`docs/swarm.md` section 6, The
QA trigger).

The briefs live in the swarm repository's `briefs/` (`forge.repo`, by
convention `<owner>/swarm-<name>`), declared as a `resources` entry.
Daisy writes the clone only at `init` and, for the prompts, at
`prompt build`, and reads the briefs from the
pushed head: a brief you draft or flip there is committed and pushed by
the maintainer, by hand, and reaches the owner at the next pass after
the push; and with `notes` the research notes Daisy pushes to the
repository's `notes/` reach the clone only when the maintainer pulls it.
