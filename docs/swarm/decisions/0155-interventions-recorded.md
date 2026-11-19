# 0155. Interventions recorded: `--type` on the maintainer's verbs, and `swarm intervene`

From Step 6 of the plan implementing the 2026-11-19 swarm roadmap (R0, measure first, 2026-11-19).

The maintainer's ruling (2026-11-19): the maintainer's interventions are
recorded by an optional `--type I<n>` on the maintainer's verbs, and a
`daisy swarm intervene --type I<n> <note...>` verb for an act no verb
makes, the types the catalogue's I1 to I14. This replaces the
catalogue's hand counts. What follows is how (docs/swarm.md section 11,
The interventions' record).

**Which verbs take `--type`.** Every verb that changes the swarm's
state on the maintainer's behalf: `triage`, `accept`, `class`,
`approve`, `merge`, `unblock`, `block`, `replan`, `close`, `pause` and
`resume` (on an issue or the whole swarm) and `restart`, from the
terminal and from a comment command. Not `status`, `list`, `journal`,
`connect` (nothing written; `journal --mark` moves only the reader's
mark) nor `metrics` (the home's records, not the swarm's state); not
`init` (it makes a swarm no one has stepped into yet), not `apply` (the
swarm itself), and not `prompt build`, which brings the clone's prompts
to the installed build's defaults, an upgrade's act: when one is run to
step in, it is recorded as any edit of the clone is, by `intervene`.
Taking it there too was the alternative; it would put a flag on a verb
with `--force` and `--dry-run` for a case `intervene` already covers.

**The row is a new `kind`, no new column.** `metrics.go`'s rule is that
a column is only ever added at the end; the step asked for a new kind
value with the type in an existing column first. The row is of kind
`intervention`, its type in `cause` and the verb in `outcome`, `role`
`maintainer`, `step_or_run` `<n>-intervention-<k>` and `notes` the note
and `#<n>`. `cause` is the column that already says why a record exists
(a stop's cause), and the kind keeps an intervention's type from being
counted among the stops' causes. The alternatives: a `type` column (a
31st, which every reader and an older header would then carry for one
kind's rows) and the type in `outcome` (what the act was, which the
verb's name says). `daisy metrics` groups the rows by kind as it does
any kind; `--swarm` still takes only a swarm's runs.

**The types are fixed in code, `I1` to `I14`.** As section 7.5's causes
are a fixed vocabulary, the range is a constant and a type outside it
is usage (exit 2) before anything is read. Read from somewhere was the
alternative: the catalogue is a document of the maintainer's home, not
of the root and not on every host that runs a swarm, so a verb's
validation would depend on a private file; and a list in `swarm.json`
would make each swarm its own catalogue, its counts not comparable with
another's, where the catalogue is the maintainer's one. The cost: a new
type, which only the maintainer adds, is a change of Daisy. The
catalogue's short names are in section 11's table, not in the code,
which needs only the range.

**The record is made after the verb's writes, the home checked
before.** Decision [154](0154-daisys-own-stops-carry-a-cause.md) writes
`--cause`'s row first because a retag is idempotent: a run again finds
the cause written and writes nothing. An intervention's record is not
idempotent, each one a new row, so it is made only when the verb exits
0 (an answer that the issue is already as asked included): a refused
verb records nothing, and a verb run again after a refused forge write
records once, at the run that completes. A missing or unusable home is
refused before the forge is read, so the usual failure writes nothing;
a record refused after the writes (the lock held past 30 seconds) is an
error naming `swarm intervene` to record it by hand.

**`<k>` counts the issue's interventions, under the metrics lock.**
The run is `<n>-intervention-<k>`, `<k>` one more than the highest that
the CSV's rows of the kind and the journal as this machine holds it
name for `<n>`, found while the lock is held, so two verbs recording at
once take two numbers. A time in the name (as a restart's
`0-restart-<time>`) was the alternative: two records in one second
would share a key, and both the CSV and the journal queue drop a second
row or line of one key, silently.

**The journal queue takes a lock.** A verb queues its line in
`<state>/journal` from its own process, beside a running `apply` that
appends to the same file and, after its push, removed it whole: a line
appended between the push's read and that removal was lost, and two
read-then-write appends lost one of the two. Every rewrite of the file
now takes `<state>/journal.lock` (waited for as the metrics lock is),
and the push drops only the lines it pushed, keeping a line queued
since. Leaving the journal to `apply` (a second queue that the pass
merges) was the alternative, a second file and a second reader for one
queue. Test: `TestSwarmJournalKeepsALineQueuedDuringItsPush`.

**`intervene` takes no issue**, as the ruling writes it: its record is
on issue 0, `0-intervention-<k>`, and its note may name the issue. The
record names no login: the maintainer's verbs act under the one
maintainer's token, and a comment command's author is in its own
journal line.

Tests: `TestSwarmVerbRecordsItsIntervention`, `TestSwarmIntervene`,
`TestSwarmInterventionRefused`,
`TestSwarmApplyCommandRecordsItsIntervention`,
`TestSwarmJournalKeepsALineQueuedDuringItsPush`,
`TestIsInterventionType`.
