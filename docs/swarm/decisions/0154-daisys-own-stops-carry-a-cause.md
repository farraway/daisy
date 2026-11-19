# 0154. Daisy's own stops carry a cause, and `unblock` retags one

From Step 5 of the plan implementing the 2026-11-19 swarm roadmap (R0, measure first, 2026-11-19).

**A run that ends in a stop Daisy originates records its cause in the
`cause` cell decision [153](0153-a-cause-on-every-member-stop.md)
added, from the same nine words, read from the run's `fault:` or
`failed:` line (sections 7.5 and 7.7).** A swarm fault is the
machine's by section 7.7's definition, so every fault records
`environment`, but a provider's fault whose message names a limit, a
quota or the credit, which records `capacity` (`provider.Capacity`; the
fault words are split into the sign-in's and the limit's, and a sign-in
fault is `environment`). A failure of the work records the word its
reason names where one is honest: `bad slices` `plan` (the plan's
slices name no declared repository), `protected path changed` `scope`
(the worker changed what the job may not), a second failed fetch from
a worker's clone `environment`. Every other failure (a timeout, a
non-zero exit, no output, a bad verdict, no commits, a secret in the
output, a rejected push, a clone reaching outside itself, the batch's
nothing proposed) records the cell empty. The vocabulary is the
roadmap's and fixed: a word for these, or a reading of a timeout as
`capacity` (the wall clock is Daisy's, not a provider's), would put a
judgement the reason does not carry into a column meant to be counted;
neither was taken. The round limit's second time, the one stop a run's
collection makes besides these, records none either: the roadmap
counted its review loop apart from the nine.

**The merge step's blocks have no record, and none is added here.**
The second conflict on one head (T25) and a merge refused while the
forge reports the pull request mergeable (T16) block the issue from
the merge step, which is no run and writes no row or journal line; the
first conflict and a failing check on the approved head (T26) send the
pull request back and stop nothing. A record of the merge step's
blocks would be a new kind of row, a design of its own; section 7.5
names each such stop and says it is recorded nowhere. A plan cut short
or a kept commit not given back is not detected by Daisy at all: the
member that meets it stops naming `kept-work`.

**`swarm unblock <n> --cause <word>` supersedes the row of the run
whose stop it releases, by the CSV's own rule for a correction.** The
row written is the held row with the word in `cause` and its notes
ending `retagged`, every other cell carried over, appended under the
metrics lock with `metrics.Supersede`, as `swarm metrics --reprice`
appends its corrections; the summary reads a key's last row. The run is
found on disk: of the issue's collected runs whose result holds
`flagged swarm:blocked`, the one that ended last, refused when a run of
the issue started after it ended (the block it set was lifted since,
and the block now released was set by the merge step or by hand, which
no row records). The alternative, the forge's newest blocking note,
was not taken: its marker names the run's number but not its role, and
the merge step's notes carry the same marker. A block set by hand after
a run's block was lifted, with no run since, still reads as that run's;
the case needs a hand to undo a lift and is left to the maintainer, who
gives no `--cause` there. The row is written before the comment and the
relabel, every refusal before it, so a run again after a refused forge
write finds the cause written and writes nothing a second time; a word
outside the vocabulary is usage (exit 2) before anything is read.
`--cause` leaves the flag name `--type` free for the interventions'
record (R0, a later step). `reconcile.IsCause` and
`reconcile.CauseWords` are exported for the verb. Tests:
`TestSwarmApplyRecordsDaisysOwnCause`, `TestSwarmUnblockRetagsTheCause`.
