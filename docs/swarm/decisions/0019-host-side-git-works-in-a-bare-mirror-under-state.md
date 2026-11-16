# 0019. Host-side git works in a bare mirror under `<state>`

Moved verbatim from `docs/swarm.md` section 14, item 19, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Host-side git works in a bare mirror under `<state>`.** Chosen: the
member's clone is cloned from `<state>/git` (from revision 4 a mirror
per repository, `<state>/git/<name>`), fetched back into it, and
pushed from it; each declared repository is only read. Alternative: the
plan's "fetches the branch from the mounted clone into the host
repository", which writes refs and objects into the product repository
(the workspace itself when a `path` is `.`, against the claim of
Step 30 of the plan that specified this document that nothing beyond
`swarm/` is written there). Both designs fetch from the
member's clone; neither runs any other git command in it. From
revision 2 every worker run's commits are fetched to
`refs/swarm-runs/<n>-worker-<k>` first, and kept there when the run
failed, so a timed-out worker's work survives for the maintainer.
