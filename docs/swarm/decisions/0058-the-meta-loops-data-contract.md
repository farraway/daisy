# 0058. The meta loop's data contract

Moved verbatim from `docs/swarm.md` section 14, item 58, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The meta loop's data contract** (revision 5; the synthesis's G7,
from *Build a Prompt Learning Loop*). Not built in version 1; recorded
so that version 1 keeps the pairing reconstructible. The dataset is
the runs whose `outcome` was `blocked` or `failed`, and every run of
a slice that took more than one round, each paired with the
maintainer's comments in words on that issue and pull request (the
trusted logins that are not the swarm's, read from the forge by the
issue and run named in `<run>`'s directory name); the target is one
persona prompt's rules (`swarm/prompts/<role>.md`), never the fixed
Task text; the output is a diff the maintainer reviews before it is
committed, the prompts being a slop-free zone. Everything the loop
needs is in `<run>/result` and on the forge, and nothing is added for
it.
