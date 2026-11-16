# 0014. Sequential reviewers, each blind to the other on the head

Moved verbatim from `docs/swarm.md` section 14, item 14, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Sequential reviewers, each blind to the other on the head.** Rule 2
("no member running for that issue") is kept, so the adversary reviews
a head before the sympathetic reviewer does: sequence gives each a
fresh context (*What It Actually Takes to Build a Software Factory*).
But the second no longer sees the first's review of the same head, so
the pair stays two independent reads; reviews of earlier heads stay.
Alternative: rule 2 per role, both at once.
