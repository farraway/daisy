# 0125. Backfilling the metrics of runs collected before item 118

Moved verbatim from `docs/swarm.md` section 14, item 125, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Backfilling the metrics of runs collected before item 118 (Step
5 of the plan for the inbox requests F27-F35, 2026-11-14; the
request F31; the maintainer's decision, reversing item 118's
recommendation).** The runs a swarm collected before Step 16b have
no row, though everything a row holds but the commits in use is in
`<run>`. `daisy swarm metrics --backfill` writes one per collected
run without a row (section 11), through the collection's own `row`,
so a backfilled row and a live one differ only in `when` (the run's
end, not the backfill's time), the two commits (unknown, empty) and
the notes (`backfilled`). A verb, not a step of `apply`: it is run
once per swarm, by the maintainer, and its rows go to the home the
machine's `<state>/home` names, which it refuses to guess.
**The run's identity** is the row's key, its kind, swarm, run and
role, the key `metrics` supersedes by: `metrics.Append` writes no
row whose key the CSV holds, for `apply` as for the backfill, so a
run recorded live is never backfilled, a collection retried after
its row adds none, and a second backfill writes nothing. The
`recorded` result line is not the test: a run recorded while
`<state>/home` named no usable home has the line and no row.
**The persona** of a run without its `persona` line is read from
`<run>/argv`, the provider whose CLI argv (section 9's `Command`, up
to its effort) lies in it, for a live collection too, before
`swarm.json`'s at the collection, which may have changed since the
launch: every run's argv names its model and effort, and those of
the runs before Step 16b are today's `Command`'s words.
**The cost of an old line**: before Step 16b a CLI that reported
usage and no cost wrote `cost 0`, as `cost -` is written now; a
result with no `persona` line (the two came together) is read as
the old form, its `0` a cost not reported, whatever the provider.
The alternative, by provider, leaves `0` for grok, which reports a
cost not always, and a run that reported usage has never cost
exactly nothing. `status` still counts such a run as costed (section
11, Throughput), so its total and the rows' sum match, its count of
runs without a cost does not. **The lock** is taken once, for one
read and one rewrite: every row is made, the workers' ranges diffed,
before it, so a live `apply` waits on a backfill of hundreds of
runs as on one row; the runs the CSV holds a row of are read first,
without the lock, so a second backfill measures nothing, and again
under it, so a row written in between is not doubled. A worker
whose commits its mirror no longer holds keeps its row, its line
cells empty, with `lines not counted` in its notes, live or
backfilled. No journal line is written: the journal is the record
of a day's collections, and those days' files are pushed. Tests:
`TestSwarmMetricsBackfill` (a run recorded live and three of the
old form, owner, codex adversary with `cost 0` and grok worker,
and one not collected: three rows, each persona from its argv
against `swarm.json`'s, the worker's lines, the times, the rows'
costs equal to `status`'s; a second backfill leaves the CSV's
bytes), `TestSwarmMetricsBackfillGoneShasAndNoHome` and
`TestAppendSkipsHeldKeys`. A backfilled row keeps the estimate it
was written with: a price added later reprices none (item 129).
