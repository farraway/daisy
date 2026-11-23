# 0172. An approval accounts for its slice, and a blocking finding names its kind

From Step R6-E of the plan implementing the 2026-11-19 swarm roadmap (the maintainer's direction of 2026-11-23, and the maintainer's ruling A2 of 2026-11-22, refuse a malformed review and do not retry it). Built in that step: `docs/swarm.md` sections 7.4 (the reviewers' `# Output` and the account reminder), 7.5 (what a review's text must hold), 7.7 (the bad verdict's reasons) and 7.11 (An approval accounts for its slice). Amends decisions [15](0015-severity-per-finding.md) (a review's text is read past its verdict line), [167](0167-the-system-prompt-stable-blocks-first.md) (the reviewers' `# Output`) and [171](0171-review-by-risk-with-an-end.md) (a blocking line that names no kind is refused, where it was counted `unnamed` and let through).

**Why.** The maintainer, on 2026-11-23: "Bare approvals keep merging.
Why?", then that it be adopted and enforced, as a fault of the prompt
and the swarm's configuration: if an agent routinely posts malformed
reviews, it is not being instructed properly. Since R6-A, 16 of 42
grok-4.7 approvals and 12 of 17 gpt-6-luna approvals held no finding
line, some the verdict line alone; one grok adversary run of 35 turns
checked the slice, then reasoned that its review would hold only the
verdict line, though it had noticed an error-swallowing pattern it never
reported, and its summary went into its final message, which Daisy
discards. Daisy read a review's verdict from its first line alone
(`verdictOf`), nothing asked an approval to account for anything, and
with the merge gate off two approvals merge. Ruling A2 (2026-11-22)
adds the review whose `[blocking]` line names a kind of its own making
(two sympathetic runs wrote `[blocking] authored-address hostname
validation <file>:<line>: …`): a malformed review is refused, its
refusal says what is missing, and no review is rerun.

**The rule.** An approval, after `VERDICT: approve`, holds one line
`[checked] claim <n>: <how it was checked; the probe and what it
showed>` for each claim of the plan's Test Plan, and, from round 2, one
line `[closed] <the earlier blocking finding's file:line or kind>: <how
its reproduction now passes>` for each blocking finding of the previous
round, or of the reviewer's last review, that it judges closed. The
reviewers' `# Output` asks for both and its approval template shows
them, one text per role still, the verdict still the first line; every
reviewer Task's last line reminds of the `[checked]` lines, and from
round 2 of the `[closed]` lines. Daisy fails an approval that lacks the
`[checked]` line of a claim, on the bad verdict's path (the run fails,
the issue is blocked, no review is posted and none rerun), its reason
naming the claims. Every review, an approval or a request for changes,
whose `[blocking]` line names no kind the blocking sentence lists
(`claim`, `test`, `defect`, `introduced`, and from round 2 `unclosed`)
fails the same way, its reason quoting the line.

**Which claims are the slice's.** Daisy reads no slice's claims: the
plan's `## Test Plan` numbers its claims across the plan, the blocking
sentence's "a claim of slice `<s>`" is the reviewer's reading, the
builder plan's `## Slice <s>` section and the slice's item under
`## Steps` name the slice's claim in prose, and only the worker's
report, free text, says that a claim is "not in this slice". What Daisy
can read reliably is the Test Plan's numbered claims, the lines
`^[0-9]+\. ` under `## Test Plan` outside a fenced block, as it reads
the slices under `## Steps`. So an approval accounts for every claim of
the Test Plan, and one for a claim outside its slice says so, and how
the reviewer knows: the reviewer Task already asks each reviewer to
check each claim of the plan's Test Plan, and the adversary to leave
one reported not in this slice unchecked, so the line records what the
review did. Enforcing the lines only on a plan of one slice, where every
claim is the slice's, was the alternative: it leaves a bare approval
merging on every slice of a longer plan, the case the direction names.
A format tying each claim to its slice (a `Slice:` line per claim) was
the other, which the step was not to invent. A plan whose Test Plan has
no such line (none, or claims numbered another way) asks no `[checked]`
line.

**`[closed]` is asked for, not enforced.** Which blocking findings a
later round must close is the previous round's, in the job's workspace,
and the reviewer's own last review's, two lists that overlap and whose
findings a `[closed]` line names by file and line or by kind, in the
reviewer's words: Daisy cannot match a line to a finding reliably, nor
tell a finding closed from one answered with a reason the reviewer
accepts, which needs no `[closed]` line. The Task and the `# Output` ask
for the lines; their enforcement is an open question.

**The kind, read once.** The kind is the first word after `[blocking]`,
case and a trailing colon aside, as the scorecard counts the blocking
findings by kind (rule 7 of section 7.11): one reading for both. A
review's round is the `round <r>` line its run's result holds from its
launch, so `unclosed` names no kind of a round-1 review's list; a run
recorded without the line is read as of a later round.

**As built: the text (Step R6-E of the plan implementing the 2026-11-19
swarm roadmap).** `reviewerOutput` asks for the lines after its sentence
on when to request changes, and its approval template shows a
`[checked]` and a `[closed]` line between the verdict and the
non-blocking finding, so the form is seen where the verdict is. The
reminder is two sentences after the verdict's, `checkedReminder` in
every reviewer Task and `closedReminder` where the Task's round is
later than the first, a Task of a later round being one that says
"This is round <r>". The `# Output` sits outside `config_hash` (decision
[165](0165-a-configuration-hash.md)), which is unchanged; `prompt_hash`
changes for the adversary and the sympathetic reviewer, whose
`system.md` holds the new texts. Test:
`TestReviewerOutputAsksForCheckedLines`.

**As built: the check (Step R6-E of the plan implementing the
2026-11-19 swarm roadmap).** The collection of an approval, once its
verdict agrees with its findings, reads the accepted plan as the merge
step reads it (`JobPlan`: the job's `plan.md`), its claims by `Claims`,
and fails the run `bad verdict: no [checked] line for claim <n>, …` when
a claim has no `[checked]` line, before anything is posted. The line is
read as the scorecard reads a blocking line's kind: a line beginning
with the marker, its words after it, case and a trailing colon aside, so
`[checked] Claim 3:` counts, and a `[checked]` inside a sentence does
not. A claim's line is not judged: one whose account is empty, or a
claim checked twice, passes, the reviewer's words being the account.
The reason keeps the bad verdict's words first, so the intervention that
releases it is still `I16`, and the cause stays empty (7.5, Daisy's own
stops). Tests: `TestSwarmApprovalWithoutCheckedLinesFails`,
`TestSwarmApprovalWithCheckedLinesCounts`.
