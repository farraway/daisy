# 0021. Protected paths

Moved verbatim from `docs/swarm.md` section 14, item 21, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Protected paths.** `protected` globs, checked in the mirror before
any push, with a failure kind of their own: a push to a branch runs
that branch's CI workflows with the repository's secrets before any
gate, and an agent instruction file steers every later member once
merged (*Orchestras, Not Factories*: paths that always need a human,
enforced rather than hoped for).
