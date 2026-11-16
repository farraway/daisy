# 0002. Two conversations, one per surface

Moved verbatim from `docs/swarm.md` section 14, item 2, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Two conversations, one per surface** (the maintainer's design change
at Step 38). The issue is the design thread and the pull request the
implementation thread; the worker answering reviews and the reviewers
read the pull request with the issue body and the accepted plan as
context, never the design debate. This replaces revision 1's single
thread on the issue, in which pull request comments were not read and
every role read the whole history: the maintainer's line comments now
reach the worker, and each role's prompt holds only what its job needs.
