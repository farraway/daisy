# 0035. A timer in `Ops`

Moved verbatim from `docs/swarm.md` section 14, item 35, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A timer in `Ops`.** The wall clock, the 429 wait and the loop
interval need one, and nothing above `Ops` may call `time` itself:
Step 28 adds `Ops.After(d) <-chan time.Time`, with a Fake hook, as
item 32 adds `Lock`.
