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
4. **Pacing.** A credential whose provider declares `tokens` in `keys`
   is not given to a launch while the tokens of its runs started in the
   last 24 hours have reached `per_day`, or those of the last 7 days
   `per_week`; the money `budget` stays shown and not enforced.
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
moment a fallback or a hand edit makes it so (built at Step R4-A of
the plan implementing the 2026-11-19 swarm roadmap; before it, the
collection read the current one).

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

**As built: `fallback` declared (Step R4-A of the plan implementing the
2026-11-19 swarm roadmap).** `personas.<role>.fallback` is read
strictly, each entry exactly the persona's four fields; an entry's
`slots` waits for rule 3's build and is refused until then as an
unknown field. Validation reads the persona, then each entry in order,
each field by the persona's rule, naming `fallback[<i>]`; an entry with
the provider, model and effort of an earlier one is refused naming the
earlier (`the same as personas.<role>` or `the same as
personas.<role>.fallback[<j>]`), so one model at another effort is
another entry. An entry's provider without a credential is refused as a
persona's is, `keys: no credential for provider "<p>"`, after `keys`'
own checks. `Format` writes the list after `co_author` on the persona's
line, each entry as one object; `init` asks nothing of it and keeps the
file's, whatever its answers for the persona. Tests:
`TestValidateNamesTheField`, `TestFormatFallback`,
`TestWizardKeepsFallback`.

**As built: the collection reads the launched persona (Step R4-A of the
plan implementing the 2026-11-19 swarm roadmap).** The adapter that
reads a collected run's stdout, its usage, cost and provider fault, is
its record's `persona` line's; a run without the line, launched before
Step 16b of the plan that clears the Deferred list or refused before its
credential, is read by the provider its argv names, and failing that
by `swarm.json`'s, the order its metrics row already took, now one
function for both. Test: `TestSwarmCollectsWithTheLaunchedPersona`.

