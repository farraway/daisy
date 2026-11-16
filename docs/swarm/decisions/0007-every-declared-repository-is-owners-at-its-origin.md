# 0007. Every declared repository is `<owner>`'s, at its `origin`

Moved verbatim from `docs/swarm.md` section 14, item 7, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Every declared repository is `<owner>`'s, at its `origin`.** A
slice's branch is pushed to its repository's `origin` and its pull
request opened in that repository, so each `repos[].path`'s `origin`
must be `<owner>/<name>`; `forge.repo`, the issues repository, need not
be declared (revision 4; revisions 1–3 required it to be `origin`'s,
with one repository). Alternative: push to a repository other than
`origin`, which needs a second URL and credentials.
