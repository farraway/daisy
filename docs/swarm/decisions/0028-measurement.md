# 0028. Measurement

Moved verbatim from `docs/swarm.md` section 14, item 28, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Measurement.** Each run's `result` records cost, tokens and turns
when the CLI reports them and one `outcome` line, and `status` has a
Throughput block, so a later tool can aggregate `runs/` without
re-deriving it; the first real run's costs (Step 30 of the plan
that specified this document) come from there. The meta loop that
would turn these signals into prompt changes is not in version 1
(section 1; *Harness Engineering* and *Software Engineering Is
Becoming Factory Engineering* name it as what makes a pipeline a
factory).
