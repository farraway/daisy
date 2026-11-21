# 0078. The pass does not wait; the pair in one pass

Moved verbatim from `docs/swarm.md` section 14, item 78, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The pass does not wait; the pair in one pass (Step 18 of the
swarm build, 2026-11-08).** On the first two days the unit was a
timer running `apply --once`, which waits for every member it
launched, and the timer counted `interval` from that exit: launches
came every 14 to 27 minutes, a ready review waited for the pass's
longest run, and the pair ran adversary then sympathetic, two passes
a round. **The unit is a service running the loop** (`Restart=on-failure`,
`RestartSec` the interval, the timer retired), which already ran a
pass every `interval`, start to start, and collected each of its runs
at the first pass after it ended; `--once` keeps its meaning for the
smokes. The alternative the retrospective named, `--once` returning
after its launches and the next process collecting from the disk,
would lose every run's exit status, cost and provider fault, which
live in the process that waits on `docker run`. That loss is taken
only where it cannot be avoided: **a run whose `apply` died** (a
crash, a second signal, systemd's stop timeout) is collected by the
next `apply` from its `out.md`, its end line `ended`, instead of
abandoned and relaunched; abandoned stays for a run with no
container and no `out.md`, and for an orphan stopped over `timeout`.
`KillMode=mixed` keeps a stop's SIGTERM from reaching the members
through `docker run`'s signal proxy, which would end every running
member at each restart as a failure of the work (exit 143, T17).
**Rule 2 is read per reviewer** in the review stages (item 14's
alternative, which replaces its sequence): both reviewers launch in
the same pass, blind to each other on the head as before (the prompt
leaves out the other's review of the head, 7.3), the second launched
on the stage the first's launch wrote, so review-1-in-progress is
written once for the pair. **An issue's roles are decided once a
pass**: otherwise the pair's launch, moving the issue to
review-1-in-progress, which the pass reads later, would launch again
a reviewer whose launch had just failed, under the same run number.
`interval` already allowed `1m`. Where the step left a choice the
smaller was taken: no stdout line of its own for an `ended` run (its
result says it), and `status`'s stage times leave it out, as they
leave an abandoned run, its time being when a pass found it.

**Amended by decision
[163](0163-the-sympathetic-reviewer-after-the-adversary-approves.md)
(Step R3-10 of the plan implementing the 2026-11-19 swarm roadmap).**
The pair no longer launches in one pass: the adversary launches alone,
and the sympathetic reviewer once the adversary's review of the head
approves, in the pass that collects that approval; an adversary's
request for changes completes the pair alone. Rule 2 is still read per
reviewer, and an issue's roles are still decided once a pass.
