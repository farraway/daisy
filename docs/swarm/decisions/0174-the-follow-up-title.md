# 0174. A follow-up is titled by its job, and the owner may retitle it by its cause

From Steps FT-1 and FT-2 of the plan implementing the 2026-11-19 swarm roadmap (the swarm meets its exit criteria: merges a day, few interventions; the maintainer's rulings of 2026-11-25). Built in FT-1: `docs/swarm.md` section 7.11 (Follow-ups, the title) and `docs/cli.md` (`swarm merge`'s writes). Built in FT-2: `docs/swarm.md` sections 2 (the batch run's `follow-ups` result line), 7.4, 7.5, 7.6, 7.7, 7.11 (The owner's triage), 10 (`EditTitle`) and 11, and `docs/cli.md` (`apply`'s stdout). Amends no decision: it changes the title the filing of Step R6-B gave a follow-up, and adds one owner's report line beside R6-B's `Triaged:`.

**Why.** A follow-up was titled `Follow-up: <owner>/<name>#<m> (#<n>,
slice <s>)`: the prefix repeated the `swarm:follow-up` label beside it,
and the rest named three numbers, so the maintainer reading the forge's
issue list learnt nothing about a follow-up without opening it, and the
owner reading `## #<f>: <title>` under `# Follow-ups` learnt nothing the
body's second line did not already say.

**The rule (FT-1).** A follow-up is filed titled `<k> findings left by
#<n>: <the title of issue n>`: `<k>` the number of its numbered items
(`1 finding` when it is one), `<n>` the issue its pull request belongs
to. The body, the label and the filing's deduplication, which reads the
body, are unchanged, so a follow-up filed under the old title is still
found and not filed again.

**The rule (FT-2).** For a follow-up it leaves open, the owner may
write a line `Retitled: #<f>: <title>` naming the findings' cause,
among the `Met:` and `Triaged:` lines, before the first `## ` line or
its `BLOCKED:` line. Daisy sets issue f's title to `<title>` and
changes nothing else. A `Retitled:` line naming an issue not under the
run's `# Follow-ups`, or with an empty title, fails the run as a
malformed output fails it (the maintainer's ruling of 2026-11-25, as a
malformed review is refused and not rerun under ruling A2 of
2026-11-22): nothing is created, retitled or closed, and the batch is
held. A failed batch's lines are not acted on. Only the owner
retitles; the forge adapter gains `EditTitle`, the smallest form of a
title's edit.

**Alternatives.** Keeping the pull request's number in the title kept
what the body's second line holds already. Naming the findings' cause
at the filing needs a reading of the findings that Daisy, which files
with no member's run, does not make; that is the owner's (FT-2). For a malformed `Retitled:` line, ignoring
it with a warning, as a `Triaged:` line naming no open follow-up is,
was the other reading; the maintainer ruled it fails the run. Which
issues were under the run's `# Follow-ups` is recorded at the launch,
in a `follow-ups #<f> …` result line beside `propose <p>`, rather than
read back from the run's `prompt.md` or from the forge at the
collection: the record is what the run was shown, and a follow-up filed
or closed since does not change it.

**Left as it was.** A follow-up filed before FT-1 keeps its old title,
unless the owner retitles it; no title is cut to a length; a
`Triaged:` line naming no open follow-up is still ignored with a
warning.

**As built (Step FT-1).** `followUpTitle` in
`internal/swarm/reconcile/followup.go`, called by the filing. Tests:
`TestSwarmFollowUpFiledAtTheMerge` (two findings, at the merge step, at
step 8 and by `swarm merge`) and `TestSwarmFollowUpTitleOfOneFinding`
(two reviews noting one finding, listed once and titled `1 finding`).

**As built (Step FT-2).** `Adapter.EditTitle` in
`internal/swarm/forge` (Gitea's `PATCH /repos/{o}/{r}/issues/{n}` with
`{"title"}`; the Fake's, a `retitled` event); `Retitled`, `badRetitle`,
`(*collection).retitle` and `reportLines`, which `Triaged` now reads
through, in `internal/swarm/reconcile/batch.go`; `followUpsLine` at the
launch; one sentence in the owner's Task after the `Triaged:` one.
Tests: `TestGiteaEditTitle`, `TestFakeEditIssueAndPullURL`,
`TestSwarmRetitledRetitlesTheFollowUp` (a line before the proposals
changes that issue's title alone; one before `BLOCKED:` beside a
`Triaged:` line; a closed follow-up, an issue with no flag, a number
the forge does not hold and an empty title each failing the run with
nothing retitled, created or closed) and
`TestSwarmOwnerBatchReadsFollowUps` (the Task's sentence).
