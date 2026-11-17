# 0149. Daisy reads a text it cut whole

From Step 38c of the plan remediating the 2026-11-15 design review (request F40, 2026-11-17).

**Daisy reads a member's text it cut on the forge whole, from the run's
`out.md`, wherever it reads more of it than its first lines (Step 38c
of the plan remediating the 2026-11-15 design review, 2026-11-17;
request F40).** Item 148 gave the members a cut text whole and left
Daisy's own reads on the forge's text, so a plan whose `## Steps` the
cut reaches was read with fewer slices: none, so one slice, and the
merge step closed the job after its first slice, or, with two
repositories declared, the worker's launch failed as bad slices
(swarm-gridapp#143's plan is 70 333 bytes). **The principle, not the
case:** a text Daisy cut reaches whoever acts on it whole, Daisy
included. One accessor reads a swarm entry's text so (`Job.whole`; the
accepted plan through `Job.PlanText`), with item 148's rules: the run
is the one the entry's marker names on the issue, never one a text
names, its `out.md` a regular file (`Lstat`) of a run not failed for a
secret in its output; any other text is read as posted, the out.md gone
among them, as before. An out.md that is there but cannot be read is an
error (the pass's, `swarm merge`'s, `status`'s), never the text as
posted, since reading the cut text would close the job early with no
trace of why. Its readers: the plan's slices (the worker's
slice and repository, the worker's Task text and `# Plan` heading, the
merge step, `swarm merge`, `status` and notify), the plan gate's note
(the slices, Open Questions and Risks), the merge gate's note's
non-blocking findings, and a review's `[blocking]` lines and their
count in a later prompt (section 7.5). A review shown as its
`[blocking]` lines alone is listed whole, so it needs neither item
148's note nor a copy. **Left as posted:** the first lines a text is
classed by (`BLOCKED:`, `QUESTION:`, `ANSWER:`, `STANDS:`,
`NO CHANGE:`, the verdict), which come before any cut; the class must
be read from the posted text, since an answer's out.md lacks the
`ANSWER:` Daisy adds; and the issue's body (`Brief:`, `Class:` and the
issues it names), which Daisy never cuts. Every reader reaches the file system through `Outputs`, the swarm's
`<state>` and an `ops.FS`, which replaces item 148's list of the issue's
runs: the outputs of the texts an issue's threads hold cut are read once
when its Job is prepared (`Job.readWhole`, the one place a read can
fail), and the copy, the prompt's note and every read of Daisy's take
them from there, so the prompt's rendering reads no file and one rule
decides all three. Alternatives: a fix at each reader (and the next
reader would repeat the gap); passing the whole text along from the
collection, which reads `out.md` already (the forge is the only record
a later pass, `status` or a verb has of a text, so each would need its
own); refusing to act on a cut plan whose out.md is gone (a new stop
the maintainer did not ask for; recorded as a question). Tests:
`TestSwarmCutPlanIsReadWhole`, `TestSwarmCutTextsTheMaintainerIsToldOf`,
`TestSwarmMergeVerbReadsTheCutPlanWhole`,
`TestRenderCutReviewFindingsWhole` and `TestOutputs`.
