# 0157. The scorecard's window ends at `--until`

From Step 8a of the plan implementing the 2026-11-19 swarm roadmap (R0, measure first, 2026-11-19).

Decision [156](0156-a-scorecard-per-merged-slice.md) left the
scorecard's window open at its end, from `--since` to now, and named
the end as an open question: the baseline's per-day figures need a day
with both bounds. `daisy swarm metrics --scorecard [--since <time>]
[--until <time>]` gives it (docs/swarm.md section 11, The scorecard).

**A record or a merge after `--until` is left out altogether**, of the
slices' lines as of the summary. `--since` keeps a slice's records made
before it, because a slice merged in the window cost what its runs cost
whenever they ran; leaving out of the lines only the merges after
`--until`, and of the summary only its records, was the alternative.
With `--until` the scorecard counts what had happened by that instant,
as the records read now tell it, and a slice's line holds no act made
after the bound. Of a slice merged by then, only the `merge` verb's
intervention can come after it, being recorded once the merge is made
(a run ending after its slice merged already counts in the issue's
next slice). A baseline taken over a past window can so be taken again and
agree, but for the corrections made since (a retag, a reprice).

**Both bounds are inclusive**: `--since` takes what is at or after it,
`--until` what is at or before it, so a merge at the bound is in the
window and two adjacent windows sharing a bound count an event at that
instant in both. An exclusive end was the alternative; it would make
`--since` and `--until` read the same time two ways.

**The window ends at `--until` or now, whichever is earlier.** A
`--until` after now (the end of today, written ahead) counts no record
after now, there being none, and its days not yet passed would dilute
the per-day rates. Refusing a `--until` after now was the alternative:
it would fail a window ended at the day's end while the day runs.
Without `--since` the window starts at the first record or merge not
after `--until`, so a `--until` before every record reads `no record`
rather than a negative length.

**`--until` before `--since` is usage** (exit 2), refused before
anything is read, the values named as given; the same time for both is
a window of one instant. It is a flag of `--scorecard` alone, as
`--since` is.

Tests: `TestScorecardRowPerMergedSlice`, `TestScorecardRefusals`.
