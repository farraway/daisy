# 0137. Restart onto a new build when idle

Moved verbatim from `docs/swarm.md` section 14, item 137, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Restart onto a new build when idle (Step 19 of the plan for the
inbox requests F27–F35, 2026-11-15; S3).** A live swarm's watch
session ran three rounds of `swarm pause`, waiting for the running
members, `systemctl restart` and `swarm resume` to put a new build
in service, and a `systemctl restart` sat in `deactivating` for
more than ten minutes while the stop drained its members. **The
loop re-executes itself** (`execve`, the same PID), in place of
exiting for systemd to start it again: it works without systemd (a
loop run by hand), and the unit sees no stop, so no drain is waited
on. **The build's identity** is its content's SHA-256, read at the
loop's start and again only when the file's size or modification
time changes (a stat per pass); the alternative, the stat alone,
restarts on the same build written again. **A changed file waits
one pass** before it counts: a pass that lands while the build is
being written (`go build -o` copies onto a new file when its work
folder is on another file system) would otherwise execute half a
binary; the cost is one `interval`. **The lock is released by the
exec**, its descriptor close-on-exec, and not before it: releasing
it first, as the plan's text reads, leaves a failed exec running
without its lock; the window between the exec and the new
process's lock, in which a hand-run `apply --once` may take it, ends
in a refused start that `Restart=on-failure` retries. **Idle** is
rule 2's notion on every issue at once: nothing running and every
run that started or ended collected, so a re-exec loses no
collection (a container this process started would be collected
by the new process from its `out.md` as *ended*, 7.7, but its cost
and a provider's fault would be lost). It is looked for after a
pass, when the pass has just collected what ended. **No bound** is
put on the wait: no cap was invented, and a busy swarm is made idle
by the maintainer's `swarm pause`. **The request**, `swarm
restart`, is a file in `<state>`, as `paused` is, removed before
the exec so the new process does not take it again; `status` shows
it. A failed exec is a warning and a journal line, and the build
that failed is not tried again until another replaces it.
`Ops.Reexec` is the loop's new effect: `ReadOnly` refuses it, and a
`Fake` without a `ReexecFn` refuses it rather than fall back to the
real exec, so no test replaces its process. Tests:
`TestSwarmApplyRestartsOntoANewBuild`, `TestSwarmRestartRequested`,
`TestSwarmApplyRestartNotOnTheSameBuild`,
`TestSwarmApplyRestartExecFails`,
`TestSwarmApplyRestartWaitsForTheCollection`,
`TestSwarmApplyRestartSignalled`, `TestReexecRefused` and
`TestLockIsReleasedByAnExec`.

**Amended by decision [170](0170-capacity.md)** (R4, specified ahead of its build, Step 12 of the plan implementing the 2026-11-19 swarm roadmap): the loop re-executes at the end of the first pass after the request or a new build with members running, the new process holding each run by its record, and a new build is told by its change time too (`docs/swarm.md` section 7.10).
