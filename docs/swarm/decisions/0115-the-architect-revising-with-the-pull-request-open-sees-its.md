# 0115. The architect revising with the pull request open sees its head

Moved verbatim from `docs/swarm.md` section 14, item 115, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The architect revising with the pull request open sees its head
(Step 4 of the plan that clears the Deferred list, 2026-11-11; the
maintainer's decision of 2026-11-10).** After the round limit (T11),
or the maintainer's relabel, the architect revises the plan from the
pull request's reviews and its check (7.3), but its clones were all
at `<base>`: it judged a change it could not read. The head is now a
local branch, `swarm/<n>-<s>`, of that repository's clone, beside
`<base>`, and the open-slice sentence names it with the diff to read
(7.1 step 3, 7.4). It is made as the local `<base>` branch is, from
the mirror's branch the clone already fetched, rather than by a
second fetch: the pass's fetch is the one that gives a reviewer the
same head. The diff in the prompt instead would add its size to
every revising run and show none of the files around it. A head gone from the mirror (the branch deleted
on the forge) is the swarm fault *clone failed* (7.7), as it is for a
reviewer. Nothing collects the architect's clone, so the branch is
read-only in effect. Tests: `TestSwarmApplyRoundLimit` (the revising
architect's `swarm/<n>-<s>` is the pull request's head),
`TestSwarmApplyRoundLimitHeadGone` (the branch gone: `fault: clone:`,
nothing launched), `TestRenderArchitectOpenSlice` (the sentence).
