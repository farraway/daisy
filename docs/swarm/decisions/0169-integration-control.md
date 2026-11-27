# 0169. Integration control: declared dependencies, a merge queue and a red main

From Step 12 of the plan implementing the 2026-11-19 swarm roadmap (R2, integration control, 2026-11-21). Specified before its build: `docs/swarm.md` section 12.1, with the changes it makes marked in sections 1, 2, 5, 6, 7.3, 7.4, 7.7, 7.8, 7.9, 10, 11 and 12; to be built at Step 13 of that plan. Amends decisions [51](0051-dependency-order-and-pins.md) (dependency order beyond a job's slices), [81](0081-conflicts-handled-by-the-swarm.md) and [93](0093-the-collection-and-the-merge-step.md) (a conflict found before the merge is tried), and [135](0135-the-owners-hold-released-by-its-issues.md) (the hold's release is rule 1's test).

**The maintainer's rulings of 2026-11-19 (Step 1 of that plan, R2):**
machine-read `Depends:` and `Touches:`, several blockers per issue,
the architect marking an overlap safe (R2b, reversing section 1's
"the owner's depends on is prose"); a serial merge queue per
repository, the check on `base` plus the head as they would merge, a
queued head checked again when `base` moves, a failure back to the
worker with the log (R2a); `base` checked after each merge, worker
launches on its repository held while it is red, the fix routed to the
breaking job's worker if its job is open, else the owner's next
proposal (R2c, reversing section 1's "post-merge signals are not
read"). The roadmap's R2d, the owner's hold as the dependency graph,
was not put as its own question; it follows from R2b, and its other
half, the owner run on events, is R10's.

**The evidence.** `base` was broken twice by two merges each green
alone (2026-11-17, #132's benchmarks against #133's import guard;
2026-11-18, #153 against #149's lane guard), blocking six jobs; the
service log holds 54 "not mergeable: back to the worker" bounces; seven
stops were prerequisites the owner had stated in prose; the owner held
its batch 78 times, released by hand about 45 times. Daisy checked a
head against the `base` it branched from, never against the `base` it
would join.

**The rules**, quoted in the same words by `docs/swarm.md` section 12.1:

1. **Dependencies.** An issue's dependencies are the issues its newest
   `Depends:` declaration names (the owner's or the QA's proposal, the
   accepted plan, or a maintainer's comment that is the line alone),
   with each issue named by the `Blocked by #<m>` comments that are the
   maintainer's newest on it. While one of them is open, nothing
   launches on the issue and the merge step merges nothing of it.
2. **Touches.** A worker's first build of a slice is not launched while
   the slice overlaps an open slice of another issue in the same
   repository, unless either slice's item in its plan names the other's
   issue on a `Safe with:` line. Two slices overlap when a `Touches:`
   pattern of one matches a path the other's branch changes, or when a
   pattern of each may match one path.
3. **The merge queue.** Each repository's approved heads form one queue,
   in the order their pairs completed, and Daisy checks them one at a
   time: the first without a passed merge check on `base` as it is now.
   The merge check runs the repository's check, in its declared
   environment, on the commit that merges the head into `base`. Neither
   the merge step nor `swarm merge` merges a head without a passed merge
   check on `base` as it is now; a failed one goes back to the worker
   with its log.
4. **The red main.** After every merge, and whenever `base`'s tree is
   one no check has passed, Daisy checks `base`. While its check fails,
   no worker launches on the repository and its queue checks no head,
   but for the fix's: the fix is given to the worker of the job whose
   merge broke it when that job is open, and is otherwise the owner's
   next proposal.
5. **The owner's hold is the graph.** The owner proposes work that waits
   on open issues with a `Depends:` line naming them, rather than
   holding its batch; a batch hold that waits on issues is rule 1's
   dependencies of issue 0, released by the same test.

**Several blockers, kept compatible with the one.** Today the newest
maintainer comment on an issue holds it when it is `Blocked by #<m>`,
and any later comment of the maintainer's lifts it. Rule 1 reads every
such comment since the maintainer's newest other one, so `swarm block`
run twice in a row holds by both and any other comment still lifts
them all: a `Blocked by` posted before the build reads as it did. A
`Blocked by` that lasted until its issue closed, whatever the
maintainer wrote after, was the alternative; it would change what an
existing thread holds at the build. A declaration (`Depends:`), the
owner's, the architect's or the maintainer's, is the lasting form: the
newest stands, so the architect refines the owner's and the maintainer
overrides both, and it is no counted entry, as a class statement is
not, so it wakes no role.

**The overlap errs towards holding.** Glob against glob has no exact
test in `path.Match`'s syntax, so two patterns are taken to overlap when
their literal prefixes nest or either matches at any depth, and a
pattern also overlaps every file the other slice's branch already
changes, which `branches.md` lists (decision
[158](0158-the-jobs-workspace-is-the-memory.md)). A false hold costs
one slice's wait; a false launch costs a conflict (T25) and a worker
run of 4 to 7M input. Only a slice's first build is held: a revision's
pull request is open already. A slice that declares no pattern is held
by no declaration; holding it beside everything (the safe side, as an
unclassed job is architecture) was the alternative, and would serialise
every repository at the build, where no open job carries the line.

**The queue is computed, serial in its checks, and holds no merge.**
The queue's order is the pairs' completion, read from the forge at each
pass, so no file can drift from the forge. One merge check runs per
repository at a time, on the first queued head without a passed check
on `base` as it is now; a passed head with the merge gate on waits for
the maintainer without holding the next, so the maintainer merges in
any order and each merge makes the others stale; with the gate off a merge
ends the repository's merges for the pass, the step today going on to try
the next head, whose check is stale. A batched queue (the
heads merged together, bisected on a failure) was the alternative the
ruling set aside. The merge check runs the repository's check in its
declared environment (7.9), on a merge commit Daisy writes on the
mirror with `git commit-tree` from `git merge-tree`'s tree, which a
check clones as it clones a worker's run ref; a conflict found there is
sent back at once (T25), before the forge is asked, with the merge gate
on as off, since sending a head back merges nothing and the ruling
sends the failure back. A failed merge check is a new transition, T32,
not T26, whose note names the forge's checks: its note names the log
the job's workspace keeps, which the worker reads.

**A tree checked green is green.** After a merge of a passed head the
forge's merge commit has the tree the merge check passed (the forge
merging `base` and the head as `merge-tree` does), so `base` is green
with no further run, and a main check runs only for a tree no check has
passed: a push by hand, a merge by hand on the forge, a forge that
merged otherwise. Running the check on `base` after every merge was the
alternative, a second check of one tree. Reading CI's status on `base`
was the roadmap's other way; it is left out, as CI's status on a head
is (decision [23](0023-the-member-image-is-a-base-daisy-runs-the-products-check.md)): Daisy's check runs the declared environment, which R3
made the one CI builds.

**The red main and its fix.** While `base` is red, worker launches on
its repository wait and its queue checks only the fix's head, which
must pass on the red `base`; reviewers, the architect and the owner go
on. The breaking job is the job of the newest merge since the last
green commit that a swarm worker pushed. Open, it is the fix's: a
counted note on its issue wakes its stage's role, and its next worker
run makes its branch's first commit the fix. Closed, or with no swarm
merge to blame, the owner's next batch proposes the fix first, marked
`Fixes: <name>`, released from `owner-held` by the red. A main check
that fails in the environment, after 7.9's rerun, leaves `base`
unchecked (a line of `<state>/main-unchecked`) and holds the queue on that commit until the bare `swarm resume`, the maintainer's,
as R3c sends a persisting environment failure to the maintainer.

**The owner's hold as the graph.** The owner held its batch because
the next work waited on open jobs; with dependencies declared it
proposes that work at once, `Depends:` naming them, and the job is held
until they close, so a `BLOCKED: waiting on` hold is the rare case. The
hold's `waits` line is read as issue 0's dependencies by rule 1's
test, every one closed, which is today's release (decision 135) in one
function with the issues'. Releasing the hold when any named issue
closes (intervention I8's remedy) was the alternative; with dependent
proposals there is little left to release, and the pacing,
`owner.max_open_proposals`, already bounds how far the owner proposes
ahead. The triage's L373 (the release as built) is superseded by this
reading, and L346 (a required context not yet reported) is folded into
section 12's checks.

**What it removes.** The owner's prose dependencies (section 4, "a job
only once every job it depends on is closed") become a declaration
Daisy reads; the worker's `BLOCKED:` with cause `prerequisite` or
`main-red`, and the maintainer's `resume` and `block --by` for
ordering, are the roadmap's measures of it, each to be near 0.

**As built (Step R2-A, dependencies and touches; rules 1, 2 and 5).**
`Dependencies`, `Deps`, `OpenDependencies`, `openIssues`, `SliceTouch`,
`ProposalTouches`, `Overlap` and `(touchRead).overlapping` in
`internal/swarm/reconcile/depends.go`; `BlockedBy` in `rule.go` returns
every `Blocked by #<m>` among the maintainer's comments since the newest
that is not one (a class statement, a command and a declaration aside),
so `swarm block` run twice holds by both and `unblock` posts each again;
a declaration is no counted entry (`counted` in `thread.go`). The pass
reads the job's plan before rule 1's hold, so the plan's declaration
counts; the merge step (gate off) reads the dependencies before it asks
the forge to merge, an already merged head going on as before; the
owner's hold reads its `waits` through `openIssues`. `Touches:
none` names no pattern, so it holds nothing beyond the branch files.
`status` reads the overlaps from the mirrors as their last fetch left
them (`git merge-base` is allowed under `ops.ReadOnly`), JSON
`held.overlaps` beside `held.blocked_by`. The gate notes' `- Depends:`
names every dependency, the declaration's and the `Blocked by`'s; the
plan gate's `- Touches:` is every slice's patterns, each once. Not built
here: the cycle warning of rule 1 (`depends on itself through …`; a
cycle is held, every issue in it being open, but nothing warns), the
queue (rule 3) and the red main (rule 4). Tests: `TestDependencies`,
`TestBlockedBy` (extended), `TestOverlap`,
`TestTaskTextsAskForDependencies`, `TestSwarmApplyHoldsADependentJob`,
`TestSwarmMaintainerHoldsByTwo`, `TestSwarmStatusHeldByDependencies`,
`TestGateNoteNamesDependencies`, `TestSwarmWorkerHeldByAnOverlap`;
`TestSwarmApplyBatchReleasedByItsIssues` unchanged.

The overlap is read only for a launch that can follow (a member slot free), after the mirror's fetch, with one `Pulls(n)` for each other issue, and the dependencies' hold stands before the current pull request's thread is read, so a held job costs the forge its dependencies' `Issue(m)` alone.

**As built (Step R2-A, the maintainer's rulings of 2026-11-26 on its
open questions).** An issue the forge does not hold holds nothing, for
an issue's `Depends:` and for the owner's batch hold alike: `release` in
`internal/swarm/reconcile/batch.go` warns `⚠ owner batch: #<m>, which
its hold waits on, names no issue; not waited on` and releases the hold
once every other issue it waits on is closed (section 6 amended; status
adds no warning). The default owner prompt (`DefaultPrompts["owner"]` in
`internal/swarm/declare/prompts.go`, section 4) proposes a job whose
dependencies are open, naming them on its `Depends:` line, in place of
proposing it only once they are closed; its hash joins
`defaultHistory`, so `swarm prompt build` brings an unedited owner
prompt to it. Tests: `TestDefaultOwnerProposesDependentWork`,
`TestSwarmOwnerHoldIgnoresAMissingIssue`.

**As built (Step R2-B, the merge queue; rule 3, with L346 and the
maintainer's ruling 5 of 2026-11-21).** `MergeCommit` in
`internal/swarm/reconcile/git.go` (`git merge-tree --write-tree` on the
base commit, `git commit-tree <tree> -p <base> -p <head>` as `reconciler
(Daisy swarm)` at the commit email, `refs/swarm-merge/<n>-<s>`);
`mergeQueue`, `queuedHeads`, `startMerge`, `collectMerge`,
`mergeFailed` (T32), `mergeEnvironment` and `MergeChecked` in
`queue.go`, the run `<n>-merge-<k>` its steps alone, its container
`<…>.<n>.merge.<k>` labelled `#<n>/merge`, the check cloned from the
merge ref (`stepRef`, `stepName`). The merge step and `swarm merge`
merge no head of a repository with a check without a passed merge check
on `base` as the mirror holds it (the merge step fetching it first, the
verb reading the last fetch); a merge ends its repository's merges for
the pass. The queue reads the forge only when a check can start, and
not for a head its records show passed or running on `base` now: with
the gate off its heads are the merge step's (no read of its own), with
it on the review-2-approved issues step (5) read, then, for a head not
so recorded, its dependencies, `Reviews(m)` and `PullComments(m)`. The
merge step reads no `Status` for a head whose merge check has not
passed. `RequiresChecks(repo, branch)` returns the required contexts
(Gitea's `status_check_contexts`, only when `enable_status_check`),
read once no context failed or is pending, a context matched with
`path.Match`. A head the mirror cannot merge (git's error) is blocked
with the reconciler's note naming the pull request, the head and
`base`, never merged unchecked. Differences from the words above:
`swarm merge`'s refusal names `(queued)` or `(running)`, not the place
in the queue. Not built here: rule 4 (the main check, a red `base`
holding the queue), the merge gate note's `- Merge check:`, `status`'s
queue line, the replay of a merge check's log into a rebuilt workspace.
Tests: `TestMergeCommit`, `TestSwarmMergeCheckRunsOnBasePlusHead`,
`TestSwarmMergeQueueMergesOnlyACheckedHead`,
`TestSwarmMergeCheckFailedBackToTheWorker`,
`TestSwarmMaintainerMergeWaitsForTheMergeCheck`,
`TestSwarmQueuedHeadRecheckedWhenBaseMoves`,
`TestSwarmMergeWaitsForAnUnreportedRequiredContext`,
`TestSwarmMergeCheckBlocksAnUnreadableHead`; the gate-off tests that
merge take one pass more, for the check.
