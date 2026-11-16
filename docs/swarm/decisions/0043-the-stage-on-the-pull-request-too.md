# 0043. The stage on the pull request too

Moved verbatim from `docs/swarm.md` section 14, item 43, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The stage on the pull request too** (the maintainer's instruction at
Step 39). Chosen: each pass copies the issue's `swarm:*` label to its
current open pull request through `Relabel` on the pull request's
number (with `remove` `""` when it has none), so no forge call is
added; the issue's label stays the one state, and no rule reads the
copy. `apply` also runs `EnsureLabels` at its start, so a deleted
label is created again. Alternative: relabel both in each collection
step, which would leave the maintainer's own relabels uncopied.
