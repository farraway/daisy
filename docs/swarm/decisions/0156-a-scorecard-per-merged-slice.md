# 0156. A scorecard per merged slice, computed when read

From Step 7 of the plan implementing the 2026-11-19 swarm roadmap (R0, measure first, 2026-11-19).

The maintainer adopted R0 (2026-11-19): a scorecard row per merged
slice, with runs by role, uncached, cached and output tokens, a
billed-equivalent cost, the wall-clock from proposal to merge, review
rounds, stops by cause and maintainer interventions, and `daisy swarm
metrics --scorecard [--since <time>]` printing the baseline. What
follows is how (docs/swarm.md section 11, The scorecard).

**Computed when read; no stored scorecard.** Every figure is already
recorded: the runs' and the interventions' rows in the metrics CSV
(decisions [153](0153-a-cause-on-every-member-stop.md) to
[155](0155-interventions-recorded.md)), the merges in the mirrors
(decision [141](0141-reverts-counted-from-the-mirrors.md)). A stored
row per slice, written by the merge step, was the alternative: it would
miss a slice the maintainer merged on the forge (the merge step finds
it merged and records nothing), go stale whenever a later row
supersedes one it summed (a retag, a reprice, an earlier review's
effect), and need a writer in the pass for a figure only a reader
wants. `--scorecard` runs under `ops.ReadOnly` and writes no row.

**A merged slice is the mirror's merge commit**, as the reverted line
reads one (0141): a merge commit on `base`'s first-parent line whose
second parent a worker run pushed, its issue and slice the run's
branch `swarm/<n>-<s>`, its time the commit's. The forge's pull
requests were the alternative; they carry no merge time and would need
a call per pull request, where the mirrors hold every merge and are
fetched each pass. `ReadPullReverts` now counts over the same reader,
`ReadMerges`, so the scorecard and the reverted line cannot disagree on
what merged.

**A record is a slice's by its time, between the issue's merges.** A
run ends, and a verb's act is made, before the merge it leads to, so
the issue's records after its previous merge and not after this one are
this slice's; the `merge` verb's intervention, recorded once the merge
is made, is the slice's whose merge is the last not after it. Reading
each run's slice from its result was the alternative: only a worker's
run names its branch; an architect's re-plan, a reviewer's round and an
intervention carry no slice, and the CSV, not the state, is the
record. The cost: a run recorded after its slice merged lands in the
next slice of the issue (none, for the last), and a row dated by its
collection rather than its end (before this step) can cross a merge by
the collection's delay. A run's row is now dated by the run's end, as a
backfill already dated it (backlog L329), which narrows that delay to
none for a row written from now on.

**The wall-clock runs from the issue's opening on the forge**, for the
issue's first merged slice, and from its previous merge for a later
one. The opening is the owner's or the QA's proposal posted, or the
issue the maintainer filed: one instant every issue has. The owner's
proposal run was the alternative the step named; Daisy records no link
from a batch run to the issues it opened (its result holds `creating
<i>`, not the number), and a maintainer's issue has no proposal run at
all. The opening is read with `Issue(n)`, one call per issue whose
first slice is shown, so `--scorecard` reads the swarm's token, unlike
`--backfill` and `--reprice`. A later slice is timed from the previous
merge, not the opening, so that the slices' times add up to the issue's
instead of each repeating the first.

**The cost is the estimate at list price, a row without one counted as
$0 and named.** `cost_estimated` is the billed-equivalent cost the
maintainer asked for (decision [152](0152-every-model-in-use-priced-from-its-providers-page.md)
prices every model in use, the cache apart); `cost_reported` is what a
CLI said, missing for codex's runs and the subscriptions'. Leaving a
slice's cost unknown when one row lacks an estimate was the
alternative: a single old row would blank the slice; extrapolating from
the other rows would print a figure no record holds. The count of rows
without an estimate is printed beside the sum, and `--reprice` gives
the old rows their estimates. Likewise, a row with `tokens_in` and no
`tokens_cached` counts all its input as uncached (the rows `--reprice`
cannot read), and a row with no usage is counted and named.

**The summary is over the window's records, every issue's and the
batch's, merged or not.** The roadmap's baseline figures are per day
across the swarm: input per merged pull request (8.0M, 16.3M, 29.4M),
runs per merge (5.7 to 8.8), the worker's stop rate (runs ending
`BLOCKED:` or `QUESTION:` over worker runs), interventions a day. Their
numerators count every run of the day, the owner's batch and the runs
on issues not yet merged included; summing the slices' rows alone was
the alternative and would leave out that rework, which is what the
figures are meant to show. A slice's row takes its records whatever
their dates; the summary takes only the window's. The window's length
is fractional days, from `--since` (else the first record or merge) to
now. Only `--since`: the baseline's per-day figures need an end too,
which the step did not ask for (an open question).

**Folded in.** Backlog L425, the maintainer's yes of 2026-11-18: a revert that a
later revert in effect names (a reapply) no longer counts, the chain
followed newest first (`git log --topo-order`, so a revert's reverters
are settled before it); `metrics.Reverted` serves `daisy metrics`'
steps, `status`'s and `daisy metrics`' pull requests and the
scorecard's merges alike (0141 amended). L362: `status`'s Throughput
counts a run ending in a question under `questions`, not `other`; the
trigger rule's answering role is `Turn.Answerer` (the backlog's
"rename it `Asker`" would name the role that asked, the opposite of
what the field holds; the name follows `Answerer`, the function that
fills it). L329: a row is dated by the run's end; a journal line with
no persona says `unknown persona`; Throughput's cost adds the runs'
estimate at list price. L381: an effect the runs read unknown never
supersedes a row (in `--reprice`, in a collection's earlier effects,
and in `--backfill`, which now supersedes the earlier reviews' rows a
backfilled review decides), so an effect set by hand stays; a repriced
row drops `<c> cached input` from its notes once `tokens_cached` holds
it.

Tests: `TestScorecardRowPerMergedSlice`, `TestScorecardRefusals`,
`TestRevertedReapplyCancelsTheRevert`, `TestPullReverts`,
`TestMetricsStepReverts`, `TestSwarmStatusThroughput`,
`TestSwarmStatusJSON`, `TestSwarmApplyRecordsWorkerLinesAndReviewRounds`,
`TestJournalLineUnknownPersona`, `TestSwarmMetricsBackfill`,
`TestSwarmMetricsReprice`, `TestSwarmMetricsRepriceReviewEffect`.

**Amended by decision [157](0157-the-scorecard-window-ends-at-until.md)
(Step 8a of the plan implementing the 2026-11-19 swarm roadmap).** The
open question of the window's end is answered: `--until <time>`,
inclusive as `--since` is, leaves out every record and merge after it,
of the slices' lines as of the summary, and the window ends at it or
now, whichever is earlier.
