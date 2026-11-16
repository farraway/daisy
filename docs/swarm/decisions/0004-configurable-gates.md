# 0004. Configurable gates

Moved verbatim from `docs/swarm.md` section 14, item 4, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Configurable gates** (the maintainer's second design change). G1
triage, G2 plan and G3 merge are `gates` fields, each on by default;
G4 unblock is always human and G0 pacing always applies. Off, the
owner's run sets `ready` (and the batch creates `ready` issues), the
architect's first plan sets `planned` with no revision round, and the
merge step merges and closes. Fully off, the swarm runs between the
maintainer's visits; `budget`, required then, is its brake once it
is enforced (item 5: validated, not enforced).
