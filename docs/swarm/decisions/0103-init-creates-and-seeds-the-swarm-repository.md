# 0103. `init` creates and seeds the swarm repository

Moved verbatim from `docs/swarm.md` section 14, item 103, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`init` creates and seeds the swarm repository (the maintainer's
D7, and the plan's derived decision on the tokens).** The
maintainer's token reads, creates and opens the repository to the
swarm's account (an organization's repository: `<owner>` is one on
Gitea, section 3, so the user's namespace is not offered); the
swarm's token reads the issues and the declared repositories and
makes the labels, as before; each write's line names its token.
`init` clones into `<dir>/<repo>` when it runs outside a clone,
seeds the tree of section 2, commits exactly the files it wrote with
the clone's own identity (the maintainer's) and pushes them to the
default branch, writes `<state>/env` and the mirrors (the readiness
warnings then read a mirror, not a working tree), and prints the
unit for the clone's path. Choices the plan left: the commit message
`swarm: written by init`; question 4's last default, `<owner>/swarm-<name>`
after the first product (the convention of item 83 applied to the
one product a swarm serves); question 32's default the seeded
`briefs/` and `docs/`; the created repository always private (the
maintainer's answer of 2026-11-10: no wizard question; the
maintainer changes its visibility on the forge if wanted). `init`
run in an empty folder has no product clone to default question 1
from, so `--defaults` there needs a folder holding the products'
clones, or a `swarm.json`.
