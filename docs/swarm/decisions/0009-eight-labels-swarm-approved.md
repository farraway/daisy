# 0009. Eight labels: `swarm:approved`

Moved verbatim from `docs/swarm.md` section 14, item 9, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Eight labels: `swarm:approved`.** Revision 1's `swarm:reviewed` meant
both "approved, the maintainer's move" and "changes requested, the
worker's", so the merge queue could not be seen. Its transition split
in two (T9, T10), and `status` gained "Waiting for you" (*Why Software
Factories Fail* places the bottleneck at human review; *Orchestras, Not
Factories* has the human see where they are needed).
