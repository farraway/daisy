# 0075. A merged pull request is recognised by its title when the forge has deleted its branch

Moved verbatim from `docs/swarm.md` section 14, item 75, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A merged pull request is recognised by its title when the forge has
deleted its branch (2026-11-08, the first day after Step 15).** Gitea
deletes a merged pull request's branch when the repository's
"delete branch after merge" default is on, and then reports its head
as `refs/pull/<m>/head`; `Pulls(n)` matched the branch `swarm/<n>-<s>`
alone, so every merged slice vanished from the swarm's view: a job's
next worker was told to implement slice 1 again over "Earlier pull
requests: None", found nothing to commit, and was relaunched each
pass. The `Refs` line stays the guard; the slice now comes from the
branch when it is one and otherwise from the title the collection
writes, `<issue> slice <s>: <title>`, and the pull request's `Head`
is then the branch it had. A title without the slice, or a head that
is neither, is not the swarm's. The alternative, requiring the
maintainer to turn the forge's default off, was rejected: the swarm
should not depend on a repository setting it cannot see.
