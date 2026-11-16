# 0104. Moving a swarm by hand

Moved verbatim from `docs/swarm.md` section 14, item 104, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Moving a swarm by hand (the maintainer's answer of 2026-11-10:
"No migrate verb").** Section 11's Moving a swarm into its
repository describes the old layout once and the steps a maintainer
takes; `init` run in the moved clone does the forge's part and
prints the unit. A swarm whose `forge.repo` was a product repository
starts clean in its new repository, `<state>` keyed on `forge.repo`
(item 83), as the plan's non-goal of changing that key requires.
