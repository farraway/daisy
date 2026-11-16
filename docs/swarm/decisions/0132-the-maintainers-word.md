# 0132. The maintainer's word

Moved verbatim from `docs/swarm.md` section 14, item 132, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The maintainer's word (Step 14 of the plan for the inbox requests
F27–F35, 2026-11-15; the maintainer's feature of 2026-11-12, control
from the forge, its parts (2) and (3)).** A plain comment of the
maintainer's reaches the swarm wherever a member will read it
(section 6, The maintainer's word). **The word** is a counted entry
by a trusted login other than the swarm's that is no `Blocked by
#<m>`: one predicate (`maintainerWord`) for the block and for the
review, so a class statement, a command and `block`'s comment keep
their own meanings, and nothing of the swarm's ever lifts a block or
reopens a head (R4). **A block answered (G1)**: the word posted after
the reconciler's note that blocked the issue, on a thread the
stage's role reads (rule 3's table; the issue's too at a question
passed to the maintainer, whose asker reads them under `#
Questions`; the issue's at the two stages with no role), clears
`swarm:blocked` at step 5, after the commands and before rule 1, so
the role runs in the same pass: Daisy's first clearing of the flag,
on the maintainer's word only. The rule is that a comment lifts a
block when the role that resumes will read it. Alternative: any of
the maintainer's comments on either thread (a comment on the issue
in review would run a reviewer that never reads it, and at the round
limit's second note would lift a block where no role runs, the hole
`unblock` refuses). **A flag set by hand** with no reconciler note
on the issue is lifted by no comment; one set again by hand after
the maintainer's answer (or `unblock`'s comment) reads as that block
answered and is lifted at the next pass: the forge seam gives a
label no time, and a blocked gate note cannot mark the flag, being
posted at step 8, after step 5 has lifted it. Alternative: a record
of each lift (unblock runs on any machine, so `<state>` would miss
its lifts) or a timeline read added to the seam. The maintainer's
holds remain `pause <n>` and `block --by`. **In review (G2)**: the word
among the current pull request's comments is a reopening of the
head beside the worker's answer without a commit (`Reopenings`, one
list every reader of a review's standing takes: the trigger rule,
the collection's pair, the rounds, the reviewer's Task round, the
prompt's rounds, `status`, the gate note and the issue's pulls
block), so both reviewers run again, whichever the comment names,
and the pair completes on reviews that both followed it. Rule 3
cannot tell whom a comment addresses, and a pair half of which never
read the comment would decide on half of it. Cost: two runs per
comment, the comments posted before the pair launches answered
together. A head reviewed by both after a comment is a round, as
after an answer, so the limit still bounds it; in the review stages
a complete pair on the head is found only at the round limit's
second note, so the count changes nowhere else. Alternative: only
the pair's standing widened, the rounds and the prompt keyed by the
answers alone (a reviewer run again would not see its own review of
the head the comment answers, which sits in the round hidden from
it). The maintainer's reviews and line comments are not read for
it, the reviewers' turn reading the comments, nor the issue's
comments, which no reviewer reads. **The stages no role reads**:
at `swarm:plan-2-in-progress` rule 2 holds every role while the
worker runs, so the plan's architect could not run there; a comment
is held and read by the role of the stage the run leaves the issue
at, on the thread it reads, which needs no new rule. At
`swarm:review-2-approved` the comment wakes no one and does not hold
the merge step (`pause` does): waking the worker or the reviewers
needs a transition out of the stage and changes what the merge gate
decides, which is the maintainer's to choose (the plan's Open
Questions). **No new label**, and `unblock` unchanged (it still
refuses at the round limit's second note, posting on the issue).
Tests: `TestSwarmApplyBlockAnswered`, `TestSwarmApplyReviewComment`,
`TestSwarmApplyQuestionAnsweredInReview`, `TestLifting` and
`TestLaunchInReview`.