**As built: `Reset` (Step R4-A of the plan implementing the 2026-11-19
swarm roadmap).** Each adapter has `Reset(fault, at)`. `codex`'s reads
the first `try again at <h>:<mm> <AM|PM>` in the message, `<h>` 1 to 12
and the marker in capitals as the live journal writes them (`12:05 AM`
is 00:05, `12:30 PM` 12:30), as that time on `at`'s day in UTC, or the
next day's when that is not after `at`, so a fault in the very minute
it names is read as the next day's; any other shape states none.
`claude`'s and `grok`'s state none, `claude`'s `usage limit
reached|<number>` among them: the number is no reset a ruling read.
Test: `TestReset`.

**As built: the capacity line, the launch and no pause (Step R4-A of
the plan implementing the 2026-11-19 swarm roadmap).** The collection of
a run in a fault whose message is a capacity fault writes its line to
`<state>/capacity`, the credential from its `credential` line
(`<provider>/env` or `<provider>/<j>`), the model from its `persona`
line, the time `Reset` reads from the run's end, RFC 3339 in UTC, the
reason the provider's message as its `fault:` line holds it; a batch
run's fault the same. A run without either line, launched by an older
Daisy, writes none. Each launch reads the file first and takes the
first entry with a credential: an `env` one no line names with the
entry's model, or a slot of its provider that no running container
holds (the running set read again then) and no line names with the
model; the slot taken is then the first such that is free, as before.
An entry passed over for a busy slot is passed over as a spent one is,
so a role with a fallback no longer waits for its persona's slot. With
none, the role waits before anything of the run is made: no run
directory, nothing posted, and a block the launch's preparation would
have found (a kept file it cannot use, bad slices) waits with it. A
line not of the file's form is the pass's error naming the file and
the line, which the bare `resume` clears. Two faults in a row pause
only when neither is a capacity fault: a capacity fault is read as no
fault by the pause's rule. A line whose time has passed is removed
after the pass's collections and before its launches, its `Capacity:`
line printed; its journal line is not queued, since the journal's
lines are a run's each and this one names none (open, for the
maintainer). The bare `resume` removes the file, printing `Capacity
released: every model is tried again at the next pass`. The worker's
commits, and a notes commit, carry the `co_author` of the entry the run
launched on, found in `swarm.json` by its `persona` line (the persona's
when none matches). `runner.TakeSlot` takes the slots it may take as a
list, which rule 3 narrows. Tests: `TestSwarmFallbackOnACapacityFault`,
`TestSwarmCapacityUntilTheReset`, `TestSwarmResumeClearsCapacity`,
`TestSwarmRoleWaitsWithNoModelAvailable`.

**As built: the note and notify (Step R4-A of the plan implementing the
2026-11-19 swarm roadmap).** At a launch on an issue, once its
container has started, the entry it launched on is compared with the
`persona` line of the role's newest run there by number that has one;
an issue with none is compared with the persona, so a first run on a
fallback is noted and a first run on the persona is not. On a fallback
the note names the persona in `in place of` (not the previous run's
entry) and, after a colon, each line of `<state>/capacity` whose
provider and model are an entry's passed over, `; ` between them; an
entry passed over for a busy slot alone has no line, and with no line
the note ends at the persona. Back on the persona it is `… again.` Its
marker is a reconciler's kind, so no count, trigger or prompt reads
it; `apply` prints `#<n> capacity note posted`. A batch run's change is
not written: the journal's lines are a run's each, and that line
belongs to no run (open, with the removal's line). Notify's item is
each line of `<state>/capacity`, its key the line, told after a fault's
pause. Tests: `TestSwarmCapacityNoteOnTheIssue`,
`TestSwarmNotifiesCapacity`.

**As built: the detached member (Step R4-B of the plan implementing
the 2026-11-19 swarm roadmap).** A member runs `docker run --detach
--rm`, its `sh` writing the CLI's streams to `/job/io/stdout` and
`/job/io/stderr` beside `/job/io/exit`. `docker run --detach` is run
to its end, once the container has started; a status other than 0 (a
missing image's 125) is the launch's failure, `launch-failed` and
`fault: launch: docker run: exit <code>: <docker's first line of
error>`, since no container started and nothing is left to collect,
where an attached run's was *Docker's own exit*, `exited <code>`;
a 125 to 127 of the CLI's, which the `sh` writes, is still `fault:
exit <code>`. This process waits on its member with `docker wait
<name>`, a container already gone (`No such container`) no error, and
keeps the wall clock as a timer of `timeout` from the launch, which
stops it and makes it a timeout. Once the wait returns the collection
reads `<run>/io/exit`: a status is `exited <code> <time>`, `<run>/io/stdout`
and `<run>/io/stderr` copied to `<run>/stdout` and `<run>/stderr`, each read
only when a regular file (a link is never followed, as `out.md`), the
cost and the provider's fault read from that stdout; no status, the
container killed outright, is `abandoned <time>` (7.7), where an
attached run's was `failed: exit 137`. The slot's lock is released once
`docker run --detach` has returned; the secret check reads the slot's
login again at the collection, beside what the launch read. A run
launched attached, before the build, keeps its exit mark,
`"$@"; s=$?; …`, which *ended* still reads. Tests:
`TestMemberArgvDetachedWithStdoutFile`, `TestArgs`,
`TestDockerStartAndTheWallClock`, `TestDockerStartRefused`.

**As built: the step's files (Step R4-B of the plan implementing the
2026-11-19 swarm roadmap).** The check and the dependency step run
`docker run --detach`, their argv under `sh -c '"$@" >/job/step/out
2>&1; s=$?; echo "$s" >/job/step/exit; exit "$s"' sh`, `<run>/step/`
removed and made again before each step starts (a rerun's too, 7.9)
and mounted at `/job/step` after the step's other mounts; every
container Daisy starts is now detached, so the attached `docker run`
and its streams are gone from the runner. At a step's end its `out` is
written to `<run>/check.out` or `<run>/deps.out` (an empty one when it
left none, and at a timeout what it had written), its `exit` the
status of `<step> exited <code>`, 125 to 127 its fault as before. A
step whose container is gone, its apply's or another's, with its
`exit` written is collected from these files; with none, a step
whose apply died or a container killed outright, it is started again,
as a step whose apply died was. Test:
`TestSwarmCheckCollectedFromItsFiles`.

**As built: any apply collects any run (Step R4-B of the plan
implementing the 2026-11-19 swarm roadmap).** A launch records
`deadline <time>` after `started <time>`, `started` plus `timeout`,
and a step `<step> deadline <time>` after `<step> started <time>`: the
line that tells a run launched detached, by this build, from one
launched before it. At each pass a run with `started`, no end line and
a `deadline` line whose container is gone is collected from its files:
with `<run>/io/exit` holding a number, `exited <code> <time>` (the
pass's time), its streams copied, its cost and its provider's fault
read, as its own process's collection does; with none, abandoned. A
run without the line is collected by the rules before (*ended*, its
out.md and no cost). A run or a step of this process whose wait has
not returned is collected the same way once its container is gone from
the pass's running set and its status is written, its wait then
forgotten (request F36): a wait is a wake-up, the files the record. A
container gone with no status while its wait has not returned is left
to its wait, which a timer's stop ends as a timeout. Tests:
`TestSwarmApplyCollectsItsOwnFinishedRun`,
`TestSwarmAdoptedRunKeepsItsCostAndFault`.

**As built: the deadline enforced whoever launched the run (Step R4-B
of the plan implementing the 2026-11-19 swarm roadmap).** A pass that
finds a run's container running, the run launched detached (its
`deadline` line), compares the pass's time with the recorded deadline:
past it, the container is stopped (`docker stop --time 30`) and the run
recorded `timeout <time>` and `failed: timeout after <timeout>`, its
streams copied and its cost read, and collected as a timeout (T17),
not abandoned; before it, the loop holds the run as its own with a
timer at that deadline (`Runner.Watch`: `docker wait`, and the stop at
the deadline, a timeout), which a later pass records as it records its
own launches. `--once` holds none, its passes stopping a run past its
deadline. A step likewise by its `<step> deadline` line, stopped as a
`<step> timeout`. This process's own launches keep their timer of
`timeout`, now set to the deadline the launch recorded. A run launched
before the build keeps *orphan over time*, by `started` and `timeout`.
`failed: timeout after <timeout>` names `swarm.json`'s `timeout` as
before, not the run's own recorded span. `--once`'s drain waits for
the runs it launched, never for one it holds for another process.
Tests: `TestSwarmAdoptedRunStoppedAtItsDeadline`,
`TestDockerWatchStopsAtTheDeadline`.
