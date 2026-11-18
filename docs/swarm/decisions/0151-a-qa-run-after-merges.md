# 0151. A QA run after merges

From Step 39 of the plan remediating the 2026-11-15 design review (talk A17, 2026-11-17). Its wording was approved by the maintainer on 2026-11-17, the role named `qa`.

**After a set number of merges on the product's base, or a release
tag, a QA run uses the product as its users do and files what fails
as proposals, which the gates then handle as they handle the owner's
(Step 39 of the plan remediating the 2026-11-15 design review,
2026-11-17; talk A17, from the talk's outer loop, which the
2026-11-14 reading recorded as: after each release, fuzz the whole app
as a user).** Until now nothing in the swarm used
the product once a change merged: the reviewers read the diff, the
worker's screenshots show the pages a slice changes (decision
[147](0147-screenshots-reach-the-pull-request-and-the-reviewers.md)), and
a failure between pages, or one a later merge brought back, waited for
the maintainer to meet it.

**A sixth role, declared or absent.** The QA is a persona of its
own (`personas.qa`, its provider, model, effort and co-author as
any role's) with its own prompt, `prompts/<role>.md` as theirs, and a Task text
Daisy fixes (sections 4 and 7.4). It runs only when `swarm.json`
declares its trigger, `qa` (section 3), and the persona and the
trigger come together: either alone is refused. `init` writes neither
and keeps both from an existing file; `prompt build` writes the
QA's default prompt once it is declared. Alternative: the owner's
batch run with a second Task text (no model of its own, and the owner's
persona is a product owner's stance, not a user's).

**The trigger is the maintainer's number, or a tag.** `qa` is
`{"merges": <n>, "tag": <pattern>}`, at least one of them, either
running it: `merges` greater than 0 with no upper bound, and Daisy has
no default for it (the maintainer's rule that no figure is Daisy's
invention); `tag` a pattern as `git tag --list` reads it. A merge is a
commit on the first repository's `base` first-parent line, counted in
its mirror (`git rev-list --count --first-parent`): forge-agnostic, and
the maintainer's own merges count as the swarm's do; a merge commit or
a squash is one, a rebase merge as many as it puts there. A tag counts
when `base` holds it and the last exploration's commit does not (`git
tag --list --merged --no-merged`); git's fetch brings with the branches
each tag that points into them, so the mirror's fetch is unchanged
(adding `+refs/tags/*` changed nothing a test could see). Alternatives:
the merged pull requests counted on the forge (a read per closed issue,
blind to merges made outside the swarm); a field inside
`personas.qa` (the personas would stop sharing one shape); a
string such as `"after": "10 merges"` (a parse for no gain); one form
only (the talk names both, and allowing both costs one `or`).

**Counted from the last completed exploration.** The trigger counts from
the commit the newest `qa` run whose outcome is `posted` explored
(its `start <name> <sha>` line): a run that reported its failures, or
`NO FAILURE:`. With none, it counts from the branch's start, so the
first pass after the declaration runs the QA as soon as `base`
holds the number of commits. Alternatives: the newest launch (a resumed
QA run would then wait for the number of merges again after a run
that explored nothing); a baseline written at the declaration (a state
file for one reading).

**The owner's machinery, with its own hold.** The QA runs as issue
0, one batch run at a time with the owner's (the owner's first in a
pass that has both), at step 7 of the pass, held by `<state>/paused`;
its prompt is the owner's batch prompt without `# Briefs` (and with
the notes read-only); its proposals are created by the owner's
collection, marked `role=qa`, and enter at T1 or T2, the triage
gate (G1) and the job class deciding the rest: at
`swarm:proposal-posted` the owner has nothing to answer on a
QA's proposal, as on its own (rule 3), so a routine one passes
triage by itself (T23). Alternative: the owner answering each (a run
per failure, before the maintainer reads it). Its proposals name no brief, so its class
line comes first; the owner's stays after its `Brief:` line. A run that
fails, writes `BLOCKED:` or proposes nothing writes
`<state>/qa-held`, which `status` names and `swarm resume`
removes, as `owner-held` is for the owner. Alternatives: `owner-held`
itself (a QA run that cannot start the app would stop the owner's
proposals); no hold, the trigger spent at the launch (an image without a
browser would relaunch the QA at every trigger, and the
maintainer would find out from the journal alone).

**The owner's cap, no pacing.** It reports at most
`owner.max_open_proposals` failures, the field's "most issues one batch
run creates", a figure the maintainer already set. It is not paced on
the open proposals: its trigger is the merges, and what it finds goes to
triage like any proposal. Alternatives: a field of its own (a second
figure to set for one meaning); the owner's pacing (a failure would
wait for the triage of unrelated proposals).

**The page map is the maintainer's, in the repository notes.** The
QA drives the product from its routes and controls as its users
see them, kept in the first repository's `repos/<name>.md`, which its
`system.md` carries as the owner's does; Daisy reads no part of it, and
a map absent is the QA's `BLOCKED:`. The first repository's image
runs it and must provide the browser and what the app needs to start,
as the worker's screenshots need. Alternative: a page-map file of its
own in `swarm.json` (a format Daisy would read for nothing it does).

**Bounded.** The trigger watches the first repository, the product,
alone: a library's merges reach the product when it takes them in. The
QA saves no screenshot: Step 38's attachment goes to one pull
request or issue, and matching a run's screenshots to several of its
proposals wants a naming contract of its own (the plan's Deferred).
The role is named `qa`, not the talk's "explorer", so that it is not
taken for the lifecycle's model checker, "the explorer" of decision
[146](0146-the-lifecycle-is-explored.md) and its test.

Tests: `TestSwarmQAAfterMerges`, `TestSwarmQAAfterATag`,
`TestSwarmQAHeld`, `TestSwarmQAUndeclared`,
`TestSwarmQAAfterTheOwner`, `TestSwarmPromptBuildWritesTheQAPrompt`,
`TestSwarmStatusMetFromBatchRunsOnly`, `TestSwarmStatusJSON`,
`TestValidateTheQA`, `TestFormatQA`, `TestProposalClass`,
`TestClassOf`.
