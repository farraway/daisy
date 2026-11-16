# 0131. The comment commands

Moved verbatim from `docs/swarm.md` section 14, item 131, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The comment commands (Step 13 of the plan for the inbox requests
F27–F35, 2026-11-15; the maintainer's feature of 2026-11-12, control
from the forge, its part (1)).** The maintainer steers a running
swarm from the forge with the verbs of section 11, one command set
for the terminal and the thread (section 6, The comment commands).
**The verbs are reused, not copied**: `apply` holds a `Command`
callback the `commands` package gives it, which runs the verb as
`daisy swarm <verb> <n> <words>` runs it, so every precondition,
refusal and write of section 11 is the command's, and the
reconciler's package never holds the maintainer's token: that
token is read at the command, by the verbs' own start, and given to
the adapter the verb opens. Alternative: the verbs moved into
`reconcile` beside a second adapter (two places for one verb, or a
move of every verb's code for one caller). **Authorization** is the
trust rule's: the comment's author in `trusted` other than the
swarm's; any other login's command is ignored, unanswered, so no
login outside `trusted` can make the swarm post; the swarm's own,
`trusted`'s last, never runs one, so a member's output that carries
a command line runs nothing (prompt injection stays text). The
token acts whoever the trusted author is (section 8, rule 9).
**Edits**: a command runs only from a comment never edited
(`Updated` not after `Created`): on Gitea a login that may write the
repository's issues may edit another's comment, the swarm's account
among them, so the author vouches only for the body as posted; an
edited one is answered *not run*. Alternative: the body as of first
sight, marked by id (an edit made before the first pass that reads
it would run as the author's). Gitea's insert sets `created_at` and
`updated_at` to the same second; a straddled second would read as
an edit, answered *not run*, visible, and posted again. **Once**:
by the comment's forge id (`Comment.ID`, which section 10's
`Comment` gains with `Updated`), recorded in
`<state>/commands/<id>` before the verb runs, and named by the
answer's marker. The record is the guard, not the answer: review
pass 1 of the step showed an answer edited by another writer of the
repository running its command again under the maintainer's token,
so the answer, which the forge lets such a writer edit or delete, is
posted again from the record and never decides a run. A refusal is
not retried (Alternative: retrying it each pass, which reruns a
stale command when the state later allows it); a record written
before the verb makes the run at most once, an `apply` stopped
during it answering `Not known: …` rather than running it again.
**Which
verbs**: those section 11 lists, each on the issue it is posted on
or whose current pull request it is on, none taking `<n>`, so a
command never reaches another issue (`block --by <m>` names its
blocker, and flags it as at the terminal); `pause` and `resume`
hold that issue alone: a comment cannot pause or resume the whole
swarm, whose `<state>/paused` is the host's, and resuming a swarm
that two faults paused is the maintainer's at the host; `prompt
build` writes the clone and is none. One command per comment: two
are answered *not run*, so an order between them is never Daisy's
guess. **Where in the pass**: step 5, per issue read at its stage
and before anything else is decided on it, paused or not (`unblock`
and `resume` must run on a held issue, `merge` on a stage with no
role); the issue is read again once one ran, so the role rule reads
what the command wrote in the same pass (Alternative: a step of its
own before step 5, which would read every open issue a second time
each pass). Its cost: one `PullComments(m)` more per issue with an
open pull request each pass. **The trigger rule (R4)**: neither a
command comment nor its answer is counted, nor a `Blocked by #<m>`
nor the maintainer's word lifting a block, so neither wakes a
member: a command is the maintainer's word to Daisy, as a class
statement is; the comments a verb posts (`unblock`'s, `close`'s)
are counted as at the terminal. Plain maintainer comments are the
plan's Step 14. **The answer** quotes the command lines and gives
the verb's stdout, or its error after it, in a code block, the
host's paths written `<state>`, `<swarm>`, `$DAISY_LOCAL` and `~`
(an error names `<state>/env`); the verb's stderr is `apply`'s. It
is a reconciler note of its own kind, never counted and never in a
prompt; a command comment is shown as any trusted comment is (its
other lines may be direction). **The journal** takes a line per
command, `<n>-command-<id>`, beside the runs'. **No new label.**
Tests: `TestSwarmApplyCommand`, `TestSwarmApplyCommandRefused`,
`TestSwarmApplyCommandNotRun`, `TestSwarmApplyCommandOfTheSwarm`,
`TestSwarmApplyCommandOnThePullAndPause`, `TestCommandLines`,
`TestNewestCounted`, `TestSwarmsReadsTheMarkerAndTheAuthor` and
`TestBlockedBy`.
