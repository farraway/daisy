# 0050. Labels across repositories

Moved verbatim from `docs/swarm.md` section 14, item 50, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Labels across repositories.** On Gitea the eight labels are the
organization's, so one `EnsureLabels` covers every repository, and a
repository's own label of the same name shadows one; on GitHub, which
has none, `EnsureLabels` covers each repository. A team member with
Write may not create organization labels (Step 1's smoke of the swarm
build: `403`), so a refused create fails `EnsureLabels` with the
missing names (`init` exits 1, its files written), and the maintainer, an
organization owner, creates the eight once; `init` and `apply` then
find them and create nothing. Alternative: repository labels on Gitea
too, eight per repository, which a Write member may create, or as the
fallback when the create is refused (two label sources to read).
