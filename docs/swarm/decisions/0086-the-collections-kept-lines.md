# 0086. The collection's kept lines

Moved verbatim from `docs/swarm.md` section 14, item 86, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The collection's kept lines (Step 25 of the swarm build,
2026-11-09).** **The mark of a finished member** (7.2, 7.7): `ended`
was taken on any regular `out.md`, so a member killed mid-write had
its partial text posted as complete and an empty one was *no
output* where abandoned would relaunch. Of the two marks the
Deferred item named, a sentinel the image writes and the streams
teed by the image, both needed a rebuild of every product image;
the member's argv is instead run by the image's own `sh`, which
writes the CLI's exit status to `/work/.swarm/exit` once it has
exited. A container killed outright writes none, so its run is
abandoned and relaunched whatever `out.md` holds; a status that is
not a number (the member's own file, never overwritten) is no mark,
and one that is a number is the mark whoever wrote it: a member that
writes a number there and is then killed, the `apply` that launched
it gone, is collected by a later `apply` as `ended` with a partial
`out.md`, the price of a mark inside the member's own mount rather
than in a rebuilt image.
The status is read as `docker run`'s would have been (125–127 a
fault, any other non-zero a failure), which the old rule lost. A
run launched before the mark (its `argv` file without the `sh -c`
line) keeps the old rule, so the first restart after the install
relaunches no run that finished. The cost: `sh` is the container's
first process and ignores `docker stop`'s signal, so a member
stopped at the wall clock is killed after the 30 seconds rather than
at its CLI's own exit. **An abandoned worker run** makes 7.6 steps
1 and 2, its commits fetched and kept as its run's ref, as a fault's
are; it posts nothing. **A tree the member left unreadable** (a
mode-000 directory with entries) is given the owner's rights,
directory by directory and never through a link, before its removal
is made again, so `work/` and `check/` no longer outlive a
collection and the pass no longer exits 1 on it. **The worker's
post** drops its text's trailing line breaks before the one blank
line and the block, and its text is cut so that the two hold to
60 000 bytes together, the block never cut (a block over the limit
alone, twenty lines of some 3 KB each, is posted whole after the
truncation note). **The run's stage** no longer falls back to its
home: the pair's second reviewer, launched on review-1-in-progress,
that finds the issue moved to review-0-ready during its run is
skipped with the warning (7.6), and T17's `swarm:blocked` is still
set beside the home where T22 has just taken the issue.
