# 0084. The adversary's third rollback exception: behaviour-preserving changes

Moved verbatim from `docs/swarm.md` section 14, item 84, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The adversary's third rollback exception: behaviour-preserving
changes (correction 3, 2026-11-09).** The first routine jobs on the
swarm's own repository were a deletion of dead code and an index over
an opener stack: changes whose tests pin the behaviour kept, which the
rollback probe cannot fail by design. The adversary applied the probe
twice on each, acknowledged the worker's reason and blocked anyway,
because the two exceptions of item 63 did not name the case. The
third exception does: when the plan or the report says the change
removes, restructures or re-costs code without changing its results,
its tests are judged by breaking the live path they exercise (a
scratch mutation in the throwaway clone, or the worker's reported
probe), not by the rollback. The alternative, dropping the rollback
for routine jobs, was rejected: the probe still catches a test that
cannot fail in a behaviour change.
