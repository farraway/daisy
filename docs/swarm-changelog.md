# The swarm — Changelog

How [docs/swarm.md](swarm.md), the swarm's contract, came to say what it
says. Each change to the contract adds a line here, in the commit that
makes it, naming the step, the sections it changed and the decision it
records under [`docs/swarm/decisions/`](swarm/decisions/).

## To 2026-11-15: the Status note

The Status note at the head of docs/swarm.md as it stood on 2026-11-16,
before the split, moved here verbatim. In it, "section 14" is the
decisions now under `docs/swarm/decisions/`, "item N" is decision N
(`docs/swarm/decisions/NNNN-*.md`), and "the header" is this note.

**Status.** Specified at Step 26 from the plan's agreed design and revised
at Step 38 (revision 2: two conversations, one per forge surface;
configurable human gates with a spending limit; and the changes taken from
a reading of revision 1 against five public talks on software factories,
listed under Sources), and at Step 40 (revision 3: what Step 39's manual
run of the five roles showed, credential slots and the image's
entrypoint, verified prompt files, the stage copied to pull requests),
and at Step 43 (revision 4: the swarm's own forge account, a board as a
view, and one job across the repositories of an organization), and at
Step 3 of the plan that builds it (revision 5, 2026-11-06: the brief as
the swarm's input and the four levels brief, job, plan, slice; the
architect's plan as a standard Daisy plan opening with its Research and
carrying a diagram when a slice spans components; the adversary's check
that each named test fails on `base`; the round limit returning the
issue to the architect; the meta loop's data contract; and two diagrams,
in sections 1 and 5; section 14, items 55–59), and at Step 1 of the
plan that makes the swarm repository the swarm's root (revision 6,
2026-11-10: the swarm's own repository, `<owner>/swarm-<name>`, holds
`swarm.json`, the persona prompts, the briefs, the research notes, the
docs that span the products and the swarm's notes about each product
repository, `repos/<name>.md`; `daisy swarm` runs from its clone, with
or without a Daisy workspace; product repositories are declared by
forge name; members read pushed content only; the secrets live in
`<state>/env`; `init` creates and seeds the repository; sections 1–4,
7.1–7.3, 7.6, 7.7, 8–11 and 13, and section 14, items 97–104). **Revision 6 is
built:** Steps 2–6 of that plan built it, each amending the
sections it touches with its As built (Step 2, 2026-11-10: finding the
swarm with no Daisy workspace, the secrets in `<state>/env` and the
unit's root; section 11, Finding the swarm, and item 105; Step 3,
2026-11-10: the product repositories by forge name, the mirrors'
remotes recorded in their configs, the notes' clone at the swarm
repository's own URL, the readiness read from the mirrors and the
wizard's forge names; item 106; Step 4, 2026-11-10: the swarm
repository's mirror, the resources and references checked out of the
mirrors at the pass's commits under one mount, the briefs read there
and the old workspace path of a brief still naming it; item 107; Step
5, 2026-11-10: `repos/<name>.md` in a member's `system.md`, `init`'s
seed of it and Drift; item 108; Step 6, 2026-11-10: `init` creating or
adopting the swarm repository with the maintainer's token, cloning it in
a folder holding none, seeding, committing and pushing it, and writing
`<state>/env`; section 10's three repository calls; item 109; Step 6a,
2026-11-10: a variable unset at `init` written commented out in
`<state>/env`; item 110; Step 6b, 2026-11-10: question 3's default
from `init --api <url>`, else `$GITEA_API_URL` or `$GITHUB_API_URL`,
else detection; item 111; Step 7, 2026-11-10, once the maintainer had
moved every swarm: the workspace layout retired, a swarm run from its
repository's clone alone, with no `repos[].path`, workspace
`resources`, `<ws>/.env.sh`, research wiki or old workspace path of a
brief; section 11, Moving a swarm into its repository, describes the
old files for a maintainer moving a swarm with an older Daisy; item
112). Step 1 of the plan that clears the Deferred list (2026-11-11)
amended section 4: the default persona prompts carry the documentation
duties of the architect, the worker and the adversary, and lose the two
sentences that repeated the Task text (item 113). Its Step 3 (2026-11-11)
amended sections 7.1 and 7.6: a member's clone has no automatic
maintenance, and the check of a worker's clone skips a path gone between
its directory's listing and the check's look at it (item 114). Its Step 4
(2026-11-11) amended sections 7.1 and 7.4: the architect run while the
issue's pull request is open has that pull request's head as a branch of
its repository's clone, which the open-slice sentence names (item 115).
Its Step 5 (2026-11-11) amended sections 7.1, 7.3 and 7.7: the
architect's and the worker's `system.md` carry the installed root's
praxis and testing skill (item 116). Its Step 7 (2026-11-12) amended
sections 2 and 11: `swarm prompt build` rewrites, commits and pushes
each persona prompt of the clone that is still an earlier default, and
keeps an edited one, printing its diff (item 117). Its Step 13
(2026-11-12) amended section 11: the `swarm` skill is installed in each
workspace by `daisy init`, with the root's other skills, no longer in
`~/.claude/skills` by `daisy install`. Its Step 16b (2026-11-12) amended
sections 2, 3, 7.6, 9 and 11: each collected run is recorded, a line in
the swarm repository's journal and a row in the metrics CSV of the home
`<state>/home` names, and a cost the CLI does not report is recorded as
missing, not as 0 (item 118). Its Step 17 (2026-11-12) amended the
header, sections 2–7, 9–11 and 13 and items 76 and 81 in wording only,
to match the code as built (item 76 now points at item 89). Its Step
18 (2026-11-13) amended the header, sections 1, 2, 8–13 and items 4, 18,
19, 22, 28, 47, 50, 51, 55, 61 and 86: section 13 is retitled Seams and
says once that its first table's steps are those of the plan that
specified this document, and a bare Step 30 elsewhere names that plan;
the self-approval's `422` (and, in section 13, the first real run's
comment-type reviews), the organization labels' `403`, `--effort`
and `codex`'s `--model`, `codex`'s trailer and `--add-dir`, and an
organization label's id across repositories state what the recorded
runs found (sections 8–10 and 12, items 47 and 50), and the points
those passages leave open are marked *not established as of
2026-11-13*; the budget reads as validated, not enforced, wherever it
read as in force; items 18, 22, 61 and 86 are completed; and item 55 is
no longer provisional (the maintainer confirmed it as built on
2026-11-12). Its Step 19 (2026-11-13) amended sections 4 and 7.4–7.7:
a brief's `**Status:**` line is read as `Status:`; with one repository
declared, a slice prefix naming no repository the swarm has a mirror
of is the slice's text; the round r > 1 reviewer's diff takes the pull
request's files whole and shows nothing when there are none; the round
limit's note after the first says `again`; a worker's posted text
loses a final CRLF whole; an abandoned worker run's failed fetch is
retried as a worker run's is, and what it fetched is kept whatever
follows; a run's copy of the notes that cannot be read is `notes
kept`; and a mount point the product's tree holds as a file is the
fault *clone failed* (item 119). Its Step 20 (2026-11-13) amended
sections 3, 10 and 11: a model with a control or format character is
refused; questions 3 and 5 default from detection when question 2
answers another kind than the file's, the API keeping the origin's
scheme and port; Gitea's `Relabel` removes a name by each of its ids
and its lists read on past a short page; `forge.Fake` answers a merge
of a closed pull request `404`; `init` warns of a kept prompt that
validation refuses and takes a variable of blanks alone as unset, and
its unit quotes `Environment=` and `ExecStart=` and says where to save
it (item 120). Its Step 21 (2026-11-13) amended item 23: the base
image's two `FROM` lines name its digest beside its tag.
Step 1 of the plan for the inbox requests F27–F35 (2026-11-14) amended
sections 5, 6, 10, 11 and 12: a merge reads the approved head's checks
first, a failed one sending the issue back to the worker (T26) and a
pending one, or none while `base` requires them, waiting for the next
pass (item 121). Its Step 2 (2026-11-14) amended sections 2, 3, 7.1–7.3,
7.6, 7.7 and 11: a repository's optional `deps`, a dependency step run
before each of its checks with the network, writing the swarm's cache
for the repository, which the check then reads offline; a step that
fails is the check's result, named as the dependency step's (item 122).
Its Step 3 (2026-11-14) amended sections 1 and 4: the default persona
prompts take the general rules a live swarm's maintainer had added by
hand (item 123). Its Step 4 (2026-11-14) amended sections 3, 4, 7.3 and
11: a resource may carry a one-line `note`, and every run's `system.md`
lists the resources under `# Resources`, each with its index page and
its note, which the owner's and the architect's defaults read in place
of a path (item 124). Its Step 5 (2026-11-14) amended sections 2, 7.6
and 11: `swarm metrics --backfill` writes the row of each collected run
the home's CSV holds none for, as a collection makes it, and a row's
persona, without the run's `persona` line, is the one its argv
launched (item 125). Its Step 7 (2026-11-14) amended sections 5, 6,
7.3, 7.6 and 10–12: each issue's body carries a block, after the
owner's marker lines, listing its pull requests, linked, with their
states, which `apply` keeps with the forge seam's new `EditIssue` and
every reader of the body leaves out, and every note about a pull
request links it in its first line (item 126). Its Step 8 (2026-11-14)
amended sections 7.6 and 11: a run's notes commit conflicting with the
head only where both inserted lines at one place lands with both kept,
a push refused while the head moved is put on the new head and pushed
again, and `status` lists the notes' branches under Waiting for you
(item 127). Its Step 9 (2026-11-14) amended sections 3, 5, 6, 7.3–7.6,
11 and 13: a repository's `rulings`, by default its protected
`docs/adr/`, and a job between slices whose plan predates a change to
them runs the architect, not the worker, after a reconciler note naming
the changed files; the architect's `STANDS:` confirms the plan and a
revision goes to the plan gate (T27) (item 128). Its Step 11b
(2026-11-15) amended sections 2, 7.6 and 9: `gpt-6.1-sol` has a list
price, with a rate for cached input; `codex`'s cached input count is
recorded on the run's `cost` line, in its row's notes and in its journal
line, and the estimate prices cached and uncached input apart (item
129). Its Step 12 (2026-11-15) amended sections 2, 5, 6, 7.3–7.5, 10,
11 and 13: a worker's or a reviewer's `QUESTION:` gives its issue the flag
`swarm:question`, on which the architect answers or passes the question
to the owner, who answers or passes it to the maintainer, blocking the
issue; an answer clears the flag and the asker's stage resumes (item
130). Its Step 13 (2026-11-15) amended sections 2, 6, 7.3, 7.6, 8, 10,
11 and 13: a line `/swarm <verb> [<words>]` in a comment by a login in
`trusted` other than the swarm's, on an open swarm issue or its current
pull request, is run by `apply`, before the role rule, as the
maintainer's verb on the issue under `DAISY_MAINTAINER_TOKEN`, and
answered once in its thread by the reconciler, the answer naming the
comment's id; an edited comment runs nothing; neither a command nor its
answer wakes a member (item 131).
Its Step 14 (2026-11-15) amended sections 5, 6, 7.3, 11 and 13: the
maintainer's plain comment answering a block, on a thread the stage's
role reads, clears `swarm:blocked` at the next pass, which runs the
role; the maintainer's comment on a pull request in review reopens its
head, so both reviewers run again; a comment at `plan-2-in-progress`
is held for the role the run leaves the issue to, and one at
`review-2-approved` wakes no one, which role it should wake being open
(item 132). Its Step 15 (2026-11-15) amended sections 2 and 11: `swarm
list`, run anywhere, names each swarm this machine runs, its clone (which
`apply` records in `<state>/clone`), its home and its pause, and `swarm
journal` prints the journal since the line `--mark` last marked seen
(`<state>/connect-seen`), the read helpers of `/daisy swarm connect`
(item 133). Its Step 16 (2026-11-15) amended sections 2, 3, 7.2, 7.6,
7.7, 11 and 13: a product's test services file, `compose.test.yaml` at
its root or the file `repos[].services` names, of a stated subset of
Compose, read at the commit run: its services run beside each check on
a private internal network, which the check joins in place of none,
with the file's `x-daisy-env` variables, and beside each worker run on
a private network of its own, and are removed once the check or the
run has ended, whatever its outcome; a file outside the subset runs no
check, its refusal the check's result, and services that cannot start
are a swarm fault (item 134). Its Step 17 (2026-11-15) amended sections
2, 6, 7.4, 7.5, 11 and 13: a batch run's `BLOCKED: waiting on #<n>,
#<m>: <why>`, the form its Task text now asks for when what is left
waits on open issues, records the named issues still open in
`owner-held`, and the first pass at which the forge reads every one of
them closed removes the hold, with a journal line, so the batch runs
with no `swarm resume`; a hold waiting on no issue stays the
maintainer's, as does a pause (item 135). Its Step 18 (2026-11-15)
amended sections 2, 3, 5 and 11: each reconciler note that puts an
issue in Waiting for you, a blocking note or a gate note but the
blocked gate's, ends `cc @<maintainer>`, the first login of `trusted`
that is not the swarm's, so the forge notifies the maintainer; and an
optional `notify` argv is run on the host once per new item waiting
for the maintainer, a swarm fault's pause and the notes branches
included, with its one line on stdin, its failure a warning (item
136). Its Step 19 (2026-11-15) amended sections 2 and 11: the `apply`
loop re-executes itself, the same PID and arguments, at the first pass
after which nothing of the swarm is in flight, when a new build stands
at the path of the binary it runs from or `daisy swarm restart`, the
eighteenth verb, asked for it, with a stdout and a journal line; a
build that cannot be executed is a warning, the loop going on (item
137). Its Step 20 (2026-11-15) amended sections 1, 5, 6, 7.4 and 11:
`daisy swarm replan <n> [direction...]`, the nineteenth verb, and
`/swarm replan [direction...]` post the maintainer's `Re-plan:
<direction>` on the issue, a counted entry, so the architect plans or
revises at the next pass with no block or unblock; between slices the
request runs the architect in the worker's place as changed rulings
do, merged slices kept (T27); a job with no plan, or in review with its
pull request closed or merged, is moved where the architect reads it
(T31); an open pull request, a run in flight or a question waiting
refuses it (item 138). Its Step 21 (2026-11-15) amended sections 1, 2
and 11: the swarm repository may hold `AUTHORITIES.md`, the
maintainer's standing authorities for an agent acting through
`/daisy swarm connect`, which acts without asking only within one,
citing it by its label, and asks everything else; the maintainer
writes the file, `init` does not seed it and Daisy's code does not
read it (item 139).
Built by the plan that builds it (the swarm build), in place of the
Steps 27–30 that section 13's first table numbers, which are those of
the plan that specified it (the swarm build takes `swarm init` first;
among its steps that amended this document, its Step 1 built `declare`,
the forge calls `init` makes and the verb, amending
sections 3, 8–11, 13 and 14; its Step 4 built `status` and the rest of the
`gitea` adapter and `forge.Fake`, amending sections 3, 10, 11, 13 and 14;
its Step 5 built the provider and runner seams, the prompt files, the
trigger rule and `apply --once` running the architect, amending sections
2, 7.1, 7.3, 7.6, 9, 10, 11, 13 and 14; its Step 6 built the lifecycle,
the gates, the merge step, the owner's batch, `apply`'s loop, `pause`
and `resume`, amending sections 2, 3, 7.1, 7.6, 7.7, 11, 13 and 14; its
Step 9 the forge milestones that mirror the briefs, amending sections 4,
10, 11, 13 and 14; its Step 10 the maintainer's six verbs, amending
sections 1, 2, 5, 6, 8, 10, 11, 12, 13 and 14; its Step 15 the stages
renamed in groups and numbered, the two in-progress stages, the flags
`swarm:blocked`, `swarm:paused` and `swarm:blocking` with `swarm block`
and an issue's `pause` and `resume`, and the empty `co_author`, amending
sections 1–7, 10–13 and 14; its Step 19 the research wiki, amending
sections 2, 3, 7.1–7.4, 7.6, 11, 13 and 14; its Step 20 the job classes
and `swarm class`, amending sections 1–3, 5, 6, 7.3–7.5, 10–13 and 14;
its Step 23 the swarm's own repository and `notes`, amending sections
2–4, 7.1–7.4, 7.6, 8, 11, 13 and 14; its Step 24 the `xai` login slot,
amending sections 1, 3, 7.1, 7.2, 7.7, 8, 9, 11, 13 and 14; its Step 30
the configuration's kept lines, amending sections 3, 11, 13 and 14; its
Step 35 the configuration's and the unit's, amending sections 3, 11, 13
and 14);
the board, the GitHub adapter and the `budget` wait for a run that needs
them. The maintainer reads the sections each code
step's hand-back cites (the plan's decision of 2026-11-06, in place of a
reading of the whole before the first code step), and section 14 lists
the places where the document chose between readings. `docs/cli.md`'s entry for the Swarm group points here:
this document is the contract for the nineteen verbs, the configuration they
read, the forge calls they make, the containers they run and every file
they write.

## From 2026-11-16

- 2026-11-16, Step 26 of the plan remediating the 2026-11-15 design
  review (P24): the document split in three. Section 14's 139 items
  became the decision records `docs/swarm/decisions/0001-…` to
  `0139-…`, one file per item, each its item's text verbatim under a
  title line; the Status note moved here; sections 1–13 kept their
  numbers, each "section 14, item N" in them now a link to decision N,
  and the two "(item 7)" and "(item 4)" that named Step 39's checklist
  now say so. The convention for a change to the contract became: its
  sections, a decision file and a line here (docs/swarm.md, Changing
  this document). No choice of the contract changed, so no decision
  file was added.
- 2026-11-16, Step 28 of the plan remediating the 2026-11-15 design
  review (P22): every adapter's input count holds the cache's input,
  the metrics CSV gains `tokens_cached` and fills `tokens_total`, and
  `swarm metrics --reprice [--dry-run]` supersedes the rows written
  before; sections 1, 2, 7.6, 9 and 11; decision
  [140](swarm/decisions/0140-every-providers-input-count-holds-its-cache.md).
- 2026-11-16, Step 29 of the plan remediating the 2026-11-15 design
  review (talk A13): `swarm status`'s Throughput gains the `reverted`
  line, the merged swarm pull requests and those a revert on `base`
  names, read from the mirrors as last fetched; section 11; decision
  [141](swarm/decisions/0141-reverts-counted-from-the-mirrors.md).
- 2026-11-16, Step 30 of the plan remediating the 2026-11-15 design
  review (P29): a reviewer run's metrics row records `review_effect`,
  its blocking findings when its role's next review approves a new
  head, the earlier review's row superseded at that collection and
  `swarm metrics --reprice` making it for runs recorded before; 7.6,
  section 11; decision
  [142](swarm/decisions/0142-a-reviews-effect-read-from-the-runs.md).
- 2026-11-16, Step 33 of the plan remediating the 2026-11-15 design
  review (P21): `daisy swarm connect [--json]`, the twentieth verb,
  prints the connect briefing as one read-only report: status's
  State, Running, Waiting for you with each issue's gate note, Held
  and runs lines, the journal since the mark by issue, the open
  questions, `AUTHORITIES.md` quoted verbatim and the swarm's metrics
  since the last model or prompt change; Daisy reads `AUTHORITIES.md`
  only to quote it; sections 2 and 11; decision
  [143](swarm/decisions/0143-the-connect-briefing-as-one-report.md).
- 2026-11-16, Step 34 of the plan remediating the 2026-11-15 design
  review (talk A4): the default reviewers name, in a non-blocking
  finding of its own, a check over the whole tree that can recognise
  every instance of a finding's kind, and count as findings a
  workaround of a design clash (naming whether the plan or the change
  makes it) and a comment or description arguing a shortcut is
  acceptable instead of fixing it or listing it as not done; the
  owner's smallest job may be one that removes the cause of a
  repeated review finding the maintainer's comments, the resources or
  the repository notes record; section 4; decision
  [144](swarm/decisions/0144-a-recurring-finding-asks-for-a-check.md).
- 2026-11-16, Step 35 of the plan remediating the 2026-11-15 design
  review (talk A6): with `gates.merge` false, each repository whose
  `check` is empty is warned of by `apply` (`⚠ swarm.json: repos[<i>]
  (<name>) has no check and gates.merge is false: no check of Daisy's
  runs on its pull requests before the merge step merges them`, at the
  first pass and when the set changes) and listed by `status` and
  `connect` under Waiting for you as `no check <name>`
  (`waiting.no_check`), which notify tells; never refused; sections 2,
  3, 5, 11 and 12; decision
  [145](swarm/decisions/0145-a-merge-with-no-check-is-warned-of.md).
- 2026-11-16, Step 36 of the plan remediating the 2026-11-15 design
  review (talk A15): the lifecycle is explored, every sequence of up to
  five events over two issues checked against four invariants and
  section 5's table (`TestExploreLifecycle` in `internal/swarm/reconcile`),
  `forge.Fake` gaining `Clone(o)` to fork the forge at each state;
  sections 10 and 13; decision
  [146](swarm/decisions/0146-the-lifecycle-is-explored.md).
- 2026-11-17, Step 38 of the plan remediating the 2026-11-15 design
  review (talk A16): every worker's Task text asks for a screenshot of
  each page its change alters, under `/work/.swarm/artifacts/`; the
  collection takes each regular PNG, JPEG or WebP file at the
  directory's top, never through a link, warns of and records anything
  else, searches it for the member's secrets and attaches it to the pull
  request through the new forge method `Attach` (Gitea's
  `issueCreateIssueAttachment`; GitHub's adapter will refuse it, having
  no such endpoint), a refusal recorded `not attached`; each reviewer's
  prompt lists the head's screenshots under `# Artifacts`, copied into
  its own `/work/.swarm/artifacts/`, with a Task sentence asking it to
  judge them; sections 2, 7.3, 7.4, 7.6, 7.7, 10 and 13; decision
  [147](swarm/decisions/0147-screenshots-reach-the-pull-request-and-the-reviewers.md).
