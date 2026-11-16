# 0008. Additions to `<state>`

Moved verbatim from `docs/swarm.md` section 14, item 8, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Additions to `<state>`:** `owner-held` (the batch trigger's brake
after a failed or empty batch, released by `resume`) and `git/` (item
19); `runs/` also holds `argv`, `stdout`, `stderr`, `result` and, from
revision 2, `system.md`, `check.out` and `check/`; `paused` may now be
written by `apply`, with its reason (items 5, 22). Alternative for the
batch brake: pause the whole swarm, which would stop issues in flight
when the backlog runs dry.
