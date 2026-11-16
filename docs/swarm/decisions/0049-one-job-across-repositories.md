# 0049. One job across repositories

Moved verbatim from `docs/swarm.md` section 14, item 49, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**One job across repositories** (D51's first gap: the library job
Step 39 ran lived in another repository than the product's issues;
and an outside agent project whose jobs span a public and a private
repository). Chosen: the issue stays in `forge.repo`; `repos` is an
ordered array of `{name, path, check, image, protected}`, the first
being the product, whose image runs the owner and the architect over
a clone of every repository at `/work/<name>`; each slice names its
repository when more than one is declared, a plan that does not is
*bad slices* (with one, an unnamed slice is in it); one mirror per
repository; a pull request outside `forge.repo` carries
`Refs <owner>/<repo>#<n>`, and `Pulls(n)` reads each declared
repository. `base` stays one field for all. The pull-request methods
take a `Ref`, so the method count stays sixteen, and `Issue` no longer
reads the issue's `id`, which `Board` reads itself. Labels, the stage
and every transition stay on the issue; a pull request carries the
copy (section 5) wherever it lives. Alternatives: one swarm per
repository, under which a library job has no product issue (Step 39);
the forge's cross-repository search for `Pulls`, whose items lack
`head.ref` and whose matching rests on the search index (section 10).