- 2026-11-17, Step 38b of the plan remediating the 2026-11-15 design
  review (request F40): a member's text Daisy cut on the forge reaches
  each later member that acts on it whole: the run's `out.md` copied to
  the member's `/work/.swarm/outputs/<n>-<role>-<k>.md`, the cut's note
  in its prompt naming that file, and a Task sentence listing the files,
  the plan by name; the plan, a review and a pull request's description
  alike; sections 2, 7.2, 7.3, 7.4, 7.5 and 13; decision
  [148](swarm/decisions/0148-a-text-cut-on-the-forge-reaches-the-member-whole.md).
- 2026-11-17, Step 38c of the plan remediating the 2026-11-15 design
  review (request F40): Daisy reads a member's text it cut on the forge
  whole, from the run's `out.md` under Step 38b's rules, wherever it
  reads more than its first lines: the plan's slices (the worker's
  slice, its Task text and `# Plan` heading, the merge step, `swarm
  merge`, `status` and notify), the plan and merge gates' notes, and a
  review's `[blocking]` lines and their count in a later prompt, so a
  plan whose `## Steps` the cut reaches is no longer closed after its
  first slice; with the out.md gone, the forge's text as before;
  sections 7.3, 7.5 and 13; decision
  [149](swarm/decisions/0149-daisy-reads-a-text-it-cut-whole.md).
