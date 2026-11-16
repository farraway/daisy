# 0064. `status` counts the maintainer's corrections

Moved verbatim from `docs/swarm.md` section 14, item 64, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`status` counts the maintainer's corrections (Step 8 of the swarm
build; the synthesis's G3).** The Throughput block gains a `stage
time` line and a line per issue, and the `Briefs:` block each
brief's milestones (section 11). The counts are read from the
forge with the reads `apply`'s pass already makes (`Issues(Closed,
label)`, then `Issue`, `Pulls` and each pull request's thread), over
the open issues and the ones the batch prompt counts as recently
closed, so a finished job stays visible for 30 days and the reads
stay bounded; the brief's counts take every closed issue, since a
milestone's progress is the whole history. A count is what the
maintainer can check on the forge by hand: a plan revision is an
architect's plan after its first, a block a reconciler note that
moved the issue to `blocked` (every note but the round limit's
first, whose closing sentence Daisy now writes from one constant the
count reads), a round section 6's count over all of a pull
request's reviews, a maintainer comment any comment, review or line
comment by a `trusted` login that is not the swarm's. The stage time
is the median member time per stage the run was launched on, from
`runs/` (the `label`, `started` and end lines), with the check
apart: the alternative, the time an issue waits at each label, cannot
be read from `runs/`, where neither the maintainer's relabels nor
the time of Daisy's own are recorded, and the first real run's
record kept stage times this way. "Waiting for you" reads an
`approved` issue's current pull request: after the maintainer's
merge with the merge gate on it says `next slice` or `close`, counted
as the merge step counts slices, not `merge` (the first real run
showed `merge` until the maintainer's relabel).
