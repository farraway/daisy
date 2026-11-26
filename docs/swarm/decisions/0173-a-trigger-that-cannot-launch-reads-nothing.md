# 0173. A batch trigger that cannot launch reads nothing

From Step D7 of the plan implementing the 2026-11-19 swarm roadmap (the swarm meets its exit criteria: merges a day, few interventions). Built in that step: `docs/swarm.md` section 11 (apply, A pass, step 7, and its As built). Amends no decision: it orders the checks of step (7)'s two triggers, the owner's (section 6, G0) and the QA's (decision [151](0151-a-qa-run-after-merges.md)), and changes no condition of either.

**Why.** A pass's step (7) read everything its trigger's prompt needs
(the open swarm issues, the class of each accepted one, the follow-ups,
and the recently closed list: every closed swarm issue, then one
`Pulls(n)` and one `Issue(n)` for each issue on it) before it asked the
launch to start the run, and the launch refused when every member slot
was taken, the swarm was paused or the loop was signalled. In a busy
swarm step (5) fills `max_members` at every pass, so every pass paid the
trigger's reads and launched nothing. Measured on a live swarm on
2026-11-25: with no owner run and no hold, passes took a median of
about 350 s (11 passes, then 7), against about 65 s while the owner ran
or was held, which short-circuit the trigger; the closed list held 206
issues, so more than 400 forge calls a pass, each `Pulls(n)` on the
gitea adapter listing every pull request of every declared repository.

**The rule.** Each trigger asks first whether the pass launches nothing
more, by the same function the launch asks: no member slot free (the
swarm's containers running at the pass's start and those the pass
started since, a check included, rule 4), the swarm paused (a second
fault in a row included) or a signal received. When it does, the trigger
makes no read and launches nothing, as before. When a slot is free the
trigger reads and launches exactly as before.

**Alternatives.** Copying the condition into the triggers was the
smallest change; two copies of rule 4's count drift, so the condition is
one function the launch and both triggers call. Running step (7) before
step (5), so that a due trigger takes a slot first, changes who runs and
is a ruling of its own, not a cost fix. Caching the gitea adapter's
pull request listing, or reading the closed list more cheaply, lowers
the cost of the pass that does launch; both are outside this step.

**Left as it was.** A launch refused later, by the credential's slot or
`<state>/capacity` (rule 4's last part, 7.10), still pays the reads: the
role's runner is chosen inside the launch. The pass that does launch
the owner still reads one full pull request listing per closed issue.

**As built (Step D7 of the plan implementing the 2026-11-19 swarm
roadmap).** `launchesNothing` on the pass holds the condition the
launch closure in `Pass` held inline, the running count now the pass's
field `mine`; the closure, `batch` and `qa` call it, the triggers before
their first read of any kind (the hold's file, the runs, the fetch).
Test: `TestSwarmBatchTriggerReadsNothingWithNoSlotFree`, for each
trigger a pass with the one slot taken reading what the same pass reads
with the trigger held, and with a slot free the trigger's run launched
and the closed issue's `Pulls(n)` and `Issue(n)` read.