- 2026-11-17, Step 38d of the plan remediating the 2026-11-15 design
  review (request F40): every text Daisy posts on the forge that carries
  a member's words (a member's text, the reconciler's notes on a run,
  the gate note; not yet the merge step's notes, the rulings note or a
  proposal's body) reaches it whole: one longer than 60 000 bytes is
  posted cut, what Daisy writes
  after it (a worker's check block, a note's last sentences, the gate
  note's decision and mention) never cut, its whole text attached to
  the issue as `<n>-<role>-<k>.md` (`<n>-gate.md` for the gate note)
  and linked from the cut's note, or, where the forge refuses the file
  (GitHub among them), its rest in numbered continuations after it,
  marked `role=reconciler kind=continued`, which no rule counts and no
  prompt shows; `Attach` takes an issue as well; sections 5, 6, 7.3,
  7.5, 7.6, 10 and 13; decision
  [150](swarm/decisions/0150-a-long-text-is-posted-whole.md).
- 2026-11-17, Step 39 of the plan remediating the 2026-11-15 design
  review (talk A17): QA, a sixth role a swarm declares or not
  (`qa`, `{"merges": <n>, "tag": <pattern>}`, with
  `personas.qa`; no default number), runs as issue 0 after that
  many commits on the first repository's `base` first-parent line, or
  a tag matching the pattern, since its last completed exploration;
  it drives the product from the page map the maintainer keeps in the
  first repository's notes, in that repository's image, and files each
  failure as a proposal marked `role=qa`, classed by its first
  line, which enters at T1 or T2 as the owner's do, or writes `NO
  FAILURE:`; a failure, `BLOCKED:` or nothing proposed writes
  `<state>/qa-held`, which `status` names and `resume` removes;
  sections 2, 3, 4, 5, 6, 7.1–7.7, 11 and 13; decision
  [151](swarm/decisions/0151-a-qa-run-after-merges.md).
- 2026-11-19, Step 3 of the plan implementing the 2026-11-19 swarm
  roadmap (R0): the models the live swarm ran without a price each have
  a list price from their provider's pricing page, the cached input
  apart, `claude-fable-5-2`, `gpt-6.2-astra`, `gpt-6-luna` and `grok-4.7`
  added, and the cached rates of `claude-fable-5-1`, `claude-opus-5-5`
  and `claude-sonnet-5-5`; one rate per model, so the estimate is below
  the bill for a cache write and for a `grok-4.7` request past 200,000
  prompt tokens; section 9; decision
  [152](swarm/decisions/0152-every-model-in-use-priced-from-its-providers-page.md).
- 2026-11-19, Step 4 of the plan implementing the 2026-11-19 swarm
  roadmap (R0): a member's stop names its cause, one line `Cause:
  <word>` under its `BLOCKED:` or `QUESTION:` line, which every Task
  text's closing paragraph asks for, the word one of the roadmap's
  `plan`, `prerequisite`, `main-red`, `environment`, `forge-fact`,
  `kept-work`, `ruling`, `scope` and `capacity`, each defined in one
  line; the run's metrics row records it in a new last column,
  `cause`, for a run whose outcome is `blocked` or `question`, empty
  for any other, for a stop without the line and, with a warning, for
  a word outside the vocabulary; Daisy's own stops name none yet;
  sections 7.4, 7.5, 7.6 and 11; decision
  [153](swarm/decisions/0153-a-cause-on-every-member-stop.md).
- 2026-11-19, Step 5 of the plan implementing the 2026-11-19 swarm
  roadmap (R0): Daisy's own stops record a cause from the same nine
  words, read from the run's `fault:` or `failed:` line: a fault
  `environment`, a provider's limit, quota or credit `capacity`, bad
  slices `plan`, a protected path `scope`, a failed fetch
  `environment`, every other failure empty; the merge step's blocks
  have no record; `swarm unblock <n> --cause <word>` supersedes the row
  of the run whose stop it releases with that cause, by the metrics
  CSV's own rule for a correction, before its comment; sections 7.5,
  7.6, 7.7 and 11; decision
  [154](swarm/decisions/0154-daisys-own-stops-carry-a-cause.md).
- 2026-11-19, Step 6 of the plan implementing the 2026-11-19 swarm
  roadmap (R0): the maintainer's interventions are recorded: every verb
  that changes the swarm's state on the maintainer's behalf (`triage`,
  `accept`, `class`, `approve`, `merge`, `unblock`, `block`, `replan`,
  `close`, `pause`, `resume` and `restart`; also from a comment
  command) takes `--type <type>`, one of the catalogue's `I1` to `I14`,
  and once its writes are made records the intervention, a metrics row
  of a new kind, `intervention` (the type in `cause`, the verb in
  `outcome`, run `<n>-intervention-<k>`), and a journal line; `daisy
  swarm intervene --type <type> <note...>` records an act no verb
  makes; the journal queue takes `<state>/journal.lock` and its push
  keeps a line queued since; sections 2, 6, 7.6 and 11; decision
  [155](swarm/decisions/0155-interventions-recorded.md).
- 2026-11-19, Step 7 of the plan implementing the 2026-11-19 swarm
  roadmap (R0): `daisy swarm metrics --scorecard [--since <time>]`
  prints a row per merged slice (a merge commit in the mirrors whose
  second parent a worker run pushed, as the reverted line reads one),
  computed from the metrics CSV's last rows, the mirrors and each
  issue's opening on the forge, and writes nothing: the runs by role,
  the uncached, cached and output tokens, the estimate at list price, a
  row without one counted $0 and named, the wall-clock from the issue's
  opening (or the previous slice's merge), the review rounds, the stops
  by cause and the interventions by type, then a summary of the
  roadmap's baseline figures over the window; a revert that a later
  revert names (a reapply) no longer counts, in `status`, `daisy
  metrics` and the scorecard; `status`'s Throughput counts runs ending
  in a question apart and adds the runs' estimate at list price; a
  run's row is dated by its end, as a backfill dates it; a journal line
  with no persona says `unknown persona`; `--backfill` supersedes the
  earlier reviews' rows whose effect a backfilled review decides,
  `--reprice` keeps an effect the runs read unknown and drops `<c>
  cached input` from a row's notes beside `tokens_cached`; sections 2,
  7.6 and 11; decision
  [156](swarm/decisions/0156-a-scorecard-per-merged-slice.md) (0141
  amended).
- 2026-11-19, Step 8a of the plan implementing the 2026-11-19 swarm
  roadmap (R0): `daisy swarm metrics --scorecard` takes `--until
  <time>`, inclusive as `--since` is, leaving out every record and
  merge after it, of the slices' lines as of the summary, the window
  ending at it or now, whichever is earlier; `--until` before
  `--since`, or without `--scorecard`, is usage; section 11; decision
  [157](swarm/decisions/0157-the-scorecard-window-ends-at-until.md)
  (0156 amended).
- 2026-11-19, Step 9 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, design): the job's workspace is the memory, specified
  ahead of its build. Section 1's principle amended: members have no
  memory, but each job has a workspace Daisy keeps, and the forge
  thread is its human record, whose posts may be cut; section 7.8 gives
  `<state>/jobs/<n>/` (`plan.md` with its amendments' slot,
  `slices/<s>/slice.md`, `kept` and `rounds/<r>.md`, `checks/<sha>.log`
  and `texts/`), written by `apply` alone at collection, mounted
  read-only at `/work/.job`, kept after the issue closes; the open
  branches in each run's io at its launch; the next worker run's start
  from the slice's kept commit unless a `Start: base` line of the
  architect's answer or the maintainer's unblocking comment says
  otherwise; every checkout at `/work/<name>`, the mounts beside the
  checkouts, a confined CLI started in `/work`; the prompt's `# Job`
  pointers in place of the bodies; Daisy's own reads from the
  workspace; decisions 148 and 149 retired once built, 150 kept as the
  human record's; a job in flight built by replaying its runs; sections
  1, 2, 7.1 to 7.6, 7.8, 9 and 11; decision
  [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md).
- 2026-11-19, Step 9b of the plan implementing the 2026-11-19 swarm
  roadmap (R1, design revised at Step 9's stop): every member run is
  one folder, `/job` (`<run>/job`, read-write, the run's own), its
  checkouts at `/job/work/<name>` for every role, its io at `/job/io`
  (was `/work/.swarm`), its notes and resources at `/job/notes` and
  `/job/resources`, the job's workspace read-only at `/job/memory` (was
  `/work/.job`); a confined CLI starts in `/job`, every other in its
  checkout with `/job` its one `--add-dir`; no mount in a checkout and
  none in a read-only mount; the check and the dependency step keep
  `/work`. The architect's builder plan is a document of its own,
  `<run>/io/builder.md` beside the plan's `out.md`, secret-checked, not
  posted, kept as the workspace's `builder.md` and re-issued whole with
  each revised plan (and, R5 deciding when, an amendment), each
  `slice.md` the slice's step and its `## Slice <s>` section; the plan's
  required `## For the builder` section is retired. `swarm merge`
  refuses a job with no workspace yet whose cut plan's `out.md` is
  gone; a revised plan's `Start: base` line resets the start; the
  collection's step 1 reads `<run>/work/<name>/.git`; `branches.md`'s
  readers named; a review whose round can no longer complete written to
  `texts/` at the next collection. Sections 1, 2, 7.1 to 7.6, 7.8, 8, 9
  and 11; decision
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md)
  (0158 amended).
- 2026-11-19, Step 10 of the plan implementing the 2026-11-19 swarm
  roadmap (R3, one check, one environment, failures classified;
  specified ahead of its build, section 7.9): the product's check
  environment declared once, `repos[].image`, `check` and `deps`, the
  test services file (gaining `ulimits` and block scalars) and a new
  `repos[].env`, given to the worker's and the reviewers' runs, the
  dependency step and the check; the check's and the dependency step's
  clone at `/job/work/<name>` once the members are there; the
  resources and the notes outside every checkout as decision 159 lays
  them out; test services beside a reviewer; the workflows CI runs on a
  pull request compared with the declaration (targets, services,
  variables) at `<base>`, a difference a warning on `apply`'s stderr,
  `status`'s Waiting for you (`ci differs <name>`), notify and `init`'s
  readiness, never a block. A check failure in packages the change does
  not touch (Go: the failing packages against the change's files and
  the module's imports) or holding one of five infrastructure
  signatures is run again once before any post; one that persists is
  the failure *check environment*, the issue blocked with cause
  `environment`, nothing posted for the worker or the reviewers; a
  worker run that starts from unpushed kept commits and adds none is
  not *no commits*. Sections 1, 2, 3, 5, 7.2, 7.5 to 7.9 and 11;
  decisions
  [160](swarm/decisions/0160-one-declared-check-environment.md) and
  [161](swarm/decisions/0161-a-check-failure-classified-before-anyone-sees-it.md)
  (0122, 0134 and 0159 amended).
- 2026-11-19, Step 10b of the plan implementing the 2026-11-19 swarm
  roadmap (R3, the maintainer's answers at the checkpoint; specified
  ahead of its build, section 7.9): CI builds the declared image itself,
  from the product's Dockerfile, which a new `repos[].dockerfile` names
  (the repository's root its context), in a step on the runner's
  Docker, whose base is local to the swarm's host (never as a job's
  `container:`, which the runner's `force_pull` would pull), the
  daemon's layer cache shared with the maintainer's builds, and runs
  the check's targets in it, the checkout copied in at
  `/job/work/<name>` and the job's network joined; the comparison with
  CI checks that a job builds from the declared Dockerfile and reads
  the targets, the services and the variables from those jobs' runs in
  the image, a mismatch still a warning; the limit, a runner whose
  Docker does not hold the base, stated. A signature classifies a
  failure as the environment's only when no package the change touches
  fails: a touched package's failure is the worker's. Sections 1, 3,
  7.6, 7.7 and 7.9; decision
  [162](swarm/decisions/0162-ci-builds-the-declared-image.md) (0160 and
  0161 amended).
- 2026-11-19, Step R1-1 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, built: `texts/`): the collection of a run on an issue
  writes each member text it posts whole to the job's workspace,
  `<state>/jobs/<n>/texts/<n>-<role>-<k>.md`, before its post, a review
  held until its round completes or can no longer complete; nothing
  reads the files yet. Section 7.8's `texts/` row says which collection
  writes a review whose round can no longer complete (the first that is
  not a fault's or an abandoned run's, which post nothing) and
  that one whose `out.md` is gone is not written; sections 2, 7.5, 7.6
  and 7.8 mark `texts/` built.
- 2026-11-19, Step R1-2 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, built: `plan.md`, `builder.md`, the slices' `slice.md`):
  the architect's texts that ask for a plan ask for the builder plan
  too, in `/work/.swarm/builder.md` (the builder sentence, and the plan
  closing naming it read back); the collection of an architect's plan
  reads it as `out.md` is read, secret-checked with it (*secret in
  output* naming `builder.md`), never posted, and writes `plan.md`, the
  plan whole, `builder.md`, or removes it when the run wrote none, and
  each declared slice's `slice.md`, its item under `## Steps` and then
  its `## Slice <s>` section; nothing reads them yet, and no post, and
  no prompt but the architect's, changes. Section 7.4 states the builder
  sentence and the plan closing, the sentence telling the architect
  that Daisy gives each worker its section joining at the switch;
  section 7.8's rows say a failed run writes no plan, a `builder.md`
  holding only whitespace is none, and how a slice is cut (fenced
  lines, the first section, a plan without slice lines); sections 2,
  7.5, 7.6, 7.7, 7.8 and 13 mark them built and name `builder.md`
  where the secret check is listed.
- 2026-11-19, Step R1-3 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, built: the slices' `kept` and `rounds/`, and `checks/`):
  a worker run with commits writes its slice's `kept`, `<n>-worker-<k>
  <sha>`, whatever its outcome, at step 2 for a run that keeps its ref
  and after the push for one that pushed, unless its commits fail step
  3's secret check or change a protected path, which step 2 now checks
  again before it writes, so a blocked, question, fault or abandoned
  run's commits are checked too (the run's outcome and its ref
  unchanged); a check that failed writes `checks/<sha>.log`, its `#
  Check` line and its whole output, which the secret check now reads
  whole (a hit outside the posted tail is *secret in output* naming the
  file, nothing written, posted or pushed); and the review that
  completes a round writes its slice's `rounds/<r>.md`, numbered over
  the life of the slice's pull requests, holding the head, each
  review's verdict and finding lines, and the diff since the round
  before in the pull request's files, computed in the mirror. Nothing
  reads them yet. Section 7.8 states the round file's two cases it left
  open (the maintainer's comment reopening the head, and commits that
  change none of the pull request's files), where the findings are read
  from and the diff's fence; sections 2, 7.6, 7.7, 7.8, 8 and 13 mark
  them built and name the failed check's whole output where the secret
  check is listed.
- 2026-11-19, Step R1-3b of the plan implementing the 2026-11-19 swarm
  roadmap (R1, the round file never holds a transition back): a round
  whose head, or the round before's, the mirror no longer holds (the
  branch force-pushed past it and the commit pruned, or the mirror made
  again) writes its file with `The changes since round <r-1> cannot be
  shown: <sha> is not in the mirror.` in place of the diff, and the
  review's collection makes T9, T10 or T11 as it would, where it
  returned the diff's error and the issue waited at every pass; any
  other failure of the diff is still an error of the collection.
  Section 7.8's round-file form states the sentence; no decision.
- 2026-11-19, Step R1-4 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, the rollout): a job open at a stage without the new
  marker `<state>/jobs/<n>/built` is built once, in pass step (5)
  before the role rule reads it, by replaying its collected runs'
  collection writes in the order they ended, through the functions that
  write at collection, so the replay is byte for byte the workspace
  their collections write; `built`, an empty file, is written last, and
  a new issue is replayed over no run before its first launch. The
  marker, not the directory's absence, is the trigger, since every
  collection since `texts/` makes the directory. A file of `texts/`,
  `rounds/` or `checks/` that is there is never rewritten, so a replay
  over a partial workspace completes it without rewriting what a
  collection wrote; the plan's text is read from `texts/` when it is
  there. A run collected elsewhere, pruned, or whose `out.md` is gone
  adds its text from its post, a cut's note replaced by `[truncated by
  Daisy; the rest is not available to you]`; when the plan `plan.md` is
  left holding is so taken, the replay blocks the issue with the
  reconciler's note naming the missing `out.md`. `swarm merge` refuses
  a job without `built` whose plan the replay would take cut, before any
  write. Sections 2, 7.6, 7.8 and 11 state the marker, the order, the
  written-once files, the note's text and what is built; `status`'s and
  the other verbs' in-memory build comes with their reads. No decision.
- 2026-11-20, Step R1-4b of the plan implementing the 2026-11-19 swarm
  roadmap (R1, a replay failure blocks only its issue): a failure that
  is the job's own, met by the replay of its workspace, by the read of
  its texts whole in pass step (5) or by the merge step's read of its
  plan, blocks its issue alone (T17, the reconciler's note naming the
  file and the cause, posted once), writes no `built` and launches or
  merges nothing for it, and the pass goes on with every other issue,
  where it returned the pass's error and stopped every launch of the
  swarm: a file of one of its runs or of its workspace that is there
  and cannot be read, or a worker's report posted with no check result
  whose repository now declares a check. A forge's read refused, a
  mirror's git error or a write is still the pass's error. Sections
  7.5, 7.7 (a new row, a job's file Daisy cannot use) and 7.8 (the
  Rollout, Daisy's own reads) state it. No decision: it applies the
  maintainer's rule of 2026-11-18, that an unreadable `out.md` blocks
  only its issue with a note naming the file.
- 2026-11-20, Step R1-5 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, built: the run's folder, `/job`, but for `/job/memory`):
  every member's container mounts `<run>/job` at `/job` and in it
  `/job/work` (`<run>/work`, a folder of clones, `/job/work/<name>` for
  every role, the worker's and a reviewer's included), `/job/io` (was
  `/work/.swarm`), `/job/notes` and `/job/resources` (were `/work/.notes`
  and `/work/.resources`); their mount points are made in `<run>/job`,
  none in a clone, and a clone's `.git/info/exclude` gains no line for
  them, so a product's tree holding `.swarm`, `.notes` or `.resources`
  no longer refuses the launch (*clone failed*'s mount-point case gone;
  a mount point that cannot be made in `<run>/job` is the pass's error,
  as the launch's other writes are). A CLI that is not confined starts
  in its checkout, `/job/work/<name>`, with `D` `/job` alone; `grok`
  starts in `/job`, and its prompt's `Working directory` line names the
  checkout for every role. Every path Daisy's own texts name moved with
  the mounts: the provider's argv (`/job/io/system.md`,
  `/job/io/prompt.md`, `/job/io/out.md`), the exit mark, the Task texts
  (the closing paragraph, the builder plan at `/job/io/builder.md`, the
  screenshots at `/job/io/artifacts/`, each clone at
  `/job/work/<name>`), the prompt's `# Repository`, `# Research notes`,
  `# Resources`, `# Briefs` and `# Artifacts` lines, and the copies of
  a cut text at `/job/io/outputs/`. The collection reads the worker's
  clone at `<run>/work/<name>` (at `<run>/work` for a run launched
  before, which has no `<run>/job`) and removes `<run>/job` with
  `<run>/work`. The persona prompts' defaults and the `swarm` skill
  still name `/work` until R1-6; the check and the dependency step keep
  `/work` until R3. Sections 2, 3, 7.1 to 7.4, 7.6 to 7.9 and 9 state
  it. No decision: it builds decision
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md).
- 2026-11-20, Step R1-6 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, the persona prompts on `/job`): the default persona
  prompts name the checkouts under `/job/work` where they named `/work`
  (the owner's and the architect's code under `/job/work`, the worker's
  branch and each reviewer's build in its clone under `/job/work`), each
  new default's sha256 appended to `defaultHistory`, so that `swarm
  prompt build` rewrites a prompt still at the default before it; the
  `swarm` skill names `/job`'s paths and has the maintainer's agent look
  for `/work` in an edited prompt and the repositories' notes, which
  `prompt build` never writes. Sections 4, 7.4 (the adversary's rollback
  sentence named `/work` where the Task text names `/job/work/<name>`),
  7.8 (the move's costs) and 11 (prompt build's table of earlier
  defaults) state it. No decision: it builds decision
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md).
- 2026-11-20, Step R1-7 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, the workspace's mount and the open branches): a run on an
  issue (every role's but the owner's and the QA's batch runs) mounts
  `<state>/jobs/<n>` read-only at `/job/memory`, after the resources'
  line, its mount point made in `<run>/job` with the others; before the
  launch of a worker's, a reviewer's or the architect's run,
  `<run>/io/branches.md` is written from the pass's mirrors, a line per
  open swarm branch but the run's own, in the repositories' declared
  order, then by issue and slice, a branch with no merge base with
  `<base>` saying `no history in common with <base>` in place of its
  files (the order, that line, and that the architect's run has no own
  branch, stated in 7.8, The open branches, where the spec left them
  open). No prompt or Task
  text names either until the switch (R1-10). Sections 2, 7.1, 7.2, 7.8
  and 9 mark them built. No decision: it builds decisions
  [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md) and
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md).
- 2026-11-20, Step R1-8 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, the start): a worker run on a slice starts from the
  slice's kept commit (`slices/<s>/kept`, read at its launch), its
  branch `swarm/<n>-<s>` made, or moved forward, there in its clone, the
  commit fetched from the mirror's run ref, or from the branch once
  pushed, so that its push is a fast-forward; from the branch's head
  when a push not the run's moved the branch past the kept run; and
  where it would with no kept run (`<base>`, or the open pull request's
  head) when the architect's answer or revised plan, or the
  maintainer's counted comment on the issue or its current pull request
  (`swarm unblock`'s, or the word that lifts the block), carries the
  line `Start: base` since the kept run ended. The kept commit and its
  ref stay. Where 7.8 left it open, stated there: the kept run's end
  read from its result (none readable: no line counts), an entry in the
  second it ended counted, the architect's text read whole, a kept
  commit no ref of the mirror holds (the kept run's ref naming it, or
  the slice's branch holding it) taken as none with a warning, and a
  `kept` Daisy cannot use blocking the issue (7.7). The worker's
  `# Job` line naming its start comes with the switch (R1-10). Sections
  7.1, 7.7, 7.8 and 11. No decision: it builds decisions
  [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md) and
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md).
- 2026-11-20, Step R1-9 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, Daisy's own reads): Daisy's reads past a member text's
  first lines read the job's workspace. The plan's slices and their
  repositories (the worker's slice, the count in its Task text and
  under `# Plan`'s heading, the merge step's, `swarm merge`'s,
  `status`'s and notify's) and the plan gate's note from `plan.md`; the
  merge gate's note's non-blocking findings and the architect's text a
  `Start: base` line is looked for in from `texts/`. A file there that
  cannot be read is met as an `out.md` is (7.5, 7.7). Where 7.8 left it
  open, stated there: a job without `built`, or whose `plan.md` is not
  there, has its plan built in memory as the replay would write it (so
  a file removed from a built workspace is read again from the runs,
  never taken for no plan), a text with no file in `texts/` is read as
  the replay would take it, and a plan so built that is taken cut from
  its post is blocked at the merge step as the replay blocks it. A
  review's `[blocking]` lines and their count stay a prompt's read until
  the switch (R1-10). Sections 2, 7.5, 7.7, 7.8 and 11, and section
  13's row.
  No decision: it builds decisions
  [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md) and
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md).
- 2026-11-20, Step R1-10 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, the switch, its first part, R1-10a: the pointers): a
  prompt on an issue carries `# Job` after `# Issue`, pointers into the
  job's workspace by role (7.8, The prompt), and `# Repository`'s line
  `- Job workspace (read-only): /job/memory`; `# Plan` is gone, the
  accepted plan in `# Thread` is its heading and pointer, the worker's
  reports (the pull request's description among them) are `# Job`'s
  Report line in the worker's and the reviewers' prompts, the worker's
  latest round is its reviews' `[blocking]` lines, and `# Direction`
  starts at the plan's newest basis, the plan or the architect's newest
  `STANDS:` after it. The worker's and the reviewers' Task texts gain
  the slice sentence, and the architect's builder sentence its last
  sentence, `Daisy gives each worker its slice's step and section.` The
  launch writes `branches.md` before the prompt, which counts its
  lines. Where 7.8 left it open, stated there: what each line reads, a
  line left out when its file is not there, the verdicts' form, the
  singular for one, a `plan.md` not there leaving the Plan line out and
  the plan as its `# Thread` entry, and a file there Daisy cannot use (a
  round's file or a `kept` not Daisy's) blocking the issue (7.7), the
  pair's second reviewer not launched after it. Decisions
  [24](swarm/decisions/0024-the-container.md) and
  [87](swarm/decisions/0087-grok-sees-only-its-working-folder.md)
  amended to the run's folder. Sections 2, 7.3, 7.4, 7.5 and 7.8. The
  prompt's reads of the texts it shows from `texts/`, and the retirement
  of the copies to `/job/io/outputs/` and of decisions 148's and 149's
  reads, are the switch's second part (R1-10b). No new decision: it
  builds decisions
  [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md) and
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md).
- 2026-11-20, Step R1-10 of the plan implementing the 2026-11-19 swarm
  roadmap (R1, the switch, its second part, R1-10b: the prompt's reads):
  every member text a prompt shows (an entry of `# Thread` or
  `# Questions`, the architect's view of a pull request's description
  and thread, a review's `[blocking]` lines and their count) is read
  from the job's workspace, `texts/`, as the prompt is rendered, for
  the texts it shows alone; with no file there, as Daisy's own reads
  take it (its run's `out.md` when Daisy cut it, else as posted). No cut
  text reaches a prompt: decision 148's copies to `/job/io/outputs/`,
  its rewritten note and its Task sentence are retired, and so is the
  pass's read of every cut text on the threads (`readWhole`); a text
  file that cannot be read blocks the issue at the launch (7.7), so a
  built job's `out.md` that cannot be read no longer blocks it, nothing
  reading it. Decisions
  [148](swarm/decisions/0148-a-text-cut-on-the-forge-reaches-the-member-whole.md)
  and [149](swarm/decisions/0149-daisy-reads-a-text-it-cut-whole.md)
  amended (149's "the one place a read can fail" corrected). Sections 2,
  7.2, 7.3, 7.4, 7.5, 7.7, 7.8 and 13. No new decision: it builds
  decision [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md).
- 2026-11-20, Step R1-11 of the plan implementing the 2026-11-19 swarm
  roadmap (the maintainer's answer to its Open Question 7): the plan gate's note
  names the builder plan after the slices, `` - Builder plan:
  `<state>/<file>` on the swarm's host, <b> bytes; not posted. ``, the
  workspace's `builder.md` or, with the plan read in memory, its run's
  `<run>/io/builder.md`, or `- Builder plan: none; each slice's worker is
  given its step alone.` The builder plan is still not posted. Sections
  5 and 7.8. Decision
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md)
  amended, its open question answered.
- 2026-11-20, Step R1-12 of the plan implementing the 2026-11-19 swarm
  roadmap (the maintainer's answer to its Open Question 12): a worker's
  report posted with no check result, whose repository has declared a
  check since, is replayed with the no-check block it was posted with,
  `**Check:** none declared.`, the block its run's records give, and
  the job is built, where it blocked its issue (R1-4b) with a note whose
  only remedy was removing the run's `out.md`. A job's own failure is
  now only a file of it there that cannot be read. Sections 7.7 (a
  job's file Daisy cannot use) and 7.8 (Rollout). Decision
  [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md)
  amended.
- 2026-11-20, Step R1-13 of the plan implementing the 2026-11-19 swarm
  roadmap (the maintainer's answer to its Open Question 16): `swarm
  unblock <n> --start base <comment...>`, and `/swarm unblock --start
  base <comment...>` in a comment, post the comment with the line
  `Start: base` and a blank line first, so the slice's next worker run
  starts where it would with no kept run; `--start` takes `base` alone,
  and `--cause` or `--start` given twice is usage. Sections 6 (The
  comment commands), 7.8 (The start) and 11. Decision
  [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md)
  amended.
- 2026-11-20, Step R1-14a of the plan implementing the 2026-11-19 swarm
  roadmap (attach-or-split's files and failures, scheduled by the
  maintainer's answer of 2026-11-18): an upload that fails with any
  error, not only a refusal, continues the text in comments, so a
  forge's `5xx` on its uploads neither keeps a run uncollected nor loses
  a first gating's note; the gate note's file is `<n>-gate-<time>.md`,
  each gating's its own. Sections 5 and 7.5, 7.6's retry, and the tests
  row. Decision
  [150](swarm/decisions/0150-a-long-text-is-posted-whole.md) amended.
- 2026-11-20, Step R1-14b of the plan implementing the 2026-11-19 swarm
  roadmap (attach-or-split's reach, scheduled by the maintainer's answer
  of 2026-11-18): the merge step's notes and the rulings note are posted
  by 7.5's rule, their last sentence never cut, attached as
  `<n>-merge-<time>.md` and `<n>-rulings-<time>.md` or continued, their
  retries reading the thread's last word past the swarm's
  continuations; a proposal's body longer than the limit opens its issue
  cut and continues on it (`created <i> #<m>` recorded), and a member
  and the triage gate note read it whole from the batch run's
  `out.md`. Sections 6 (the rulings), 7.5, 7.6 (the result's lines) and
  12, and the tests row. Decision
  [150](swarm/decisions/0150-a-long-text-is-posted-whole.md) amended.
- 2026-11-20, Step R0-7 of the plan implementing the 2026-11-19 swarm
  roadmap (the maintainer's answer to its Open Question 4): `swarm
  intervene --type <type> --issue <n> <note...>` records the
  intervention on issue `<n>`, `<n>-intervention-<k>`, its notes and
  journal line ending `#<n>`, so the scorecard counts it on the issue's
  slice; without `--issue` it stays on issue 0; `<n>` is not looked up
  on the forge, and one not an issue number, or `--issue` twice, is
  usage. The opening's list of the verbs, and section 11 (The
  interventions' record, daisy swarm intervene). Decision
  [155](swarm/decisions/0155-interventions-recorded.md) amended.
- 2026-11-20, Step R3-1 of the plan implementing the 2026-11-19 swarm
  roadmap (R3, one declared check environment): `repos[].env` built
  for the worker's run, the dependency step and the check, given before
  `deps.env`'s and `x-daisy-env`'s, refused when a name is not a
  variable name, is a secret's or is also in `deps.env`, or a value
  holds a newline; `init` keeps it. The reviewers' runs wait for R3-3.
  Sections 3 (`repos[].env`, the field order, the error table), 7.2
  (`--env`, the R3 block) and 7.9's opening. Decision
  [160](swarm/decisions/0160-one-declared-check-environment.md)
  amended.
- 2026-11-20, Step R3-1b of the plan implementing the 2026-11-19 swarm
  roadmap (R3; the maintainer's answer to its Open Question 8):
  `repos[].dockerfile` built as a field: a relative, clean path, not
  `.` nor ending `/`, one line, as `services` is (which now refuses `.`
  too), written after `image` and kept by `init`; the declared file is
  protected by default, a worker's change to it the failure *protected
  path changed* without the path in `protected`, matched whole, and the
  Task texts name it among the protected paths. The comparison with CI
  waits for R3-4. Sections 3 (the field order, `repos[].dockerfile`,
  `repos[].protected`, the error table), 7.4, 7.6 (step 3), 7.9 and 11
  (daisy swarm init, the fields not asked). Decision [162](swarm/decisions/0162-ci-builds-the-declared-image.md)
  amended.
- 2026-11-20, Step R3-2 of the plan implementing the 2026-11-19 swarm
  roadmap (R3): the test services file's `ulimits` built, a limit's name
  to one number or to exactly `soft` and `hard`, digits without a
  leading `0`, `soft` not above `hard`, run as
  `--ulimit <name>=<soft>:<hard>` in name order after `--tmpfs`; the YAML subset reads literal and folded block
  scalars (`|`, `>`, alone or with `-` or `+`), an indentation indicator
  refused by line; a flow mapping open at its line's end, which
  panicked, refused. Sections 3 (`repos[].services`), 7.2 (the services'
  `docker run`, the R3 block) and 7.9 (the opening, the comparison with
  CI). Decision [160](swarm/decisions/0160-one-declared-check-environment.md)
  amended.
- 2026-11-20, Step R3-3 of the plan implementing the 2026-11-19 swarm
  roadmap (R3): the adversary's and the sympathetic reviewer's runs get
  their repository's `env` and the test services file read at the head
  they review, on a network of their own named as the run's container,
  with `x-daisy-env`, made, warned of when refused and removed at the
  collection as a worker's are (`services <role> up|down|refused`);
  without either, a reviewer's argv is unchanged. Sections 2 (the
  result's lines), 3 (`repos[].services`, `repos[].env`), 7.2 (the
  member's `(variables)` and `(services)` lines, the arguments, the R3
  block), 7.6 (step 4), 7.7, 7.9 (the opening, Beside a reviewer), 11
  (stdout, stderr) and the test plan. Decision
  [160](swarm/decisions/0160-one-declared-check-environment.md)
  amended.
- 2026-11-20, Step R3-3b of the plan implementing the 2026-11-19 swarm
  roadmap (R3): the YAML subset reads a mapping or a list as a list
  item (`- name: x` with its further keys at its first key's
  indentation, `- - a`, a dash alone with its item below), nested, so a
  workflow's `steps`, `services` and matrix `include` are read; forms
  it read as text where PyYAML reads otherwise (a merge or null key,
  plain text beginning with an indicator, a tab outside quotes, flow
  entries empty or of one pair, a quote inside plain text hiding a
  comment) are refused by line or read as YAML does, an empty item
  stays refused; the services file refuses a list where a field takes
  none by that field's error. Sections 3 (`repos[].services`) and 7.9 (the comparison with
  CI). Decision
  [160](swarm/decisions/0160-one-declared-check-environment.md)
  amended.
- 2026-11-20, Step R3-4 of the plan implementing the 2026-11-19 swarm
  roadmap (R3): the comparison with CI built as a function,
  `declare.CompareCI`, of the forge's kind, the declaration, the test
  services file and the workflow files as read, returning 7.9's lines
  (not yet told: R3-5); where 7.9 was silent it reads a step's `run` as
  the shell splits it, a `${{ … }}` within its word, docker's options as
  docker does, a build without `-f` as of `Dockerfile`, a job's or
  workflow's `defaults.run.working-directory` as the step's, the
  network's line with `dockerfile` declared alone, and orders the lines.
  Sections 3 (`repos[].env`, `repos[].dockerfile`) and 7.9 (the opening,
  the comparison with CI). Decision
  [162](swarm/decisions/0162-ci-builds-the-declared-image.md) amended.
- 2026-11-20, Step R3-5 of the plan implementing the 2026-11-19 swarm
  roadmap (R3): the comparison with CI told, read from each declared
  repository's mirror at `<base>` with git's read commands alone:
  `apply` prints its lines on stderr in step (8) at the process's first
  pass and again when the set changes, `status` lists `ci differs
  <name>` after `no check` (`waiting.ci_differs`, `connect` too),
  notify tells `<owner>/<repo> waits for you: ci differs <name>` once,
  and `init`'s readiness prints each line as `⚠ Readiness: …`; a test
  services file refused at `<base>` is a line of its own and the
  comparison is made without it. Sections 2 (`<state>/notified`), 3
  (`repos[].env`, `repos[].dockerfile`), 5 (Notify), 7.9 (the opening,
  the comparison with CI) and 11 (`init`, `apply`, `status`), and the
  swarm skill's Waiting for you table. Decision
  [162](swarm/decisions/0162-ci-builds-the-declared-image.md) amended.
- 2026-11-20, Step R3-6 of the plan implementing the 2026-11-19 swarm
  roadmap (R3): the classification built as a function,
  `reconcile.Classify`, of the failed step's output, the change's
  paths, the head's Go facts and whether the step timed out, and the
  facts' reading from the mirror, `Git.GoFacts` (not yet called: R3-7);
  where 7.9 was silent it reads a `FAIL<tab>` line's path to its first
  blank or tab, resolves imports through every module and `vendor`
  directory of the head, each local `replace` and each symbolic link to
  a directory, counts a package's test files' imports for it
  alone, holds every package at or below a `go.work` or a module below
  the root, naming no package directory, or with a file whose imports
  Go's parser refuses as touched, holds a package the change deletes or
  moves away touched with its importers, and a submodule the change
  moves touched with every package that has an import, ignores ASCII
  case alone and names the earliest signature. Section 7.9 (the opening, the
  classification). Decision
  [161](swarm/decisions/0161-a-check-failure-classified-before-anyone-sees-it.md)
  amended.
- 2026-11-21, Step R3-7 of the plan implementing the 2026-11-19 swarm
  roadmap (R3): the collection classifies a worker's failed check or
  dependency step before the push, runs a classified failure again
  once (`check rerun: <reason>`, `check-1.out` or `deps-1.out`, the
  stdout line) and holds one classified again as the failure check
  environment, cause `environment`, nothing pushed, posted on the pull
  request or written to `checks/`; on the maintainer's answer of
  2026-11-21, a Go head's output naming no failing package is not
  classified, whatever signature it holds; the change and the head's
  facts are read only for an output that could be classified, and a
  check's rerun beside a dependency step is the check's alone.
  Sections 1 (the scope's check row), 2, 7.5, 7.6, 7.7, 7.8, 7.9 (the
  opening, the classification) and 11 (apply's stdout), each R3 marker
  of the rerun now built. Decisions
  [161](swarm/decisions/0161-a-check-failure-classified-before-anyone-sees-it.md)
  and [162](swarm/decisions/0162-ci-builds-the-declared-image.md)
  amended.
- 2026-11-21, Step R3-8 of the plan implementing the 2026-11-19 swarm
  roadmap (R3): a worker run that starts from kept commits not yet on
  the slice's branch records `unpushed <sha>` at its launch, and,
  fetching its start unchanged, has those commits to push: not *no
  commits*, its collection goes on to the secret check, the check and
  the push, and a `NO CHANGE:` first line from such a start is a
  revision's text; `kept` still names the kept run, and a run that
  does not push keeps the commits as its own ref, which the note of
  a failure or a `BLOCKED:` names. Sections 2, 5 (T12), 6 (an answer without a commit), 7.1,
  7.5, 7.6 (steps 1 and 2, the R3 paragraph, the trailer, the
  screenshots and the worker's row of the calls), 7.7 (no commits),
  7.8 (Rollout) and 7.9 (the rerun's table, the classification's As
  built). Decision
  [161](swarm/decisions/0161-a-check-failure-classified-before-anyone-sees-it.md)'s
  After the block is lifted, built.
- 2026-11-21, Step R3-9 of the plan implementing the 2026-11-19 swarm
  roadmap (R3): the check's and the dependency step's clone mounted at
  `/job/work/<name>`, their working directory, as a member's checkout
  is. Sections 3 (`repos[].deps`), 7.2 (the check's and the dependency
  step's argvs and rows, The run's folder, From R3), 7.8 (The costs) and
  7.9 (The paths' As built). Decision
  [160](swarm/decisions/0160-one-declared-check-environment.md)'s
  paths, built.
- 2026-11-21, Step R3-10 of the plan implementing the 2026-11-19 swarm
  roadmap: the sympathetic reviewer after the adversary approves. On
  `review-0-ready` the adversary launches alone; the sympathetic
  reviewer launches on a head once the adversary's review of it
  approves, in the pass that collects the approval; the adversary's
  request for changes completes the pair with its one review (T10,
  T11 at the limit); a round is a head on which the pair is complete,
  so `max_rounds` counts review cycles as before; the default
  sympathetic prompt drops its reading guide. Sections 4 (the
  sympathetic's row and prompt), 5 (the stages, the in-progress
  stages, T10, T21, Runs that do not move the label), 6 (rule 3's
  table, the role rule, the rounds, The maintainer's word), 7.3 (the
  rounds), 7.6 (the reviewers' calls), 7.8 (a round's file) and 11
  (the defaults' history). Decision
  [163](swarm/decisions/0163-the-sympathetic-reviewer-after-the-adversary-approves.md),
  amending decisions
  [78](swarm/decisions/0078-the-pass-does-not-wait-the-pair-in-one-pass.md)
  and
  [14](swarm/decisions/0014-sequential-reviewers-each-blind-to-the-other-on-the-head.md).
- 2026-11-21, Step R3-12 of the plan implementing the 2026-11-19 swarm
  roadmap: each role's effect measurable in the new order. A review's
  `review_effect` is read from its role's next review on the same pull
  request whatever round (a higher `round` line), not the next round
  only; the scorecard groups the window's runs by role and
  configuration (provider, model, effort, prompt hash). Sections 7.6
  (The run's record) and 11 (The scorecard). Decisions
  [142](swarm/decisions/0142-a-reviews-effect-read-from-the-runs.md)
  and [156](swarm/decisions/0156-a-scorecard-per-merged-slice.md)
  amended, and decision
  [163](swarm/decisions/0163-the-sympathetic-reviewer-after-the-adversary-approves.md)'s
  scorecard paragraph.
- 2026-11-21, Step R3-13 of the plan implementing the 2026-11-19 swarm
  roadmap: prompt parts, added to a role's prompt rather than edited
  into it. `swarm.json`'s `prompt_parts` names files in the clone and
  the roles each is added to, after `prompts/<role>.md`, as its own
  section, in the list's order; validation refuses a path outside the
  clone, a missing or empty file, an unknown role and an empty `roles`;
  the run's `prompt_hash` covers them; `swarm prompt build` replaces an
  unedited prompt whole, the swarm's additions in parts. Sections 3, 4,
  7.3, 7.6 and 11, and the swarm skill. Decision
  [164](swarm/decisions/0164-prompt-parts-added-to-a-roles-prompt.md).
- 2026-11-21, Step R3-12b of the plan implementing the 2026-11-19 swarm
  roadmap: a configuration hash. The metrics CSV gains `config_hash`,
  the sha256 of the role's prompt and its prompt parts as the run's
  `system.md` holds them (its `# Persona` line up to its `# Task`
  line), made at the collection and by `--backfill`, and by
  `--reprice` from a run's kept `system.md`; the scorecard groups by
  provider, model, effort and `config_hash` (`config <hash>`, `config
  unknown` for rows without one), `prompt_hash` kept as it was. Sections
  7.6 (The run's record) and 11 (the reprice, The scorecard). Decision
  [165](swarm/decisions/0165-a-configuration-hash.md), amending
  decisions [156](swarm/decisions/0156-a-scorecard-per-merged-slice.md)
  and [164](swarm/decisions/0164-prompt-parts-added-to-a-roles-prompt.md).
- 2026-11-21, Step R3-11 of the plan implementing the 2026-11-19 swarm
  roadmap: the spec reconciled with R1's and R3's build, prose only.
  The markers "specified ahead of its build" of what R1 and R3 have
  built now say built, with their steps: sections 1 (the principle,
  the scope's check row), 2 (the layout's `jobs/<n>/` and
  `check-1.out`, the files' `jobs/<n>/` row, the result's `check
  rerun`), 7.2 (From R3), 7.6 (From R3), 7.7 (The job's workspace),
  7.8 (its head), 7.9 (its head, naming what R3 leaves to the product
  and its maintainer) and 11 (From R1); `amendments/` (R5), the board,
  the GitHub adapter and the budget stay specified ahead of their
  build. Section 7.9's rulings gain the Go head's qualifier of Step
  R3-7, its paths no longer say the check's clone is `/work`, and
  section 7.6's step 2 names a worker's question among the runs that
  keep their ref. Decisions
  [134](swarm/decisions/0134-a-products-test-services-beside-its-check.md),
  [158](swarm/decisions/0158-the-jobs-workspace-is-the-memory.md),
  [159](swarm/decisions/0159-one-folder-per-run-and-the-builder-plan.md),
  [160](swarm/decisions/0160-one-declared-check-environment.md),
  [161](swarm/decisions/0161-a-check-failure-classified-before-anyone-sees-it.md)
  and [162](swarm/decisions/0162-ci-builds-the-declared-image.md) name
  the steps that built them. No decision of its own: no choice is made.
- 2026-11-21, Step R3-3c of the plan implementing the 2026-11-19 swarm
  roadmap, on the maintainer's answers to its Open Questions 29, 32 and
  35: the test services file (`repos[].services`, else
  `compose.test.yaml`) protected by default, as the declared Dockerfile
  is, and named in the Task texts' protected paths; a slot's `--env
  <home>=/credential` after every declared `--env`, and a member's
  declared line naming its `env` credential's variable left out;
  `dockerfile` and `services` refusing a NUL, a `.git` element and a
  space at either end; a dependency step's failure classified by a
  signature whatever the head. Sections 3, 7.2, 7.4, 7.6 (step 3) and
  7.9. Decision
  [166](swarm/decisions/0166-the-services-file-protected-and-the-credential-last.md),
  amending decisions
  [160](swarm/decisions/0160-one-declared-check-environment.md),
  [161](swarm/decisions/0161-a-check-failure-classified-before-anyone-sees-it.md)
  and [162](swarm/decisions/0162-ci-builds-the-declared-image.md).
- 2026-11-21, Step E1 of the plan implementing the 2026-11-19 swarm
  roadmap: the system prompt's stable blocks first. `system.md` writes
  the persona, its prompt parts, praxis and the testing skill, the
  repository notes and the resources first, and the run's header line
  and `# Task` last, so that runs of one role share a prefix a
  provider's prompt cache keeps; `config_hash` reads from `# Persona`
  to the first block after the prompt parts, so one persona with its
  parts hashes alike in either order. Sections 4 (The swarm's notes
  about a repository), 7.3 (the layout, The order) and 7.6 (The run's
  record). Decision
  [167](swarm/decisions/0167-the-system-prompt-stable-blocks-first.md),
  amending decisions [102](swarm/decisions/0102-repos-name-md.md),
  [108](swarm/decisions/0108-the-swarms-notes-about-each-repository-and-drift-as-built.md),
  [116](swarm/decisions/0116-the-architect-and-the-worker-read-daisys-praxis-and-testing.md),
  [164](swarm/decisions/0164-prompt-parts-added-to-a-roles-prompt.md)
  and [165](swarm/decisions/0165-a-configuration-hash.md).
- 2026-11-21, Step E2 of the plan implementing the 2026-11-19 swarm
  roadmap: a delta review. A reviewer's round r > 1 on a head with
  commits since its own last review is given the previous round's
  findings (the slice's latest round's file in the job's workspace) and
  the change since that review as its task, the whole change readable
  but not the task; a finding on code unchanged since is non-blocking;
  a reviewer with no review of its own on the pull request reviews the
  whole change; round 1 and the rounds on an unchanged head are as
  before. The scorecard reads its before and after as two windows.
  Sections 7.4 (the reviewers' row) and 11 (The scorecard). Decision
  [168](swarm/decisions/0168-a-delta-review.md), amending decisions
  [15](swarm/decisions/0015-severity-per-finding.md),
  [63](swarm/decisions/0063-the-adversarys-fail-against-base-check-is-bounded.md)
  and
  [163](swarm/decisions/0163-the-sympathetic-reviewer-after-the-adversary-approves.md).
- 2026-11-21, Step E2b of the plan implementing the 2026-11-19 swarm
  roadmap: the default reviewer personas follow the delta Task. The
  adversary's and the sympathetic reviewer's defaults no longer say
  "Read the whole diff": each reads the diff the Task names, the whole
  diff in round 1 and what the Task names in a later round, so neither
  contradicts a delta round's Task; their sha256 appended to the table
  of earlier defaults. Sections 4 (the adversary's and the sympathetic
  reviewer's default prompts) and 11 (The table of earlier defaults).
  Decision [168](swarm/decisions/0168-a-delta-review.md), amended with
  an As built note.
- 2026-11-21, Step C2–C3 of the plan implementing the 2026-11-19 swarm
  roadmap: `swarm status`'s reverted line reads `(fetch time unknown)`
  when a mirror holding `base` was read but none records a fetch, where
  it said `(no mirror fetched)`; its document's cost sums are rounded to
  six decimals; `swarm list` and `swarm connect` say "any positional
  argument". Section 11 (`daisy swarm status`'s Throughput, `daisy swarm
  list`, `daisy swarm connect`). Decision
  [141](swarm/decisions/0141-reverts-counted-from-the-mirrors.md),
  amended with an As built note.
- 2026-11-21, Step 12 of the plan implementing the 2026-11-19 swarm
  roadmap (R2, integration control; specified ahead of its build, to
  be built at Step 13): machine-read `Depends:` declarations (the
  proposal's, the plan's, a maintainer's comment; the newest stands)
  and several `Blocked by` comments together hold a job while a
  dependency is open; a slice's `Touches:` patterns hold its first
  build while it overlaps an open slice of another issue, unless
  marked `Safe with:`; a merge queue per repository checks the first
  approved head without a passed merge check on `base` as it is now,
  on the commit that merges it, and neither the merge step nor `swarm
  merge` merges a head without one, a failure going back to the worker
  with its log (T32) and a conflict found there at once (T25); `base`
  is checked when its tree is one no check passed, and while it is red
  the repository's worker launches and queue wait but for the fix, the
  breaking job's when open, else the owner's next proposal; the owner
  proposes dependent work rather than holding its batch, its hold read
  as issue 0's dependencies; a required context not yet reported holds
  a merge. Sections 1, 2, 5, 6, 7.3, 7.4, 7.7, 7.8, 7.9, 10, 11 and 12,
  and the new 12.1. Decision
  [169](swarm/decisions/0169-integration-control.md), amending
  decisions [51](swarm/decisions/0051-dependency-order-and-pins.md),
  [81](swarm/decisions/0081-conflicts-handled-by-the-swarm.md),
  [93](swarm/decisions/0093-the-collection-and-the-merge-step.md) and
  [135](swarm/decisions/0135-the-owners-hold-released-by-its-issues.md).
- 2026-11-21, Step 12 of the plan implementing the 2026-11-19 swarm
  roadmap (R4, capacity; specified ahead of its build, to be built at
  Step 13, the fallback and the adoption first): a persona's
  `fallback` models, tried in order, a capacity fault making its model
  unavailable on its credential until the reset its message states or
  the bare `swarm resume`, pausing nothing, each change noted on the
  issue and notified, and the collection reading a run with its own
  persona's adapter; every container detached, writing its output and
  exit status to its run's directory, so any `apply` collects any run
  with its cost and fault and enforces its recorded deadline (F36), a
  restart re-executing with members running and a new build told by
  its change time too (L345); `slots` per persona and a `Capacity:`
  block naming the bottleneck; `keys.<provider>.tokens` pacing each
  credential over the last day and week; the deadline told, a warning
  file at 80% of `timeout`, the partial output posted with a timeout
  (F37). Sections 1, 2, 3, 5, 6, 7.2, 7.4, 7.7, 8, 9 and 11, and the
  new 7.10. Decision [170](swarm/decisions/0170-capacity.md), amending
  decisions [5](swarm/decisions/0005-budget.md),
  [24](swarm/decisions/0024-the-container.md),
  [26](swarm/decisions/0026-the-wall-clock-is-applys-not-stop-timeouts.md),
  [62](swarm/decisions/0062-what-step-6-built-of-the-lifecycle-and-the-verbs.md),
  [89](swarm/decisions/0089-the-pass-and-the-gates.md) and
  [137](swarm/decisions/0137-restart-onto-a-new-build-when-idle.md).
- 2026-11-21, the verdict fix of the plan implementing the 2026-11-19
  swarm roadmap: every role's `system.md` holds its output contract,
  `# Output`, right after `# Persona` and before the prompt parts, one
  text per role naming the first lines its output may or must begin
  with (a reviewer's literal `VERDICT:` lines with a template); a Task
  whose first line is required, a reviewer's and a question's, ends
  with a reminder restating it, and the reviewer's verdict sentence
  leaves its row's text; `config_hash` leaves the block out. A smaller
  model's reviews had lost their verdict line under Step E1's order.
  Sections 7.3, 7.4, 7.6 and 11. Decision
  [167](swarm/decisions/0167-the-system-prompt-stable-blocks-first.md)
- 2026-11-21, Step R4-A of the plan implementing the 2026-11-19 swarm
  roadmap (R4, capacity; the fallback): `personas.<role>.fallback`
  built as a field, each entry the persona's four fields, validated as
  the persona's, its provider with a credential in `keys`, no entry
  twice; written after `co_author` on the persona's line and kept by
  `init`. An entry's `slots` waits for R4-C. Section 3 (the field, the
  error table). Decision [170](swarm/decisions/0170-capacity.md)
  amended.
- 2026-11-21, Step R4-A of the plan implementing the 2026-11-19 swarm
  roadmap (R4; the fallback): a collected run's output read with the
  adapter of its own `persona` line, not the role's persona as
  `swarm.json` has it at the collection. Section 7.10 (The fallback).
  Decision [170](swarm/decisions/0170-capacity.md) amended.
- 2026-11-21, Step R4-A of the plan implementing the 2026-11-19 swarm
  roadmap (R4; the fallback): `Reset` built per provider adapter, the
  reset a capacity fault's message states: `codex`'s `try again at
  <h>:<mm> <AM|PM>` as the first such time after the fault in UTC,
  none for `claude` and `grok`. Section 9. Decision
  [170](swarm/decisions/0170-capacity.md) amended.
- 2026-11-21, Step R4-A of the plan implementing the 2026-11-19 swarm
  roadmap (R4; the fallback): `<state>/capacity` built, a capacity
  fault's line written at its collection until its stated reset or the
  bare `swarm resume`; at a launch the first of the persona and its
  fallback whose model is available on a free credential, the role
  waiting with none, nothing made or posted; a capacity fault counting
  towards no pause; a line removed at the first pass after its reset,
  and the file by the bare `resume`; a fallback's commits carrying its
  `co_author`. Sections 2 (the tree), 7.7, 7.10 and 11 (pause and
  resume). Decision [170](swarm/decisions/0170-capacity.md) amended.
- 2026-11-21, Step R4-A of the plan implementing the 2026-11-19 swarm
  roadmap (R4; the fallback): the `kind=capacity` note on the issue when
  a run launches on another entry than the role's newest run there, and
  when it is back on the persona, no counted entry; notify telling each
  line of `<state>/capacity` once. A batch run's journal line waits.
  Sections 2 (`<state>/notified`), 5 (Notify), 6 (Trust), 7.10 and 11
  (apply's stdout). Decision [170](swarm/decisions/0170-capacity.md)
  amended.
- 2026-11-21, Step T5a–d of the plan implementing the 2026-11-19 swarm
  roadmap: a run's row has no `cost_estimated` when its `cost` line's
  `cached <c>` is over its `<in>`. Section 7.6 (The run's record).
  Decision [140](swarm/decisions/0140-every-providers-input-count-holds-its-cache.md),
  amended with an As built note.
- 2026-11-22, Step R4-B of the plan implementing the 2026-11-19 swarm
  roadmap (R4, adoption): a member runs `docker run --detach`, its
  `sh` writing the CLI's standard output and error to `/job/io/stdout`
  and `/job/io/stderr` beside `/job/io/exit`, and its collection reads
  its status, streams, cost and fault from those files; a refused
  `docker run --detach` is the launch's fault, a container gone with no
  status abandoned; a slot's lock held through the launch alone.
  Sections 7.2, 7.10 and 8. Decision
  [170](swarm/decisions/0170-capacity.md) amended.
- 2026-11-22, Step R4-B of the plan implementing the 2026-11-19 swarm
  roadmap (R4, adoption): the check and the dependency step run
  detached, their `sh` writing their output and status to
  `<run>/step/` (`out`, `exit`), emptied before each step and renamed
  to `check.out` or `deps.out` at its end; a step gone with its status
  written is collected from those files by any `apply`, one with none
  started again. Sections 7.2 and 7.10. Decision
  [170](swarm/decisions/0170-capacity.md) amended.
- 2026-11-22, Step R4-B of the plan implementing the 2026-11-19 swarm
  roadmap (R4, adoption): a launch records the run's `deadline`, a
  step's `<step> deadline`; any `apply` collects a detached run whose
  container is gone from its files, `exited <code>` with its cost and
  its provider's fault, its own process's while its wait has not
  returned among them (request F36). Sections 7.7 and 7.10. Decision
  [170](swarm/decisions/0170-capacity.md) amended.
- 2026-11-22, Step R4-B of the plan implementing the 2026-11-19 swarm
  roadmap (R4, adoption): the run's recorded deadline enforced whoever
  launched it: a container found running past it stopped and its run a
  timeout, one before it held by the loop with a timer at it. Sections
  7.7 and 7.10. Decision [170](swarm/decisions/0170-capacity.md)
  amended.
- 2026-11-22, Step R4-B of the plan implementing the 2026-11-19 swarm
  roadmap (R4, adoption): the restart re-executes at the end of the
  first pass after the request, or of the pass after the one that saw a
  new build, its members running on and held by the process it starts;
  the build told by its change time too (L345); a signal ends the loop
  at its pass's end without a drain, exit 0, the exit 130 of a second
  signal gone. Sections 7.10 and 11. Decision
  [170](swarm/decisions/0170-capacity.md) amended.
- 2026-11-22, Step R6-design of the plan implementing the 2026-11-19
  swarm roadmap (R6, review by risk, with an end, and the parts of R5
  it needs; specified ahead of its build, to be built once the
  maintainer has ruled on its open questions): a review finding blocks
  only when the slice's claim fails, a test the plan names is missing
  or fails, a defect in the changed code meets a realistic user or
  input, or the revision introduced a fault, a fixed list in Daisy's
  Task text for both reviewers, each blocking finding naming its kind
  and its reproduction; "realistic" read from each repository's notes'
  `## Trust boundaries`, which `init` seeds; the plan's Test Plan names
  each test's probe and each slice its unsupported setups, the worker
  reporting each probe and the adversary checking them, a probe of its
  own non-blocking; a finding closed by its own reproduction, and from
  round 2 only an unclosed one, the revision's fault or a failing claim
  blocking; every non-blocking finding of a merged pull request filed
  once by Daisy as a follow-up issue (`swarm:follow-up`, the eighteenth
  label), which the owner's batch triages (`# Follow-ups`, `Follow-up:`
  and `Triaged:` lines); the scorecard's escapes (reverts, `Defect in:`
  issues and hand fixes) and follow-up items proved real, by
  configuration; the round limit's outcome left to the maintainer; the
  worker's Task naming Daisy's gates; the default reviewer, architect
  and worker prompts' designed text. Sections 4, 5, 6, 7.3, 7.4, 7.5,
  7.6, 11 and 12, and the new 7.11. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), amending
  decisions [15](swarm/decisions/0015-severity-per-finding.md),
  [63](swarm/decisions/0063-the-adversarys-fail-against-base-check-is-bounded.md),
  [144](swarm/decisions/0144-a-recurring-finding-asks-for-a-check.md),
  [156](swarm/decisions/0156-a-scorecard-per-merged-slice.md) and
  [168](swarm/decisions/0168-a-delta-review.md).
- 2026-11-22, Step R6-A of the plan implementing the 2026-11-19 swarm
  roadmap, the rulings: the maintainer ruled on decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md)'s open
  questions. At the round limit the architect judges the outstanding
  blocking findings and writes `OVERRULED:` when none qualifies, the
  sympathetic reviewer then reviewing and its approval going to the
  merge gate, `max_rounds` unchanged (specified ahead of its build, Step
  R6-D); the live swarm's token rules go into the default reviewer
  prompts; the default trust boundaries stand; `swarm merge` files the
  follow-up under the maintainer's token (specified ahead of its build,
  Step R6-B). Sections 5, 7.11 and 12 and the same decision.
- 2026-11-22, Step R6-A of the plan implementing the 2026-11-19 swarm
  roadmap, the blocking sentence (rules 1, 2 and 4 of section 7.11):
  every reviewer's Task lists the four kinds of finding that block, as
  paragraphs of its own, "realistic" read by the repository notes'
  `## Trust boundaries` or, with none, the default; the reviewers'
  `# Output` block defers to the list and its template shows a blocking
  finding's kind and reproduction. Sections 4, 6, 7.4 and 7.11. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-22, Step R6-A of the plan implementing the 2026-11-19 swarm
  roadmap, convergence (rule 5 of section 7.11): after the list of what
  blocks, a delta round's Task says when a blocking finding is closed and
  that only an unclosed one, the revision's fault or a claim the head
  fails blocks, in place of its sentence on code changed since the
  previous head; a reviewer whose first review falls in a later round is
  bound by the round; the worker answers, rather than fixes, a blocking
  finding that is none of the kinds. Sections 6, 7.4 and 7.11. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-22, Step R6-A of the plan implementing the 2026-11-19 swarm
  roadmap, the probes and the unsupported setups (rules 3 and 4 of
  section 7.11): each architect text that asks for a plan asks for a
  `Probe:` line for each claim's test and an `Unsupported:` line in each
  slice's item, which the builder plan's slice section holds; the worker
  applies and reports each named probe; the adversary checks them after
  its rollback, a probe of its own non-blocking. Sections 7.4 and 7.11.
  Decision [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As
  built.
- 2026-11-22, Step R6-A of the plan implementing the 2026-11-19 swarm
  roadmap, Daisy's gates (rule 9 of section 7.11): every worker's Task
  says, after its check sentence, that the declared check and the forge's
  required checks are Daisy's gates, and that a part of them its
  container cannot run is listed, not a stop. Sections 7.4 and 7.11.
  Decision [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As
  built.
- 2026-11-22, Step R6-A of the plan implementing the 2026-11-19 swarm
  roadmap, the defaults (rules 1 to 5 and 9 of section 7.11): the
  architect's, the worker's and the two reviewers' default prompts take
  the rules, and the reviewers' the token rules the maintainer ruled in
  (neither reruns the check; the adversary runs the probes and stops once
  its scope is checked; the sympathetic reviewer reads and runs nothing,
  its Task not offering its clone to build and test in); `swarm init`
  seeds `## Trust boundaries` in the notes it writes. Sections 4, 7.4 and
  7.11. Decision [171](swarm/decisions/0171-review-by-risk-with-an-end.md),
  As built.
- 2026-11-22, Step R6-A of the plan implementing the 2026-11-19 swarm
  roadmap, `Defect in:` (rule 7 of section 7.11): the owner's and the
  QA's batch texts ask a job whose failure is the change of a merged
  swarm pull request to name it on a `Defect in: <name>#<m>` line.
  Sections 7.4 and 7.11. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-22, Step D1–D2 of the plan implementing the 2026-11-19 swarm
  roadmap (wording): a wording-only change adds no decision file
  (Changing this document, and the root's `AGENTS.md`); the `swarm`
  skill's reach is a clone or a folder holding one (section 1), and it
  pushes the clone's files when the maintainer asks (sections 1 and 11).
  Outside this document, the skill pulls (`git pull --ff-only`) before
  that push, logs a rejected draft as rejected and names `swarm list`
  and `swarm metrics`.
- 2026-11-22, Step SF-1 of the plan implementing the 2026-11-19 swarm
  roadmap, the output's file: every role's `# Output` begins with the
  sentence naming `/job/io/out.md`, the file Daisy reads, never the final
  message, and every Task text ends with a line of its own naming it, the
  reminder, where there is one, after it. `prompt_hash` changes for every
  role, no `config_hash`. Sections 7.3 and 7.4. Decision
  [167](swarm/decisions/0167-the-system-prompt-stable-blocks-first.md), As
  built.
- 2026-11-22, Step SF-2 of the plan implementing the 2026-11-19 swarm
  roadmap: `claude`'s `You're out of usage credits. Switch to another
  model …` (429, `model_requires_usage_credits`) is a capacity fault
  naming no reset, no longer `failed: exit 1`. Sections 7.7 and 9.
  Decision [170](swarm/decisions/0170-capacity.md), As built.
- 2026-11-22, Step SF-3 of the plan implementing the 2026-11-19 swarm
  roadmap: `swarm block <n> --by <m>` holds whether placed before or
  after `swarm unblock <n>`: `unblock` posts `Blocked by #<m>` again after
  its comment on the issue when that comment would end an open #m's hold,
  each of the two once however often the verb is run. Sections 5, 6 and
  11. Decision [74](swarm/decisions/0074-the-label-vocabulary.md), As built.
- 2026-11-22, Step SF-4 of the plan implementing the 2026-11-19 swarm
  roadmap: every maintainer's verb says where its comment went, `#<n>
  commented on the issue` or `#<n> commented on pull <name>#<m>` (and
  `comment already posted` likewise), where `unblock` printed `#<n>
  commented` for the issue. Section 11. Decision
  [91](swarm/decisions/0091-the-maintainers-verbs-and-status.md), As
  built.
- 2026-11-22, Step SF-5 of the plan implementing the 2026-11-19 swarm
  roadmap: the interventions' catalogue gains `I15` (a review round limit
  extended), `I16` (a bad verdict) and `I17` (a prompt changed), taken by
  every `--type` and counted by the scorecard; the existing types keep
  their numbers. Section 11. Decision
  [155](swarm/decisions/0155-interventions-recorded.md), As built.
- 2026-11-22, Step S7a of the plan implementing the 2026-11-19 swarm
  roadmap: a gate note's quoted line over 120 characters is cut before a
  Markdown link the cut would split, so a link is kept whole or left out.
  Section 5. Decision
  [76](swarm/decisions/0076-the-gate-note-and-an-answer-without-a-commit.md),
  As built.
- 2026-11-22, Step S7b of the plan implementing the 2026-11-19 swarm
  roadmap: an issue flagged at a gate and then blocked, its newest gate
  note not the blocked gate's, gets the blocked gate's note at the next
  pass, once; before, a block at a gated stage posted none. Section 5.
  Decision
  [76](swarm/decisions/0076-the-gate-note-and-an-answer-without-a-commit.md),
  As built.
- 2026-11-22, Step R6-B of the plan implementing the 2026-11-19 swarm
  roadmap, the label (rule 6 of section 7.11): `swarm:follow-up` is the
  seventh flag and the eighteenth label, which `init` and `apply`
  ensure; `Issues(state, "")` lists every issue in the state, no label
  filter, in the Gitea adapter and the fake. Sections 2, 5, 10 and 11.
  Decision [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As
  built.
- 2026-11-22, Step R6-B of the plan implementing the 2026-11-19 swarm
  roadmap, the follow-up's filing (rule 6 of section 7.11): when an
  issue's pull requests are listed (step 8 of a pass, the merge step,
  `swarm merge` under the maintainer's token), each merged pull request
  the block does not yet show merged has the non-blocking findings of
  every round's swarm reviews, each once and read whole where Daisy cut
  them, filed as one issue carrying `swarm:follow-up`, before the
  block's write, unless one is filed already. Sections 5, 6, 7.8, 7.11,
  11 and 12. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-22, Step R6-B of the plan implementing the 2026-11-19 swarm
  roadmap, the owner's batch reads the follow-ups (rule 6 of section
  7.11): its prompt holds `# Follow-ups` after `# Open swarm issues`,
  each open follow-up's numbered items, and its Task text asks it to
  propose a job of the items worth fixing on a `Follow-up:` line and to
  write `Triaged:` for a follow-up with nothing left to propose. The
  owner's `prompt_hash` changes. Sections 7.3, 7.4, 7.11 and 11.
  Decision [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As
  built.
- 2026-11-22, Step R6-B of the plan implementing the 2026-11-19 swarm
  roadmap, `Triaged:` closes the follow-up (rule 6 of section 7.11): the
  owner's batch's `Triaged: #<f>: <why>` lines, read beside its `Met:`
  lines before a proposal or a `BLOCKED:` line, each close the open
  follow-up they name after the reconciler's note giving the reason and
  the batch's proposals that name it; a line naming any other issue is
  ignored with a warning. Sections 7.5, 7.6, 7.11 and 11. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-22, Step SF2 of the plan implementing the 2026-11-19 swarm
  roadmap: the interventions' catalogue gains `I18` (a persona's model or
  effort changed), taken by every `--type` and counted by the scorecard
  as `I15` to `I17` are. Section 11. Decision
  [155](swarm/decisions/0155-interventions-recorded.md), As built.
- 2026-11-22, Step SF2 of the plan implementing the 2026-11-19 swarm
  roadmap, wording only: the shape of `claude`'s out of usage credits
  result, *to confirm* since Step SF-2, is confirmed on four live
  architect runs' output. Section 9. Decision
  [170](swarm/decisions/0170-capacity.md), As built.
- 2026-11-22, Step SF2 of the plan implementing the 2026-11-19 swarm
  roadmap, wording only: the blocked gate's pull request link sits in
  its `<why>`, not in a quoted line the cut shortens, and the cut's known
  limits are named; `unblock`'s `Blocked by #<m>` posted again is stated
  as built, each of its two comments once however often it is run.
  Sections 5 and 11, docs/cli.md. Decision
  [76](swarm/decisions/0076-the-gate-note-and-an-answer-without-a-commit.md),
  As built.
- 2026-11-23, Step R6-C of the plan implementing the 2026-11-19 swarm
  roadmap, escapes and follow-ups (rule 7 of section 7.11): each
  slice's scorecard line ends with its escapes (its merge reverted, the
  issues whose `Defect in:` line names its pull request, the hand
  commits on `base` whose message does), each made up to `--until`, and
  its follow-up's items with those a merged job's `Follow-up:` line
  names; the summary adds their totals. The scorecard reads every
  issue, open and closed, and the worker runs' `created pull` lines.
  Sections 7.11 and 11, docs/cli.md. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-23, Step R6-C of the plan implementing the 2026-11-19 swarm
  roadmap, the balance by configuration (rule 7 of section 7.11): an
  adversary's or a sympathetic reviewer's line in By configuration adds
  the merged heads its runs approved, the approval standing, with those
  slices' escapes, and the follow-up items its runs raised with those
  proved real. Sections 7.11 and 11, docs/cli.md. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-23, Step R6-C of the plan implementing the 2026-11-19 swarm
  roadmap, blocking findings by kind (rule 7 of section 7.11): the
  scorecard's summary ends with the window's posted reviews' blocking
  findings counted by the kind the word after `[blocking]` names, a line
  naming none `unnamed`. The balance is built; rule 8, the round
  limit's `OVERRULED:`, stays specified ahead of its build. Sections 7.11
  and 11, docs/cli.md. Decisions
  [156](swarm/decisions/0156-a-scorecard-per-merged-slice.md) and
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-23, Step R6-E of the plan implementing the 2026-11-19 swarm
  roadmap, an approval accounts for its slice: the reviewers' `# Output`
  asks an approval for a `[checked] claim <n>:` line per claim of the
  plan's Test Plan and, from round 2, a `[closed]` line per earlier
  blocking finding it judges closed, its approval template showing both;
  every reviewer Task's last line reminds of them. Sections 7.4 and
  7.11. Decision
  [172](swarm/decisions/0172-an-approval-accounts-for-its-slice.md).
- 2026-11-23, Step R6-E of the plan implementing the 2026-11-19 swarm
  roadmap, the check: an approval without the `[checked]` line of a
  claim of the plan's Test Plan is a bad verdict, its reason naming the
  claims missing, the run failed and the issue blocked; no review is
  posted and none rerun. Sections 7.5, 7.7 and 7.11. Decision
  [172](swarm/decisions/0172-an-approval-accounts-for-its-slice.md), As
  built.
- 2026-11-23, Step R6-E of the plan implementing the 2026-11-19 swarm
  roadmap, the kind (the maintainer's ruling A2 of 2026-11-22): a review
  whose `[blocking]` line names no kind of the blocking sentence's list,
  or `unclosed` in round 1, is a bad verdict, its reason quoting the
  line; the scorecard's count and the check read the kind one way.
  Sections 7.5, 7.7 and 7.11. Decision
  [172](swarm/decisions/0172-an-approval-accounts-for-its-slice.md), As
  built.
- 2026-11-23, Step R6-F of the plan implementing the 2026-11-19 swarm
  roadmap, realistic bounds every kind (the maintainer's ruling A4 of
  2026-11-22): the blocking sentence states, in a paragraph of its own
  after the list, that a finding of any kind blocks only when a
  realistic user, input or repository state meets it, as the trust
  boundaries and the slice's `Unsupported:` line define it, and, for
  tooling that builds or tests the code, a case the repository has
  today; a test that survives the probe the plan names for it still
  blocks. The later rounds' narrowing keeps the bound, and the worker
  answers a finding no realistic case meets. Sections 7.4 and 7.11.
  Decision [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As
  built.
- 2026-11-23, Step R6-F of the plan implementing the 2026-11-19 swarm
  roadmap, a documentation-only slice's claim (the maintainer's ruling
  A3 of 2026-11-22): the blocking sentence's claim item adds that on a
  slice that changes only documentation a claim blocks only on a false
  statement the slice added or changed, the reviewer judging whether the
  slice is one; the worker's sentence on a finding that does not block
  and the sympathetic reviewer's default say the same, the default's
  hash appended to the table of earlier defaults, whose counts are
  brought up to date. Sections 4, 7.11 and 11. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-25, Step D7 of the plan implementing the 2026-11-19 swarm
  roadmap, a trigger that cannot launch reads nothing: step (7)'s
  owner's and QA's triggers make no read when the pass launches nothing
  more (no member slot free, the swarm paused or a signal received),
  asked through the function the launch asks; with a slot free they read
  and launch as before. Section 11. Decision
  [173](swarm/decisions/0173-a-trigger-that-cannot-launch-reads-nothing.md),
  As built.
- 2026-11-25, Step FT-1 of the plan implementing the 2026-11-19 swarm
  roadmap, a follow-up titled by its job: a follow-up is filed titled
  `<k> findings left by #<n>: <the issue's title>`, where it was
  `Follow-up: <owner>/<name>#<m> (#<n>, slice <s>)`; its body, label and
  deduplication are unchanged. Section 7.11 and `docs/cli.md`'s `swarm
  merge`. Decision [174](swarm/decisions/0174-the-follow-up-title.md).
- 2026-11-25, Step FT-2 of the plan implementing the 2026-11-19 swarm
  roadmap, the owner's retitle: a `Retitled: #<f>: <title>` line among
  the owner's report lines sets the title of a follow-up it leaves open
  to its findings' cause (`EditTitle`, new on the forge adapter), and
  nothing else of it; one naming an issue not under the run's `# Follow-ups`,
  recorded at the launch in a `follow-ups` result line, or with an empty
  title fails the run. Sections 2, 7.4, 7.5, 7.6, 7.7, 7.11, 10 and 11,
  and `docs/cli.md`'s `apply`. Decision
  [174](swarm/decisions/0174-the-follow-up-title.md), As built.
- 2026-11-26, Step R6-D of the plan implementing the 2026-11-19 swarm
  roadmap, the round limit as ruled (option (b), the maintainer's
  ruling 1 of 2026-11-22): at the limit's first time the architect's
  Task holds the judgement, after the open-slice sentence, asking it to
  judge the last round's blocking findings against rules 1 and 5 and
  to write `OVERRULED: <why, finding by finding>` when none qualifies;
  Daisy then posts it on the pull request, writes no plan and returns
  the issue to `swarm:review-1-in-progress`, T33, new in section 5's
  table and the explorer's, where the adversary's overruled request
  stands as its approval would, so the sympathetic reviewer reviews the
  head and its approval goes to the merge gate as T9's does; the
  follow-up files the overruled findings marked `outstanding at the
  round limit`. Sections 5, 6, 7.4, 7.5, 7.6 and 7.11, and
  `docs/cli.md`'s `swarm merge`. Decision
  [171](swarm/decisions/0171-review-by-risk-with-an-end.md), As built.
- 2026-11-26, Step R4-C of the plan implementing the 2026-11-19 swarm
  roadmap (R4, slots per role, rule 3): `personas.<role>.slots`, on a
  persona or a fallback entry, the slots of its provider it may take,
  in their order, validated against `keys.<provider>.slots`; `status`
  gains its `Capacity:` block after `Personas:`, each role's queue and
  running members and what holds a waiting queue, the lines of
  `<state>/capacity` and the bottleneck, JSON `capacity`. Sections 3, 6,
  7.10, 8 and 11, and `docs/cli.md`'s `swarm status`. Decision
  [170](swarm/decisions/0170-capacity.md), As built.
- 2026-11-26, Step R4-C of the plan implementing the 2026-11-19 swarm
  roadmap (R4, pacing, rule 4): `keys.<provider>.tokens`, `{"per_day",
  "per_week"}`, paces each credential of the provider: one whose runs
  started in the last 24 hours, or 7 days, have reached the limit is
  passed over as an unavailable one is, and `status` names the time it
  frees and each credential's tokens. Sections 1, 3, 6 and 7.10, and
  `docs/cli.md`'s `swarm status`. Decision
  [170](swarm/decisions/0170-capacity.md), As built.
- 2026-11-26, Step R4-C of the plan implementing the 2026-11-19 swarm
  roadmap (R4, the deadline, rule 5, request F37): every Task text's
  closing paragraph names the run's deadline and `/job/io/deadline`;
  at 80% of the run's span the process holding it writes that file and
  records `warned <time>`; a run stopped at its deadline has the output
  it had written posted with its T17 note, after the secret check,
  never read as the role's text. Sections 2, 7.4, 7.7 and 7.10.
  Decision [170](swarm/decisions/0170-capacity.md), As built.
- 2026-11-26, Step R4-C of the plan implementing the 2026-11-19 swarm
  roadmap, the maintainer's rulings on its open questions: a paused
  swarm's waiting queues and bottleneck are held by `swarm paused` in
  `status`'s `Capacity:` block; an entry's `slots` on an `env`
  credential is refused `<provider>'s credential is env, which has no
  slots`. Sections 3 and 7.10. Decision
  [170](swarm/decisions/0170-capacity.md), As built.
- 2026-11-26, Step R4-L of the plan implementing the 2026-11-19 swarm
  roadmap (a role's model by label, the maintainer's request of
  2026-11-26): `personas.<role>.by_label`, an ordered list of entries
  each a fallback entry's fields after a `label`, any label, none empty
  or twice, each field and its `slots` validated as a fallback entry's;
  `init` writes none and keeps the file's; written on the persona's line
  after `fallback`. Section 3. Decision
  [175](swarm/decisions/0175-a-roles-model-by-label.md).
- 2026-11-26, Step R4-L of the plan implementing the 2026-11-19 swarm
  roadmap (a role's model by label): on a run on an issue the first
  `by_label` entry whose label the issue carries heads the role's
  entries, the persona and its `fallback` after it, unchanged; a run
  with no issue never matches; the run's `persona` line, the capacity
  note and the worker's `co_author` follow the chosen entry; `status`
  shows each entry under `Personas:` and as its own `Capacity:` line,
  JSON `personas.<role>.by_label` and `capacity`'s `label`. Sections 1,
  7.10 and 11, and `docs/cli.md`'s `swarm status`. Decision
  [175](swarm/decisions/0175-a-roles-model-by-label.md), As built.
- 2026-11-26, Step R4-L of the plan implementing the 2026-11-19 swarm
  roadmap, the maintainer's ruling on its open question: a `by_label`
  entry with the provider, model and effort of the persona, or of an
  earlier `by_label` entry, is refused, `the same as personas.<role>`
  or `the same as personas.<role>.by_label[<j>]`, as a fallback
  duplicate is; one the same as a `fallback` entry stays valid. Sections
  3 and 7.10. Decision
  [175](swarm/decisions/0175-a-roles-model-by-label.md), As built.
- 2026-11-26, Step R2-A of the plan implementing the 2026-11-19 swarm
  roadmap (dependencies and touches, rules 1, 2 and 5 of section 12.1):
  an issue's newest `Depends:` declaration (the proposal's, the plan's,
  or a maintainer's comment that is the line alone) and every `Blocked
  by #<m>` among the maintainer's newest comments hold its launches and
  its merge while one is open; a declaration is no counted entry; a
  slice's first build waits while its `Touches:` patterns or branch
  files overlap another issue's open slice, unless `Safe with:` names
  it; the owner's hold is released through the same test; `status`'s
  `Held:` names `#<n> blocked by #<m>` per dependency and `#<n> overlaps
  #<m>`, JSON `held.overlaps`; the triage and plan gates' notes gain `-
  Depends:` and `- Touches:`; the owner's batch Task text asks for both
  lines after `Class:` and for dependent work to be proposed, the
  architect's for the plan's `Depends:` and each slice's `Touches:` and
  `Safe with:`. Sections 1, 5, 6, 7.4, 11 and 12.1, and `docs/cli.md`'s
  `swarm status`. Decision
  [169](swarm/decisions/0169-integration-control.md), As built.
- 2026-11-26, Step R2-A of the plan implementing the 2026-11-19 swarm
  roadmap, the maintainer's rulings on its open questions: the default
  owner prompt proposes a job whose dependencies are open, naming them on
  its `Depends:` line, in place of waiting for them to close; an issue
  the owner's batch hold waits on that the forge does not hold holds
  nothing, as a dependency's does not, with the warning `not waited on`.
  Sections 1, 4 and 6. Decision
  [169](swarm/decisions/0169-integration-control.md), As built.
- 2026-11-26, Step CA of the plan implementing the 2026-11-19 swarm
  roadmap (the maintainer's ruling "Co-author should always match the
  model"): `co_author` leaves swarm.json, on the persona, a fallback
  entry and a `by_label` entry alike; a run's `Co-Authored-By:` value is
  derived from the provider and the model it launched with, `<model>
  <noreply@anthropic.com>`, `<noreply@openai.com>` or `<noreply@x.ai>`,
  stamped by the hook on every role's clone, with no `""` and no way to
  turn it off; a file still holding `co_author` is refused naming the
  field, and `init` drops it, keeping the rest; the wizard's co-author
  questions (22–26) are retired, the others keeping their numbers; the
  collection's unstamped check and the notes' commit derive it from the
  run's `persona` line. Sections 3, 7.1, 7.6, 7.10, 9 and 11, and
  `docs/cli.md`'s `swarm init`. Decision
  [176](swarm/decisions/0176-the-co-author-is-the-models.md), amending
  decisions [52](swarm/decisions/0052-every-role-names-its-model-the-commits-author-and-co-author.md)
  and [175](swarm/decisions/0175-a-roles-model-by-label.md).
- 2026-11-26, Step CA of the plan implementing the 2026-11-19 swarm
  roadmap, the maintainer's ruling on its open question: `init`, dropping
  the retired `co_author`, says so before `✓ Wrote swarm.json`, `✓
  Dropped co_author from <k> entries: the co-author trailer is now the
  model's`. Section 11's `init` stdout and `docs/cli.md`'s. Decision
  [176](swarm/decisions/0176-the-co-author-is-the-models.md), As built.
- 2026-11-26, Step R2-B of the plan implementing the 2026-11-19 swarm
  roadmap (the merge queue, rule 3 of section 12.1, with L346 and the
  maintainer's ruling 5 of 2026-11-21): each repository's approved heads
  are checked one at a time, in the order their pairs completed, on the
  commit that merges the head into `base` (`refs/swarm-merge/<n>-<s>`,
  the run `<n>-merge-<k>`); the merge step and `swarm merge` merge no
  head without a passed merge check on `base` as it is now, a merge
  ending its repository's merges for the pass; a conflict the queue's
  merge finds is T25 at once, a failed check T32 with its log at
  `checks/<merge sha>.log`, a check failing in the environment again on
  its rerun a block; a head the mirror cannot read is blocked with a
  note, never merged unchecked; a required context that has not reported
  holds the merge (`RequiresChecks` returns the contexts). Sections 2,
  5, 7.7, 10, 12 and 12.1. Decision
  [169](swarm/decisions/0169-integration-control.md), As built.
- 2026-11-27, Step R2-B of the plan implementing the 2026-11-19 swarm
  roadmap, the maintainer's ruling on its open question: a merge check
  that failed on a `base` that has moved since is checked again on
  `base` as it is now before the head goes back to the worker; a
  failure on `base` as it is now is still T32 at once, a conflict T25.
  Section 12.1. Decision
  [169](swarm/decisions/0169-integration-control.md), As built.
- 2026-11-27, Step R2-C of the plan implementing the 2026-11-19 swarm
  roadmap (the red main, rule 4 of section 12.1): a commit of `base`
  whose tree no check has passed or failed is checked by a main check,
  `0-main-<k>`, a merge check recording its merge's tree so the forge's
  merge of a passed head is green with no run; while `base` is red no
  worker launches on its repository and its queue checks no head but the
  fix's, the queue waiting for a green `base`; the fix goes to the
  breaking job's worker when its issue is open, else to the owner's next
  proposal (`# Main`, `Fixes: <name>`, `owner-held` released); a main
  check failing in the environment writes `<state>/main-unchecked`,
  which the bare `resume` removes; red and green are journalled;
  `status` gains the `Queue <name>:` line and `main red`/`main
  unchecked`, which notify tells, and the merge gate note `- Merge
  check:`. Sections 1, 2, 5, 6, 7.3, 7.4, 7.7, 7.8, 7.9, 11 and 12.1;
  `docs/cli.md`'s status and resume. Decision
  [169](swarm/decisions/0169-integration-control.md), As built.
- 2026-11-27, Step R2-C of the plan implementing the 2026-11-19 swarm
  roadmap, the maintainer's rulings on its open questions: the main
  checks' fetch, once a pass while a member slot is free, stands as
  built; with no green commit known, the owner's `# Main` lists the
  merges back to the oldest commit any check recorded, and with none the
  newest merge alone. Section 12.1. Decision
  [169](swarm/decisions/0169-integration-control.md), As built.
- 2026-11-27, Step T8a of the plan implementing the 2026-11-19 swarm
  roadmap: `forge.Fake` saves its state to a JSON file (`Save`) and
  loads it back (`LoadFake`, given the account and the clock). Section
  10, `forge.Fake`. Decision
  [177](swarm/decisions/0177-the-fake-forges-file.md).
- 2026-11-27, Step T8b of the plan implementing the 2026-11-19 swarm
  roadmap: `DAISY_FORGE=<file>`, a test seam, makes every swarm verb
  open the fake forge the file holds (`forge.OpenFile`, the file's
  `tokens` giving each token's account) in place of the declared one.
  Section 10, The fake forge's file; `docs/cli.md`'s Environment read.
  Decision [177](swarm/decisions/0177-the-fake-forges-file.md).
- 2026-11-27, Step T8c of the plan implementing the 2026-11-19 swarm
  roadmap: the acceptance cases for `swarm init` and `swarm status`
  over the fake forge's file, and `fixture_fake_forge`. Section 10, The
  fake forge's file, As built. Decision
  [177](swarm/decisions/0177-the-fake-forges-file.md).
- 2026-11-27, Step T8b of the plan implementing the 2026-11-19 swarm
  roadmap, the maintainer's ruling on its open question: a fake forge's
  file whose `forge.repo` or declared repositories are not `swarm.json`'s
  is refused, naming both. Section 10, The fake forge's file;
  `docs/cli.md`'s Environment read. Decision
  [177](swarm/decisions/0177-the-fake-forges-file.md), As built.
- 2026-11-27, Step S1 of the plan implementing the 2026-11-19 swarm
  roadmap (L297): the default architect prompt names a slice's documents
  in its item under `## Steps` and in the Test Plan claims it meets,
  where it named "each slice's acceptance claims", which the Task text
  puts in the plan-wide `## Test Plan`; its sha256 appended to the table
  of earlier defaults. Sections 4 and 11. Wording; no decision.
- 2026-11-27, Step S2 of the plan implementing the 2026-11-19 swarm
  roadmap (L328, L375): `swarm prompt build`'s commit names the commit
  the binary was built from, ` (modified)` kept as `daisy version`
  prints it, not `$DAISY_ROOT`'s `HEAD`, and its clone's origin is
  checked before any write; `swarm init` and `swarm prompt build` take
  `--co-author` (else `DAISY_CO_AUTHOR`) and end their commit with that
  trailer. Section 11's `init` and `prompt build`; `docs/cli.md`'s
  Commits. Decision
  [178](swarm/decisions/0178-the-maintainers-swarm-commits.md).
- 2026-11-27, Step S3 of the plan implementing the 2026-11-19 swarm
  roadmap (L347, L349): the worker's Task text at
  `swarm:review-3-changes-requested` names, among the entries to answer,
  Daisy's note naming the checks that failed on the approved head (T26);
  the owner's at `swarm:proposal-posted` names the protected paths, the
  list its default prompt says the Task names. Section 7.4. Wording; no
  decision.
