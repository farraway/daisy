# 0128. A ruling re-opens a plan

Moved verbatim from `docs/swarm.md` section 14, item 128, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A ruling re-opens a plan (Step 9 of the plan for the inbox
requests F27–F35, 2026-11-14; F33).** A live job was planned before
two ADRs merged into its product's `docs/adr/`; three slices merged,
and the worker on the fourth blocked on the contradiction, a run
spent and the job held until the maintainer relabelled it for a
re-plan. **The rule** (section 6, The rulings): between slices, a
job whose plan predates a change to its repositories' `rulings`
runs the architect in the worker's place, after a reconciler note
naming the changed files, and the architect revises the remaining
slices or confirms they stand. **Between slices** is
`plan-1-accepted` with nothing running or uncollected on the issue
and its current pull request not open, which includes a plan whose
first slice has not started: a ruling that lands between the plan's
acceptance and slice 1 makes the same plan wrong. The alternative,
at least one slice merged, would let slice 1 be built on a plan the
ruling contradicts. A pull
request in review, or a worker running, is never interrupted: the
check waits for the slice to merge. **The default** is `docs/adr/`
when `protected` holds that entry, so a swarm that protects its
ADRs (as the live swarm's product does) needs no new field, and none otherwise;
`"rulings": []` turns it off; the patterns match as `protected`'s,
one matcher. **The basis** is the run's own record, its `start
<name> <sha>` lines (every architect run clones every declared
repository at `<base>`), not its `resource` lines, which name only
repositories a resource comes from. **How a plan stands**: the
architect's output begins `STANDS: <why>`, posted as its text, no
plan, and the new basis. Alternatives: the plan written again
unchanged (Daisy would have to compare plans to tell a confirmation
from a revision, and a reworded plan would read as a revision and go
to the gate), or a reconciler note carrying the new commit (a second
record of what the run's `result` already holds). **The trigger**:
the note is a counted reconciler note marked with the basis's run,
so rule 3 wakes the architect as after T11, and it ends with a
sentence the block count leaves out (`handsOn`, beside T11's first
note and the note on an issue moved during a run); it is posted once
a basis, so a pass with no member free, or a failed launch, posts no
second note, and once the architect has written, the basis is its
run, cloned after the ruling, so nothing is compared again and the
worker runs: no member is launched by Daisy's own note but the
architect, once. **T27**: a revised plan, with the plan gate on,
goes to `plan-0-posted`, as a plan does (the maintainer's for an
architecture job, T24's for a routine one); a confirmation, or a
revision with the gate off, keeps `plan-1-accepted`, so the next
slice proceeds with no maintainer step, as the request asks. Not
done: a ruling changed by the job's own merged slice (possible only
for a ruling path the repository does not protect) is compared like
any, and the architect confirms it. Tests:
`TestSwarmApplyRulingStands` (an ADR merged between slices: the
note, the architect on `plan-1-accepted`, `STANDS:`, then the worker
on slice 2 and no second note), `TestSwarmApplyRulingRevised` (T27
with the plan gate on, gated; accepted with it off, the worker
next), `TestSwarmApplyRulingUnchanged` (a change outside the
rulings, `rulings: []`, another repository's ADR, an ADR before the
plan's run: the worker, no note), `TestSwarmApplyRulingOpenSlice`,
`TestSwarmApplyRulingNotedOnce`, `TestLaunchReplan`,
`TestPlanAndBasis`, `TestRenderArchitectRulings`,
`TestFormatRulings` and `TestRulingPaths`.
