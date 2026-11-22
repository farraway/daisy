# 0144. A recurring finding asks for a check

From Step 34 of the plan remediating the 2026-11-15 design review (talk A4, 2026-11-16; wording approved by the maintainer 2026-11-17).

**The default reviewers name a check for a finding new code can
repeat, and the owner may propose the job that adds it (Step 34 of
the plan remediating the 2026-11-15 design review, 2026-11-16; talk
A4).** A correction the swarm's reviews made stayed a correction:
nothing asked whether a check could stop the next instance, and the
owner proposed only milestone work, so removing a pattern the
reviewers kept flagging fell to the maintainer. Root-side, testing
rule 8 and praxis rule 12 already close a class that can recur in new
code under a check over the whole tree; the reviewers do not receive
praxis, so their defaults (section 4) take the principle in their own
words, each in an existing bullet. **Both reviewers**, in the bullet
that makes every finding its own marked line: "When a finding is one
instance of a kind new code can repeat, and a check that scans the
whole tree (a test, a lint) can recognise every instance, name that
check, at the finding's file and line, in a non-blocking finding of
its own." The check is offered without holding the merge, a finding
of no such kind asks for none, and the line keeps the file and line
every finding carries. **Both reviewers** count as findings "code
that works around a clash with the design instead of resolving the
clash, naming whether the plan or the change makes it" (the plan's
clash is the architect's to resolve, not the worker's) and "a comment
or description that argues a shortcut is acceptable instead of fixing
it or listing it as not done" (so the worker's `Not established:` and
a claim reported not in this slice, which the Task texts ask for, are
not findings): the adversary's list of defects widened, the
sympathetic's judgment of fit, with the clash's naming in brackets
there. Their severity stays the reviewers' existing rule. **The
owner's** "Prefer the smallest job that moves the next milestone"
widens: "A job that removes the cause of a finding the reviews keep
repeating, as the maintainer's comments, the resources or the
repository notes record it, moves that milestone too: the job adds
the check that catches the finding or removes the pattern, and names
where the finding is recorded." The owner reads no review (its batch
prompt, 7.3, holds the briefs, the issues' titles and each closed
issue's newest comment not the swarm's), so the job rests on what it
does read, and its `Brief:` line still names the milestone the
batch's Task text (7.4) asks every proposal to move. **The persona
names no marker:** the Task text's `[non-blocking]` stays the Task
text's (section 4: the persona states a stance, item 113's rule: no
Task text repeated). `prompt build` moves a swarm whose prompts are an
earlier default to these (the three roles' new hashes appended to
`defaultHistory`, item 117). Alternatives: a new bullet per role (the
existing bullets carry the principle, so the prompts do not grow a
rule); the reviewers' check line `[blocking]`, which would hold a
correct change for a check the job did not ask for; the literal
`[non-blocking]` marker in the persona; a check line for any finding a
check could catch, which fires on nearly every defect; an owner bullet
of its own for gardening, beside milestone work, which the batch's
Task text has no `Brief:` line for; review findings added to the
owner's batch prompt, a change to 7.3 and its code left for later.
Tests: `TestDefaultPromptsAreSectionFours` and
`TestDefaultHistoryHoldsEachCurrentDefault`.

**Amended by decision [171](0171-review-by-risk-with-an-end.md)** (R6, specified ahead of its build, Step R6-design of the plan implementing the 2026-11-19 swarm roadmap): the non-blocking check finding is no longer only offered: with every other non-blocking finding of a merged pull request's reviews, Daisy files it in a follow-up issue (`swarm:follow-up`), which the owner's batch reads under `# Follow-ups` and proposes from, a job naming its items on a `Follow-up:` line (`docs/swarm.md` section 7.11).
