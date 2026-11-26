# 0174. A follow-up is titled by its job, and the owner may retitle it by its cause

From Steps FT-1 and FT-2 of the plan implementing the 2026-11-19 swarm roadmap (the swarm meets its exit criteria: merges a day, few interventions; the maintainer's rulings of 2026-11-25). Built in FT-1: `docs/swarm.md` section 7.11 (Follow-ups, the title) and `docs/cli.md` (`swarm merge`'s writes). Amends no decision: it changes the title the filing of Step R6-B gave a follow-up, and nothing the filing reads or writes besides.

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

**Alternatives.** Keeping the pull request's number in the title kept
what the body's second line holds already. Naming the findings' cause
at the filing needs a reading of the findings that Daisy, which files
with no member's run, does not make; that is the owner's (FT-2).

**Left as it was.** A follow-up filed before FT-1 keeps its old title;
no title is cut to a length.

**As built (Step FT-1).** `followUpTitle` in
`internal/swarm/reconcile/followup.go`, called by the filing. Tests:
`TestSwarmFollowUpFiledAtTheMerge` (two findings, at the merge step, at
step 8 and by `swarm merge`) and `TestSwarmFollowUpTitleOfOneFinding`
(two reviews noting one finding, listed once and titled `1 finding`).
