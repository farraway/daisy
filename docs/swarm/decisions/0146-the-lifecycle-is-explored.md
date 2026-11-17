# 0146. The lifecycle is explored

From Step 36 of the plan remediating the 2026-11-15 design review (talk A15, 2026-11-16).

**The swarm's lifecycle is model-checked by a bounded explorer over the
Fakes (Step 36 of the plan remediating the 2026-11-15 design review,
2026-11-16; talk A15).** The lifecycle is a state machine over forge
labels (section 5) that `apply` reconciles level by level, and the
example tests walk the paths their authors thought of; an interleaving
nobody wrote down (a CI status changing between a review and a merge, a
pause between a collection and a launch) went untested.
`TestExploreLifecycle` (`internal/swarm/reconcile`) explores every
sequence of events up to a bound over two issues, #1 approved at
`review-2-approved` and #2 at `review-0-ready`, each with its pull
request open, the merge gate off: a pass; a member's end (a reviewer's
verdict either way or its `BLOCKED:`, the architect's plan, the worker's
push); the maintainer's pause, resume, unblock, block and accept; a CI
status, pending then success or failure; a merge refused, a
mergeability unknown or settled, checks required on `main`, Daisy's
next relabel refused. After every event, and every relabel of Daisy's,
it checks that each open issue carries exactly one stage, that no pull
request merged with a check failed or pending on its head, that nothing
launched on an issue carrying `swarm:blocked` or `swarm:paused` or held
by the maintainer's `Blocked by #<m>` (the explorer's own record of the
comment, not the code's reading of it), and that every label Daisy
wrote is a row of section 5's table. The first violation fails the test
with the shortest sequence that reaches it: with the merge step's read
of the checks (F27) reverted, `CI reports pending on #1's head`, then
`a pass of apply`.

**Where, and what it drives.** In `reconcile`'s own tests, not `cmd`'s:
a `cmd` pass runs real git for its mirrors and clones and costs a
quarter of a second, too slow for thousands of states. The explorer
calls `apply`'s pass step by step (collect, the stages with the role
rule, the merge step, the flag, the labels copied) and stands in for the
two parts that reach git: the launch's preparation, past which the
launch's own in-progress relabel runs (`Swarm.inProgress`, split out of
`launch` for it), and the worker's collection, whose head is pushed to
the forge and whose T8 or T12 is the collection's own `relabel`. Every
other collection (the reviewers', the architect's, a block) is the real
one, and an `Exec` fails the test, so a path that reaches git is seen.
A state is a fingerprint (the forge's issues, pull requests, comments,
reviews and checks; every file; the containers; the model's record;
times replaced by their ranks), expanded once, breadth first: each
state expanded is replayed once from the start, and each of its
children is a clone of it with one event more: `forge.Fake.Clone`
(section 10) and an in-memory file system in the test. The bound is five events by default, 11,604 states
in about five seconds; `-explore.depth` runs deeper (six: 44,024
states, 20 s; seven: 155,886, 79 s). No violation at HEAD to seven.

**The table is written in the test and held to the spec's text.** The
code has no transition table; its moves are spread over the
collection, the launch, the merge step and the flag. `sectionFive`
encodes section 5's rows, one transition per move a row draws (T11's two
targets and its block, T15's close and terminal, T22's homes), with
their gate and setter; `TestSectionFiveTable` reads `docs/swarm.md` and
fails when a row's id is missing on either side, or a row no longer
names the stages, gate or setter the test gives it. A write of Daisy's
is matched move by move (the stage left for the stage written, a close,
each flag set or cleared); `swarm:gated` is step 8's predicate and no
row. A relabel refused is refused whole, as `forge.Fake`'s relabel is
one call: the half-made relabel of a two-call forge is the two-stage
state section 5 already names and `apply` skips.

Alternatives: the explorer in `cmd`, driving the whole pass through
real git (exact, and too slow to explore); a hand-written model of the
lifecycle checked instead of the code (it would check the model);
parsing the table's prose cells for the allowed moves (its cells name
some stages by description, "the in-progress stage a run's launch
wrote"); a random walk in place of the bounded search (no shortest
counterexample, and no claim about every sequence within the bound);
replaying each child from the start in place of cloning it (two to
three times slower). Tests: `TestExploreLifecycle`, `TestSectionFiveTable`,
`TestFakeClone`.
