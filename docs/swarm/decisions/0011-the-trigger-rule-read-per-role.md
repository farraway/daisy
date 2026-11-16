# 0011. The trigger rule, read per role

Moved verbatim from `docs/swarm.md` section 14, item 11, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The trigger rule, read per role.** Chosen: "the newest counted entry
of the thread is not R's own", with the issue body as the first entry,
the issue thread for the design stages and the pull-request thread for
`reviewed`, and, in `in-review`, "R has not reviewed the head".
Alternative: the plan's literal "the last comment is not the swarm's",
under which the architect never runs on `ready` after the owner spoke
last, and the second reviewer never runs after the first posted.
