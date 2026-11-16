# 0034. The round limit is set by the reviewer's collection

Moved verbatim from `docs/swarm.md` section 14, item 34, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The round limit is set by the reviewer's collection** (T11: the
pair completing on round `max_rounds` with a request for changes moves
`in-review` to `blocked`). The plan's table puts it on the worker's
`reviewed` row; there it would need a worker launch to decide not to
launch. From revision 5 the limit moves `in-review` to the architect's
stage (`planning`, or `ready` with G2 off) the first time and to
`blocked` the second (item 55); the collection still decides it.
