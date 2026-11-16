# 0026. The wall clock is `apply`'s, not `--stop-timeout`'s

Moved verbatim from `docs/swarm.md` section 14, item 26, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The wall clock is `apply`'s, not `--stop-timeout`'s.** Docker's
`--stop-timeout` is the grace between `docker stop`'s signal and its
kill, not a run limit. Chosen: `apply` runs `docker stop --time 30` at
`timeout`; `--stop-timeout 30` stays on the argv. Alternative: the
plan's `--stop-timeout` set to `timeout`, which would bound nothing.
