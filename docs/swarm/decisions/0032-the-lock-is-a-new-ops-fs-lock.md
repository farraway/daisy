# 0032. The lock is a new `Ops.FS().Lock`

Moved verbatim from `docs/swarm.md` section 14, item 32, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The lock is a new `Ops.FS().Lock`** (an `flock` the kernel releases
on exit) added at Step 29. Alternative: an exclusive `Mkdir`, which a
crashed `apply` leaves behind for the maintainer to remove.
