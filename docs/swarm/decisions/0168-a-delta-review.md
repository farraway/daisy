# 0168. A delta review

From Step E2 of the plan implementing the 2026-11-19 swarm roadmap (the maintainer's direction of 2026-11-21: the roadmap's R6b, delta review, pulled forward before the swarm resumes). Built in that step: `docs/swarm.md` sections 7.4 (the reviewers' row) and 11 (The scorecard's By configuration). Amends decisions [15](0015-severity-per-finding.md), [63](0063-the-adversarys-fail-against-base-check-is-bounded.md) and [163](0163-the-sympathetic-reviewer-after-the-adversary-approves.md).

**Why.** From round 2 a reviewer was given the row's whole text, "The
change is `git diff <base>...HEAD`", and a round sentence asking it
first to say whether its earlier blocking findings were resolved and to
raise new ones only on code changed since: the task named the whole
change, so a reviewer read and judged all of it again each round, the
revision a few lines of it. Since R1 the job's workspace keeps each
round's findings in `slices/<s>/rounds/<r>.md` (decision
[158](0158-the-jobs-workspace-is-the-memory.md)), so a later round can
be given what the previous round found and what changed since.

**The delta review.** A reviewer's round r > 1, on a head with commits
since the head it last reviewed, is given the previous round's findings,
the slice's latest round's file in the job's workspace (or, with none
there, the previous round the prompt's `# Pull request` shows), and the
change since its last head, the round sentence's diff (decision
[90](0090-rounds-briefs-and-the-forge.md): the files the branch changes,
a merge of `<base>` left out). It is asked first to say, for each
blocking finding of that round and of its own last review, whether it
is closed or answered with a reason it accepts, then to review what
changed: the Test Plan's claims the revision bears on, and the worker's
"Not established:" lists against what the revision claims. The whole
change stays in its clone and is named, to read where the revision
needs it, but reviewing it again is not the task. The text replaces the
row's opening, up to its verdict sentence, and the round sentence it
carried before; the verdict sentence, the slice sentence, the
screenshots and the question sentence are as in round 1. Round 1 is
unchanged, and so are the two rounds on the head the reviewer reviewed
before (the maintainer's comment, the worker's `NO CHANGE:`), which
have no change to review.

**On unchanged code** a delta reviewer raises no new blocking finding:
decisions 15 and 63's round r > 1 rule, that new findings are raised
only on code changed since, is kept and said as the text's last line
before the verdict: a finding on code unchanged since its last head is
non-blocking. An earlier blocking finding not closed is not new, and
stands blocking. Letting a delta reviewer raise a blocking finding on
unchanged code the revision's change makes wrong (a caller left behind
by a changed signature) was the alternative: the line is changed code's
consequence, and the reviewer names it on the changed line, which the
rule allows.

**Each reviewer's own head.** Under decision 163's order the
sympathetic reviewer reviews a head only once the adversary approved
it, so its last head can be several heads older than the adversary's
(the adversary having sent the heads between back alone). Each
reviewer's delta starts from its own last review, `<previous head>` as
the round sentence already read it, so the sympathetic reviewer sees
every change since its own review, and its own earlier findings are
named beside the latest round's. A reviewer with no review of its own
on the pull request (the sympathetic reviewer whose first review falls
in round 2 or later) has nothing to start from and reviews the whole
change as in round 1, with no round sentence: before this step it was
given the round sentence with an empty head. Sending each reviewer to
the round file of its own last review was the alternative: the latest
round's findings are what the worker's revision answers, and the
reviewer's own last review is in the prompt's thread.

**Not changed.** The adversary's fail-against-base addition (decision
63) is given in a delta round as in round 1: its rollback judges a test
against `<base>`'s code, which the revision does not change, so a test
the revision neither adds nor changes judges as it did; restricting it
to the tests the revision touches is a later saving, not built here.

**The scorecard.** The delta review changes Daisy's Task text, not a
persona or a prompt part, so `config_hash` (decision
[165](0165-a-configuration-hash.md)) is the same before and after it.
Its before and after are read as two windows of the scorecard, `--until`
and `--since` the time the build that carries it was installed, each
row's `root_sha` naming the root its run was collected under (a run
launched before the install and collected after it reads as after: the
windows are cut at the install's time, not by `root_sha`). A hash of
Daisy's fixed Task text recorded per row, so that the scorecard's By
configuration would separate them itself, was the alternative: one more
column for a change made once.

**As built: the default reviewer personas (Step E2b of the plan
implementing the 2026-11-19 swarm roadmap, 2026-11-21).** The
adversary's and the sympathetic reviewer's default prompts (section 4)
said "Read the whole diff and the code around it" in every round,
against a delta round's Task, which names the change since the
reviewer's last head and says reviewing the whole change again is not
the task. Each now reads "the diff the Task names and the code around
it": the whole diff in round 1, in a later round what the Task names
(the whole change for a reviewer with no review of its own on the pull
request). Wording only, no new rule; their sha256 are appended to the
table of earlier defaults (section 11), so `swarm prompt build`
rewrites a prompt still at the previous default. The sympathetic
reviewer's "try it in your clone under /job/work where that is cheap"
stays: a delta round's Task still gives the reviewer a throwaway clone
to build and test in, and decision 163's order leaves the sympathetic
reviewer's own probes of a new test as they were. Test:
`TestDefaultReviewersReadWhatTheTaskNames`.

**Amended by decision [171](0171-review-by-risk-with-an-end.md)** (R6, specified ahead of its build, Step R6-design of the plan implementing the 2026-11-19 swarm roadmap): the delta text's last sentence becomes the convergence sentence: a blocking finding is closed when its own reproduction passes on the head; from round 2 only an earlier blocking finding not closed, a fault the revision introduced or a claim the head fails blocks, so a claim the head fails blocks on unchanged code too and a new variant of a closed finding on changed code does not; a non-blocking finding the previous round or the reviewer's last review raised is not raised again. A reviewer's first review in a later round is bound by the round. The sympathetic reviewer's "try it in your clone where that is cheap", which this decision's As built note kept, gives way to reading the tests against the plan's probes, should the maintainer take the token rules (`docs/swarm.md` section 7.11).
