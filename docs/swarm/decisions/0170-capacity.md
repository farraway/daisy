# 0170. Capacity: fallback models, adopted runs, slots per role, token pacing and the deadline

From Step 12 of the plan implementing the 2026-11-19 swarm roadmap (R4, capacity is a resource Daisy manages, 2026-11-21). Specified before its build: `docs/swarm.md` section 7.10, with the changes it makes marked in sections 1, 2, 3, 5, 6, 7.2, 7.4, 7.7, 8, 9 and 11; to be built at Step 13 of that plan, the fallback and the adoption first. Amends decisions [5](0005-budget.md) (a token pacing beside the money budget), [24](0024-the-container.md) (the container runs detached), [26](0026-the-wall-clock-is-applys-not-stop-timeouts.md) (the wall clock is the run's record's), [62](0062-what-step-6-built-of-the-lifecycle-and-the-verbs.md) and [89](0089-the-pass-and-the-gates.md) (a capacity fault pauses nothing) and [137](0137-restart-onto-a-new-build-when-idle.md) (a restart waits for no idle pass).

**The maintainer's rulings of 2026-11-19 (Step 1 of that plan, R4):**
ordered models per persona, across providers; a refused model or
provider unavailable until its stated reset; the change noted on the
issue and notified (R4a, reversing section 1's "no fallback on a
provider's failure"). Launches paced to token budgets per subscription
provider that the maintainer declares in `swarm.json`, no defaults; the
money `budget` stays shown, not enforced (R4c). A restart adopts
running containers, each run's stdout to a file in its run directory,
the new process enforcing their wall clock (R4d). And of 2026-11-15
(request F37): "Once an agent consumes 80% of its limit inform it and
ask it to consider breaking up the work."

**The evidence.** Provider quotas and credits stopped the swarm six
times in eight days (2026-11-12 an OpenAI usage limit, 3.4 h paused;
xAI's 402 on 11-14 and 11-18; Fable's credits on 11-15 to 17, five
architect runs failed; OpenAI's limit again from 11-19 04:37Z), each
needing a ruling and a hand edit of `swarm.json`, five of eleven
persona commits forced by quota or undoing such a change. Review slots
on one provider queued nine jobs (I11). New builds waited 23 h in all
for an idle pass (I10); a finished run went uncollected for up to 20
minutes (F36); an architect lost 45 minutes of planning at its time
limit with nothing written (F37). The service journal holds the
messages: `You've hit your usage limit. … try again at 3:53 PM.`
(codex) and `API error (status 402 Payment Required): Grok Build usage
balance exhausted` (grok), the second stating no reset.

**The rules**, quoted in the same words by `docs/swarm.md` section 7.10:

1. **Fallback.** A persona may list `fallback` models in order, and a
   run of the role launches on the first of the persona and its
   fallbacks whose model is available on a free credential of its
   provider. A provider's fault for a limit, a quota or the credit makes
   that model unavailable on that credential until the reset its
   message states, or until `swarm resume` when it states none; it
   pauses nothing and blocks nothing, and each change is noted on the
   issue and notified.
2. **Adoption.** Every container Daisy starts runs detached and writes
   its output and its exit status to files in its run's directory, so
   any `apply` collects a run whose container is gone, with its cost
   and its fault, and stops one at the deadline its run records,
   whichever `apply` launched it. A restart re-executes at the first
   pass's end after the request or a new build, members running.
3. **Slots per role.** A persona's, or a fallback's, `slots` names the
   slots of its provider it may take; `status` shows each role's queue
   and names the bottleneck.
4. **Pacing.** A credential whose provider declares a token `budget` is
   not given to a launch while the tokens of its runs started in the
   last day, or the last week, have reached it; the money `budget`
   stays shown and not enforced.
5. **The deadline.** A member is told its deadline; at 80% of its
   `timeout` Daisy writes `/job/io/deadline`, asking it to write its
   output now or propose how to split the work; a run stopped at its
   deadline has the output it had written posted with its failure.

**The unit of capacity is a model on a credential.** A slot is one
login with its own quota, and the roadmap's own example, the architect
on `claude-fable-5-2` then `claude-opus-5-5`, falls back within one
`anthropic` credential when Fable's credit is spent: so a capacity
fault marks the pair the run used, its credential (`env`, or slot `j`)
and its model, not the provider. Marking the whole provider was the
alternative; it would skip the roadmap's fallback, and a quota that is
the account's costs one more faulted run per model, each failing at its
first request. The faulted run stays a swarm fault (T22, relaunched)
but counts towards no pause: the pause on two faults in a row exists to
stop a broken machine, and a quota is not one. With no entry
available the role waits, nothing posted: no job fails for capacity.

**A stated reset, or the maintainer.** The reset is read from the
fault's message by the provider's adapter (`Reset`), the only reader
of each CLI's words: `codex`'s `try again at <h>:<mm> <AM|PM>` is the
first such time after the fault in UTC, the container's zone (the
image sets no `TZ`), *to confirm* on the next such fault; no other
message is known to state one. A message that states none (xAI's 402,
a balance only a payment refills) leaves the pair unavailable until
the bare `swarm resume`, the maintainer's word, as a fault's pause is
today. A retry after a fixed time was the alternative; it is a limit
no ruling set (praxis rule 16), and a probe run against a spent
balance fails as the first did.

**The collection reads the run as it was launched.** A run's stdout is
read with the adapter of its own `persona` line: the role's persona in
`swarm.json` may be another by the time the run is collected, the
moment a fallback or a hand edit makes it so (as built, the collection
read the current one).

**The note is the issue's, not a stage's.** A run on an issue
launched on another entry than the role's previous run there gets one
`kind=capacity` note, and one more when it is back on the persona; like
the gate note it is no counted entry, so it wakes no role and no prompt
shows it. Notify tells each unavailable pair once, by its line. The
scorecard's By configuration separates a fallback's runs already, which
is how the roadmap asks to see what a weaker model costs.

**Detached containers, their output in files.** The ruling asked for
each run's standard output in a file of its run directory; Daisy makes
every container write its output and its exit status into its run's
directory (the member's `sh` already writes `<run>/io/exit`, the exit mark of decision
[86](0086-the-collections-kept-lines.md), here extended to `<run>/io/stdout` and
`<run>/io/stderr`; a step's into `<run>/step/`), and runs it `--detach`, so no
process holds a container's streams. Any `apply` then collects any run
by its record, with its cost and its provider's fault, which an
*ended* run lost; and the deadline is a line of the record, `started`
plus `timeout`, which whichever process holds the run enforces by a
timer, to the second. Keeping `docker run` attached and having a new
process read `docker logs` of an adopted container was the alternative:
it keeps the output in the daemon's log driver rather than the run's
directory, and needs each container kept after its exit until
collected. F36 (triage item L341: a run of this process whose wait had
not returned was skipped) is gone by construction: no wait is left to
return.

**The restart waits for nothing.** With every run held by its record,
the loop re-executes at the end of the first pass after a request or a
new build, and its successor holds the runs; the idle pass, which kept
builds waiting 23 h, goes. A signal likewise ends the loop at its pass's
end without draining, so `systemctl stop` leaves members running for
the next `apply`. The build is told by size, modification time and
change time (L345: a copy that keeps the times was missed by size and
modification time), hashed when one changes.

**Slots per role and the bottleneck.** `personas.<role>.slots` (and an
entry's) lists the slots of its provider it may take, so the maintainer
gives the reviewers their own logins rather than editing `max_members`
or the slots by hand; absent, every slot, as now. `status`'s
`Capacity:` block shows each role's queue, what holds it and the role
with the most issues waiting for a credential: intervention I11's
remedy. A pool object under `keys` was the alternative; it would name
the roles in two places.

**Pacing by tokens, per credential, over the last day and week.** The
maintainer declares `keys.<provider>.tokens`, no default, counted for
each credential apart, since a slot is one login with its own quota; a
credential that has reached its budget is passed over as an
unavailable one is, so pacing and the fallback are one mechanism. The
windows are the 24 hours and 7 days before a launch, as the providers'
own limits roll, rather than calendar days; the tokens are those the
run's record counts, the cache's included. Spreading the budget evenly
over the window (an allowance growing with the hour) was the
alternative; it is a second limit no ruling set. The money `budget`
stays as section 3 specifies it.

**The deadline told, the warning written, the partial output posted.**
Every Task text names the run's deadline and the file
`/job/io/deadline`, which Daisy writes at 80% of `timeout`: no CLI
takes a message into a running session, and a file the member is told
to look at is the one channel all three have. At the deadline the run
is stopped as before (`docker stop --time 30`, no new grace period),
and the output it had written is posted with the timeout's note, after
the secret check, never read as the role's text, so a half plan informs
the maintainer without becoming the plan.
