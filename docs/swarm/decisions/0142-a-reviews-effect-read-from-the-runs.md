# 0142. A review's effect read from the runs

From Step 30 of the plan remediating the 2026-11-15 design review (P29, 2026-11-16).

**A reviewer run's row records its effect, the blocking findings it
raised that the worker fixed, read from the runs with no model judging
a finding (Step 30 of the plan remediating the 2026-11-15 design
review, 2026-11-16; P29).** The sympathetic reviewer was about half of
the swarm's reported cost with nothing measuring what it bought; a
reviewer is measured before one is changed. **The definition:** a
review not posted (failed, stale, discarded, a question) and an
approving review have effect 0; a review requesting changes is
credited the count of its `[blocking]` lines when its role's next
posted review on the issue is of the next round (its `round` line one
more: the same pull request, its rounds counted on), on another head
(its `start` line another commit: the worker pushed since) and
approves; 0 when that review is on the same head (the findings answered
with `NO CHANGE:` or by the maintainer, not fixed) or requests changes
again; unknown, the cell empty, while no next review is collected, for
a run without a `round` line (recorded before Step 16b), and when the
next is of another round (a re-plan at `max_rounds` counts the rounds
again; another pull request starts at 1). Every input is already in
`<run>/result` and `<run>/io/out.md`, so the runs collected before this
change are measured too. **What it misses:** a finding fixed in a round
whose review still requests changes (two of three fixed, one left)
counts 0 for that review, and only the last review of a chain that
ends approving is credited; the same finding raised by both reviewers
counts for each; a non-blocking finding the worker fixed counts
nothing; a blocking finding the reviewer dropped after the worker's
answer on a new head counts as fixed; and a finding fixed by the
architect's re-plan after the round limit stays unknown. Judging
findings one by one (a model matching the second review's text to the
first's) was the alternative: it would cost a model call per review
and be a judgment, not a count. **Recorded in the row, superseded once
known:** unlike a revert (decision
[141](0141-reverts-counted-from-the-mirrors.md)), a review's effect is
final the moment its next review is collected, so the collection of
that review appends one row superseding the earlier review's, every
other cell carried over, and `swarm metrics --reprice` makes the same
row for runs recorded before; the plan's review rows, which no runs
back, take the same column from `metrics add --review-effect`, so one
column holds both and `daisy metrics` sums it by role and model.
