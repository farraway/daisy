# 0048. A board as a view

Moved verbatim from `docs/swarm.md` section 14, item 48, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A board as a view** (the maintainer's decision at revision 4).
Labels stay the only state store on both forges; with `forge.board`,
`apply` keeps each open swarm issue's card in its stage's column of a
Gitea organization project (the project API Gitea gained in 28.0),
through one method, `Board`, so every forge call stays an `Adapter`
method. A board fault is a warning: it blocks no launch, writes no
`paused`, counts toward no swarm-fault pair and sets no exit code, so
a view cannot stop the lifecycle. A `Board` call is never retried,
and a pass stops calling it after one fault. Alternative: the common
three retries, under which a failing board delays a launch by up to
30 s per issue. GitHub's boards (Projects v2, items
with a status field rather than columns) are not mapped: not supported
in version 1, refused at validation.
The maintainer creates the project; `Board` creates only a missing
column. Alternatives: the board as the state store (two stores to
keep consistent, and a lifecycle that would differ by forge), or no
board.
