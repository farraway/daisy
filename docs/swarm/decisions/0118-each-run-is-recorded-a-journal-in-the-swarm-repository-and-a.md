# 0118. Each run is recorded: a journal in the swarm repository and a row in a home's metrics CSV

Moved verbatim from `docs/swarm.md` section 14, item 118, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Each run is recorded: a journal in the swarm repository and a
row in a home's metrics CSV (Step 16b of the plan that clears the
Deferred list, 2026-11-12; the maintainer's answer to its question
4).** A run whose CLI reports usage and no cost recorded `cost 0`,
which `status` counted as costed and a built budget would have read
as spent; it now records `cost -`, a cost missing (section 3), and
the provider's `Report` says whether a cost came (`CostKnown`,
section 9). Each collected run is recorded once (7.6, The run's
record): a row in the metrics CSV of the home `<state>/home` names,
through the writer `daisy metrics add` uses, so that a plan's steps
and a swarm's runs are measured in one file, and a line in the
swarm's journal, `journal/<YYYY-MM-DD>.md` in the swarm repository,
its local view. The home is the machine's, in `<state>`, which `init`
records from the workspace it runs in, not `swarm.json`'s: a home is
a maintainer's, and the swarm repository is shared. One journal file
a day, pushed once a pass, keeps each push a small append (one
`journal.md` would rewrite a growing blob at every pass). The
journal's commit is made on the head as just fetched and only
appends, so a refused push keeps the lines for the next pass rather
than taking the notes' side branch, which exists for a commit made on
an older head. No member reads the journal: it is the maintainer's
record of what the runs cost and did, not material for a task (a
member's material is its `prompt.md`, section 7.3), so a `resources`
entry naming it is refused. The persona and a reviewer's round are
written at the launch (`persona`, `round`), since `swarm.json` may
change before the run is collected. Two writers of one CSV, `metrics
add` and `apply`, each read it and rewrite it whole, so both take
`$DAISY_LOCAL/metrics.lock` from the read to the rewrite; the lock
is per machine rather than beside the CSV, where the home's next
commit would sweep it up. The list prices are Anthropic's published
rates for the three Claude models the swarm uses; `gpt-6.1-sol` and
the `grok` models have none in the root, so no estimate, never a
guessed one (item 129 prices `gpt-6.1-sol`). The turns go in the row's notes: a new column would
make every existing CSV's header refuse (`docs/cli.md`, `metrics`).
Tests: `TestSwarmApplyRecordsEachRun` (two runs, one reporting its
cost and one not: two journal lines pushed by the reconciler, two
rows with the second's `cost_reported` empty and both estimates,
`status` counting one run without a cost),
`TestSwarmApplyRecordsWorkerLinesAndReviewRounds` (a worker's lines
per class from its pushed range, the reviewers' round; no
`<state>/home`, no row), `TestSwarmJournalKeptForTheNextPass` (a
refused push and a `journal` that is a file keep the lines),
`TestSwarmInitRecordsTheWorkspacesHome`, `TestQueueHoldsARunOnce`,
`TestAppendUnderTheLockLosesNoRow`, `TestLockGivesUpNamingTheHolder`,
`TestEstimate` and `TestReport`. The runs collected before it got
no row here, by the recommendation the plan's archive recorded;
`swarm metrics --backfill` writes them (item 125).
