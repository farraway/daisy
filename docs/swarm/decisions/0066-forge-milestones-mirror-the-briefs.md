# 0066. Forge milestones mirror the briefs

Moved verbatim from `docs/swarm.md` section 14, item 66, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Forge milestones mirror the briefs (Step 9 of the swarm build;
the maintainer, 2026-11-06).** Each `## ` heading of an `active`
brief has a milestone of `forge.repo` titled `<slug>: <milestone>`,
`<slug>` the brief's file name without `.md`, which `apply` ensures
at every pass (section 11, step 8), and each swarm issue whose
`Brief:` line names one is put in it, so a brief's progress shows on
the forge's own milestone pages and `status` reads its counts there.
The plan named two methods, `EnsureMilestone` and `SetMilestone`;
a third, `Milestones`, reads them with the forge's counts, since
`status` runs read-only and cannot ensure, and one list lets a pass
create only what is missing. **The issues from before the
milestones are put in them too:** the step takes every swarm issue,
open or closed, in no milestone, not only a new proposal, so the
first real swarm's issues, two of them closed, count from the first
pass. The alternative, `status` counting an issue in no milestone by
its `Brief:` line beside the forge's counts, was rejected: two
sources for one number, and the forge's page would still disagree.
An issue already in a milestone is left there, so `apply` never
undoes a maintainer who moved one (an issue whose heading was
renamed stays in the old title's milestone). The step runs at every
pass, not only after a batch, so an issue the maintainer files with
a `Brief:` line, and a write a refusal stopped, are taken at the
next; with `--once` it runs after the drain, so a batch's proposals
are in their milestones in the pass that ran it. It is a view, as
the board is: every failure is a warning, never a fault or an exit
code, and its warnings are for open issues only, since a closed
issue's line cannot be acted on. `status` shows the forge's counts,
which take every issue in the milestone (the maintainer's own
included) and none outside it, and names a milestone not yet on the
forge. A `## ` line inside a fenced block is no milestone, nor a
heading without text (section 4), now that a heading becomes a forge
object. Two `active` briefs of one file name would share their
milestones; the second is warned and mirrored to none. GitHub's
milestone pairs are not mapped, its adapter waiting for a run that
needs it (section 10).
