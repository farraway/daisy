# 0158. The job's workspace is the memory

From Step 9 of the plan implementing the 2026-11-19 swarm roadmap (R1, the job's workspace is the memory, 2026-11-19). Specified ahead of its build: `docs/swarm.md` section 7.8, with the changes it makes marked in sections 1, 2, 7.1 to 7.6, 9 and 11.

**The principle, amended by the maintainer's ruling of 2026-11-19:**
Members have no memory, but each job has a workspace Daisy keeps, and
the forge thread is its human record: its posts may be cut, because no
member reads them back to do its work. Section 1 said: "Members have no
memory: each run is a fresh container given its thread so far; the
threads are the memory." Text on the forge was a lossy, unbounded
memory: a plan cut at 60 000 bytes (#102, #143, #144; decisions 148 to
150 patched it), a blocked worker's commits left in the mirror's run
refs while the next run started from base (five times on 2026-11-18,
each answered by a patch published by hand), a CI log and the other
open branches a member could not read, and every prompt growing with
the history (the plan inline at 40 to 80 KB). Whatever a later member
acts on is now a file of the job's workspace, whole, which the prompt
points into, and Daisy reads the same files.

**Where and who.** `<state>/jobs/<n>/`, one directory per issue, written
by `apply` alone at the collection of the issue's runs, each file whole
by a rename, kept after the issue closes, pruned by the maintainer only.
Alternatives: a branch per job in the swarm repository (pushed, so
visible on the forge and kept off the machine, but every member text
would be pushed beside the issue thread, history would grow with every
round, and the forge is what the ruling moves the memory off); the runs'
own directories (per run, where a later member needs the job's state
across runs). The open branches are not the job's memory, since other
jobs' pushes and merges change them and no collection of this job sees
them, so `branches.md` is written into the run's own io at its launch
rather than into the workspace at every launch. The CI job's log is
later work: no forge call reads it (section 10's `Status` gives a
context's state and URL), and R3's one declared environment makes
Daisy's check the CI's.

**The files**, beyond the six the roadmap names (`plan.md`, `slice.md`,
`kept`, `checks/<sha>.log`, `rounds/<r>.md`, `branches.md`): the slice's
files sit under `slices/<s>/`, since a job's slices are built one after
another and each keeps its own kept commit and rounds; amendments have
their slot, `amendments/<a>.md`, applied after the plan in the order
collected, R5's rule deciding what an amendment is; and `texts/` holds
every member text the job posted, whole. The roadmap's six leave the
worker's report and the answers on the forge alone, where a reviewer
checks the report's claims and `Not established:` list; with `texts/`
no member reads any post, which is what lets the posts be cut, and
decision 148's copies and 149's reads of `out.md` have one
replacement. A round's file is written when the round completes, the
pair's two reviews with it, so the reviewer of a round never finds its
pair's review of it, the independence section 7.3 gives the current
round.

**The mount:** the workspace itself, read-only at `/work/.job`. A copy
per run (kept for audit, immune to a write during the run) was the
alternative; Daisy writes only whole files by rename, the one pair of
runs on an issue at once is kept apart by the round's timing above, and
a copy per launch would repeat the plan and every text for each run.
It is the one mount that is not the run's own, so section 8's mount
principle names it: read-only, written by Daisy alone with text that
passed the secret check.

**The run's folder and `grok`.** The maintainer's ruling of 2026-11-19
puts the resources, the notes and the job's workspace outside `/work`
(the checkout), never under it, in a layout `grok`'s confinement allows
(decision 87: a headless `grok` session works only in its working
directory). Chosen: `/work` becomes a folder holding the clones for
every role, each checkout `/work/<name>` as the owner's and the
architect's are already, and the mounts beside the checkouts and in
none; a `Confined` CLI starts in `/work`, every other in its checkout
with the mounts given by `--add-dir`. This reads the ruling's `/work` as
the checkout it names: the checkout moves down a level instead of the
mounts out of `/work`, so the mounts keep their paths, nothing is under
any checkout (intervention I14), R3 reuses the layout with nothing more,
and `grok`'s layout is the one its reviewers have run in since decision
87, a working directory holding dot-folders. Alternatives: the mounts at
`/job`, `/resources` and `/notes` with `grok` started in `/` (the one
folder holding them all beside a checkout at `/work`: untried, its
session then trusting the container's whole tree, and nothing under `/`
writable for a session's own files); a symbolic link in the checkout to
each mount (a path under the checkout again, and a confined session may
refuse a link leading out of it); the bodies inline for a confined
provider alone (two prompts, one memory per provider). The cost: a
`grok` worker or reviewer no longer reads the repository's own
instructions from its working directory, and every path under `/work`
in the Task texts and a swarm's persona prompts and notes moves under
`/work/<name>`, where the architect's plans already name files.

**The start.** The maintainer's ruling of 2026-11-19: the next worker
run of a slice starts from the last run's kept commit unless the
architect's answer or the maintainer's unblock says start from base;
nothing discards kept work automatically. Chosen: the line `Start: base`
in the architect's `ANSWER:` text, or in the maintainer's counted
comment since the kept run (the one `swarm unblock` posts, or the
maintainer's word that lifts the block), read at the next worker run's
launch, a slice whose pull request is open then continuing from the
branch's head without the kept commits not pushed, since the swarm
never forces a push; `kept` is written only at collection and never cleared, the run
ref stays, and a branch moved past the kept commit by the maintainer's
push starts from the branch's head, since the swarm never forces a
push. Alternatives: a flag on `swarm unblock` writing the reset into
the workspace (a verb writing Daisy's memory outside a collection, and
the maintainer's word without the verb would need its own path); the
start from base by default with kept work offered in the prompt (the
behaviour the ruling replaces). Commits that failed the secret check or
changed a protected path are never `kept`, since a run built on them
fails the same way.

**The rollout.** A job in flight when the build arrives has its
workspace built once, by replaying the collection's writes over its
collected runs in `<state>/runs` and the mirrors, at the first pass that
finds it without one; `status` and the verbs build it in memory. A text
whose `out.md` is gone is taken as posted, a cut one with the note the
maintainer chose on 2026-11-18 (`the rest is not available to you`), and
a cut plan so taken blocks its issue, the maintainer's yes of the same
day. Alternative: old jobs keep the thread path until they close, which
would leave two memories half built, the risk the plan names.

**What it makes unnecessary:** decision 148's copies, decision 149's
reads of `out.md`, and the run refs as the only home of kept work, each
retired by the step that switches the members over (decisions 24, 87,
148 and 149 are amended then). Decision 150's attachment and
continuations stay, as the human record's, the maintainer having chosen
them for the forge's readers.

**Amended by decision [159](0159-one-folder-per-run-and-the-builder-plan.md)
(Step 9b of the plan implementing the 2026-11-19 swarm roadmap), before
any of it was built.** The run's folder and the mount above are
replaced: every member run is one folder, `/job` (`<run>/job`, the
run's own), its checkouts at `/job/work/<name>`, its io at `/job/io`,
its notes and resources at `/job/notes` and `/job/resources`, and the
workspace read-only at `/job/memory`, not at `/work/.job`; a confined
CLI starts in `/job`, every other in its checkout with `/job` given by
`--add-dir`. The plan carries no `## For the builder` section: the
architect writes a builder plan of its own, `<run>/io/builder.md`
beside `out.md`, kept in the workspace as `builder.md` and re-issued
whole, from whose `## Slice <s>` sections `slice.md` is cut. `swarm
merge` refuses a job with no workspace yet whose cut plan's `out.md` is
gone, and a revised plan's `Start: base` line resets the start as an
answer's does.

**Amended by the maintainer's answer of 2026-11-20 (Open Question 12 of
the plan implementing the 2026-11-19 swarm roadmap; built at its Step
R1-12).** The rollout replays a worker's report with the check block its
run's records give, the one it was posted with: a run that records no
check result posted `**Check:** none declared.`, and is replayed with it
though its repository has declared a check since. It no longer blocks
its issue, which needed the maintainer to remove the run's `out.md` and
lose its text to the post. Alternative: the block it replaced, which
kept the replay from writing a block the repository no longer declares,
at the cost of a stop whose remedy discards the run's own text. Test:
`TestReplayOfAReportWithNoCheck`.

**Amended by the maintainer's answer of 2026-11-20 (Open Question 16 of
the plan implementing the 2026-11-19 swarm roadmap; built at its Step
R1-13).** `swarm unblock <n> --start base <comment...>`, and the
comment command `/swarm unblock --start base <comment...>`, write the
line `Start: base` for the maintainer, first in the comment the verb
posts and followed by a blank line, so that the start's reset needs no
quoted multi-line argument at the terminal and can be given from a
comment, whose own line `Start: base` would not count (a command
comment is no counted entry). The flag writes into the comment, the
forge record, as before, not into the workspace, the alternative the
start above declined. Alternatives: the line last in the comment
(where a fenced block the comment leaves open would hold it); the
flag's word left out, `--start` alone (the cli's value flags take a
word, and `base` leaves room for no other start the ruling does not
name). Test: `TestSwarmUnblockStartBase`.
