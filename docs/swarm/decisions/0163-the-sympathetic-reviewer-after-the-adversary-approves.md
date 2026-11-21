# 0163. The sympathetic reviewer after the adversary approves

From Step R3-10 of the plan implementing the 2026-11-19 swarm roadmap (the maintainer's ruling of 2026-11-20). Built: `docs/swarm.md` sections 4 (the sympathetic's row and default prompt), 5 (the stages, the in-progress stages, T10, T21, Runs that do not move the label), 6 (rule 3's table, the role rule, the rounds, The maintainer's word), 7.3 (the rounds), 7.6 (the reviewers' calls), 7.8 (a round's file) and 11 (the defaults' history). Amends decisions [78](0078-the-pass-does-not-wait-the-pair-in-one-pass.md) (the pair in one pass) and [14](0014-sequential-reviewers-each-blind-to-the-other-on-the-head.md) (the blindness, as it applies).

**Why.** The two reviewers cost about the same (about $525 and $581
reported over some 520 runs each), and the adversary's blocking
findings are fixed 121 times against the sympathetic reviewer's 46
(decision [142](0142-a-reviews-effect-read-from-the-runs.md)'s
effect). A sympathetic review of a head the adversary sends back is
spent on code about to change. The maintainer's ruling: launch the
sympathetic reviewer only once the adversary approves the head; and,
since he does not read its reading guide, its default prompt drops it.

**The order.** On `swarm:review-0-ready` only the adversary launches
(T21 writes `swarm:review-1-in-progress`, as before). The sympathetic
reviewer launches on a head only once the adversary's review of that
head, standing since the head's newest reopening (section 6), approves.
A pass collects before it launches (section 11), so in `apply`'s loop
the sympathetic reviewer is launched in the pass that collects the
adversary's approval, on the `review-1-in-progress` the adversary's
launch wrote; with `--once`, whose drain collects after the pass's
launches, in the next pass.

**The pair complete.** The adversary's request for changes completes
the pair on the head with its one review: T10 below `max_rounds`, T11
at it, and no sympathetic review. The adversary's approval leaves the
pair open, the issue at `review-1-in-progress`, for the sympathetic
reviewer, whose review completes it: both approving, T9; its request
for changes, T10 or T11.

**The rounds, counted as before.** A round is a head (with the
reopenings before its reviews) on which the pair is complete: the
adversary requested changes, or both reviewers reviewed it. So each
review cycle counts once, as a cycle of the pair in one pass did, and
`max_rounds` bounds the same number of cycles. A head the adversary
approved and the sympathetic reviewer has not reviewed is no round
yet, as a head one reviewer alone had reviewed was none. `This is
round <r>`, the gate note's and `status`'s rounds, `<r>` of a round's
file in the job's workspace (7.8) and the reviewers' `review_rounds`
follow the one count. The reviewers' run numbers no longer
advance together (the sympathetic reviewer runs only on approved
heads), so a round limit's note can be word for word an earlier one of
the other reviewer's run of the same number (round 1 of 1 again after
a re-plan): a collection takes a reconciler note for its own only when
it was posted at or after its run started.

**Reopenings.** The worker's answer without a commit (`NO CHANGE:`) and
the maintainer's comment on the pull request reopen the head as
before: no review of the head before the reopening stands, the
adversary's approval among them, so the adversary reviews the head
again first and the sympathetic reviewer follows once it approves. A
sympathetic review submitted after the reopening (its run launched
before it) stands, and the adversary's new review completes the pair.
One reopening now costs the adversary's run, and the sympathetic
reviewer's when the adversary approves.

**A reviewer that fails or goes stale** ends as before (T22, the issue
back at `review-0-ready`). After the adversary's, the adversary is
launched again and the sympathetic reviewer still waits; after the
sympathetic reviewer's, the adversary's approval standing, the role
rule launches the sympathetic reviewer alone, its T21 writing
`review-1-in-progress` again. A stale run's head has moved, so no
review of the old head stands on the new one, and the adversary is
first again.

**Blind as it applies.** Decision 14's blindness stands: the
sympathetic reviewer's prompt still leaves out the current round, so
it does not read the adversary's approval of the head; the adversary
never meets a sympathetic review of the head it reviews. Alternative:
showing the sympathetic reviewer the adversary's approving review,
which would spend its independence for findings the adversary already
judged non-blocking.

**The scorecard.** Its review rows read as before: `review rounds` the
highest `review_rounds` of a slice's records, which counts each
completed round, an adversary's request for changes alone included;
the runs by role show the sympathetic reviewer's fewer. A review's
effect (decision 142) is read as before, from its role's next posted
review on the issue, which must be of the next round: a sympathetic
request for changes whose next sympathetic review follows a round the
adversary alone completed reads unknown.

**Alternatives.** Keeping the pair in one pass (decision 78), which
spends the sympathetic reviewer on heads the adversary sends back; the
sympathetic reviewer first, which the effect figures argue against;
counting a round only when both reviewed (the old words), under which
an adversary's request for changes would never count and `max_rounds`
would never stop a pull request the adversary keeps sending back.

Tests: `TestSwarmSympatheticAfterTheAdversaryApproves` (the adversary
alone on review-0-ready, the sympathetic reviewer at the next `--once`
pass and T9; an adversary's run whose apply died collected and the
sympathetic reviewer launched in that one pass; an adversary's request
for changes reaching T10 with one review and, at `max_rounds` 2, T11 on
the second such round, no sympathetic run; the round limit's second
time reached by sympathetic run 1 after adversary run 1's first),
`TestLaunchInReview`,
`TestPullState`, `TestDefaultHistoryHoldsEachCurrentDefault`.

**Amended by Step R3-12 of the plan implementing the 2026-11-19 swarm
roadmap (2026-11-21).** The scorecard paragraph's last sentence no
longer holds: a review's effect is read from its role's next posted
review on the same pull request whatever round (decision
[142](0142-a-reviews-effect-read-from-the-runs.md), amended), so a
sympathetic request for changes whose next sympathetic review follows a
round the adversary alone completed is credited as any other.

**Amended by decision
[168](0168-a-delta-review.md)
(Step E2 of the plan implementing the 2026-11-19 swarm roadmap).**
Each reviewer's round r > 1 text starts from its own last review, so the sympathetic reviewer's delta may span several heads; a sympathetic reviewer whose first review falls in a later round reviews the whole change, with no round sentence.
