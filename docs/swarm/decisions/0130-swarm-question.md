# 0130. `swarm:question`

Moved verbatim from `docs/swarm.md` section 14, item 130, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`swarm:question` (Step 12 of the plan for the inbox requests
F27–F35, 2026-11-15; the maintainer's feature of 2026-11-12: worker
or reviewer → architect → owner → maintainer).** A member that needs
a fact the plan does not give had two ways out, guessing or
`BLOCKED:`, the second the maintainer's every time. A worker's or a
reviewer's `QUESTION:` gives the issue the seventeenth label, the
flag `swarm:question`, under which the architect, then the owner,
answers or passes the question on, and only the owner's `QUESTION:`
reaches the maintainer, as a block (section 6, The question; T28–T30).
**Where it is posted**: on the issue, whatever the asker, as the
asker's role. The architect and the owner read the issue thread, so
every level sees the question and the answers in one place; the
pull request's thread is left as it was, so the worker on
`review-3-changes-requested`, whose turn reads it, still finds the
review it was asked to answer newest, and a reviewer still has no
review of the head: neither needs a rule to resume. Alternative: the
asker's own thread, the pull request's for those three, with a
reconciler note mirroring it to the issue (two posts per question,
and the asker's own question newest in its thread, so it would not
run again on the answer without the answer posted there too).
**The level is derived from the thread**, not labelled: the owner
answers when the newest of the swarm's questions is the architect's,
else the architect; the owner's question clears the flag, so a third
level never reads the flag. Alternative: a level marker in a
reconciler note or a label per level (state kept twice). **The
answer is marked** `ANSWER:`, added by Daisy when the member's text
does not begin so, because every reader of "the architect's newest
comment" (the accepted plan, the rulings' basis, `status`'s plan
revisions, the plan gate's regating) must tell an answer, and a
question, from the plan: an unmarked answer on `plan-1-accepted`
would be read as a plan of one slice. Alternative: telling an answer
by its place after a question (an answer and a plan revision after
a hand-cleared flag read alike) or by the run's `result` (a thread
reader would read every run's file). The run itself records
`question` at its launch, so its collection treats its output as an
answer even when the maintainer removed the flag during the run (a
plan revision's relabel, T27, would follow otherwise). **What is not
an answer**: `BLOCKED:` from the architect or the owner, or a failure
of its work, is T17, which now clears `swarm:question` beside the
block it sets, so the question goes straight to the maintainer and
`unblock` resumes the asker; a swarm fault keeps the flag and the
answerer runs again. **The Task texts**: the worker's and the
reviewers' gain one sentence offering `QUESTION:` in place of a
guess (7.4), the feature's contract; the architect's and the owner's
question texts are new rows. **The asker's prompt** gains `#
Questions`, the exchange since the first question, left out when
there is none so that every other prompt is unchanged; a reviewer
reads no issue thread, so the maintainer's answer to a question that
reached the maintainer reaches it there. **No cap**: each level
answers once or passes the question on, so it reaches the maintainer
in at most three hops; the questions a job asks are not counted (the
maintainer's direction). **The rulings wait** while the flag is on:
a ruling noted under the flag would run the architect on the
question, after which its answer, no basis, would leave the note
following the basis and the architect's own word newest, and nothing
would run. **The label**: an organization's labels are made by its
owner on Gitea (item 80), so `apply` stops at its start, naming
`swarm:question`, until the maintainer creates it once. Tests:
`TestSwarmApplyQuestionAnswered` (the worker's question, the
architect's answer, the worker again with it under `# Questions`
and the plan unchanged), `TestSwarmApplyQuestionToTheMaintainer`
(architect → owner → maintainer: blocked with the note naming the
question, nothing launched, `status`'s `question #1`, `unblock`
resuming the worker), `TestSwarmApplyReviewerQuestion` (the
adversary's question answered with the pull request in the
architect's prompt, the adversary alone run again) and
`TestSwarmApplyQuestionBeforeRulings`.
