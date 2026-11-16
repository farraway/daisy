# 0020. No history rewrites; base drift

Moved verbatim from `docs/swarm.md` section 14, item 20, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**No history rewrites; base drift.** The worker never amends,
rebases or resets a pushed commit (the push is never forced, so a
rewrite would only be rejected). With the merge gate off the worker
merges `base` into its branch first and a conflict is `blocked`; with
it on, the maintainer resolves conflicts at merge. Resolving a
conflict automatically is recorded, not built. (Amended by item 81:
every worker run merges `base` first and resolves what it can, and a
pull request the forge reports not mergeable goes back to the
worker.)
