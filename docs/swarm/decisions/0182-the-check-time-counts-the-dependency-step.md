# 0182. The check's time counts the dependency step

From Step S9 of the plan implementing the 2026-11-19 swarm roadmap (the
backlog item L348). Built in `docs/swarm.md` section 11 (`swarm
status`'s Throughput, `stage time`).

**Why.** A worker run whose repository declares `deps` runs the
dependency step before its check (7.6, step 4), and that step is part
of what the check costs the run. `stage time`'s `check` counted only
`check started` to `check exited`, so a repository whose dependency
step grew slower showed no change there.

**The rule.** The `check` span starts at the `deps started` line that
precedes its `check started`, when there is one, and at `check started`
otherwise; a rerun of the check alone (7.9) starts its own span, as
before. Pruning `<state>/deps/<name>` stays the maintainer's: the
cache refills at the next check.

**Alternatives.** A `deps` entry of its own in `stage time` would show
the step apart, but adds a column the backlog item did not ask for; a
run whose dependency step failed, with no check, is still not timed,
where counting it to `deps exited` would mix a failed step into the
check's median.
