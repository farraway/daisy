# 0063. The adversary's fail-against-base check is bounded

Moved verbatim from `docs/swarm.md` section 14, item 63, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The adversary's fail-against-base check is bounded (Step 7's first
correction, 2026-11-07).** The first real review pair had the
adversary request changes on two findings its own text admitted were
not defects: a parser and a classifier the slice kept in `_test.go`
files, whose tests the prescribed rollback of non-test paths cannot
make fail, and a Test Plan claim met by a pre-existing test the
change did not touch, which passed against `base` by construction.
The rollback is a probe for a test that cannot fail; where it cannot
apply, the adversary now checks by reading that the test exercises
its claim, and that is the only finding those two cases can yield
(7.3). The worker's `reviewed` task says it may answer a finding with
a reason instead of a change, as its persona already did (section
4), and the round `r > 1` text asks the reviewer whether each earlier
finding is resolved or answered with a reason it accepts, so a
procedural finding ends in one round rather than at `max_rounds`.
The alternative, dropping the rollback probe, was rejected: it is the
one check that catches a test which passes without the change.

**Amended by decision
[168](0168-a-delta-review.md)
(Step E2 of the plan implementing the 2026-11-19 swarm roadmap).**
The round r > 1 text is a delta review's on a head with commits since the reviewer's last review: it asks whether each blocking finding of the previous round and of the reviewer's own last review is closed or answered with a reason it accepts; the fail-against-base check is given as in round 1.

**Amended by decision [171](0171-review-by-risk-with-an-end.md)** (R6, specified ahead of its build, Step R6-design of the plan implementing the 2026-11-19 swarm roadmap): beside the rollback, the adversary checks the probe the plan's Test Plan names for each test, a test that survives it blocking as its claim's failure; a test that survives a probe the reviewer invents is non-blocking unless the surviving change shows the head is wrong; a plan with no probes leaves the rollback as the one named check (`docs/swarm.md` section 7.11).
