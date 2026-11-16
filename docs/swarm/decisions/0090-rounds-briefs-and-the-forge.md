# 0090. Rounds, briefs and the forge

Moved verbatim from `docs/swarm.md` section 14, item 90, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Rounds, briefs and the forge (Step 28 of the swarm build,
2026-11-09).** **The round's diff** (7.4): a round r > 1 reviewer
was given `git diff <previous head>..HEAD`, which after the worker
merged `base` (its first sentence, or T25) holds every other
slice's change; the diff is now limited to the files the branch
changes (`$(git diff --name-only <base>...HEAD)`) and the text says
a merge's lines are not the pull request's. A range-diff was the
alternative, harder for a reviewer to read; a file both the branch
and `base` change still shows `base`'s lines in it, which the
sentence covers. **The round lines** (7.3) name a round by the head
it reviewed (`- round on <sha>:`), no longer by a number the Task
text's `This is round <r>` contradicted after T11 or a one-reviewer
head. **The worker's `review-3-changes-requested` text** names the
merge step's direction beside the reviews and the maintainer's
comments; sections 5 and 6 name T25 where they listed only a review.
**The answers** (section 6) are read from the pull request's
comments by the trigger rule too, which read the whole thread, the
description included; a revision's `NO CHANGE:` comment after a
push still reads as one, which changes nothing (7.5 says so).
**One fence reading** (`internal/swarm/fence`) for the plan's
sections and slices, `Shift`, `emptyField` and the briefs' and
notes' headers: a fence closes only on a run of its character at
least as long with nothing after it, as CommonMark has it, where
both copies closed on a fence line with an info string. **The label
id map** (section 10) is held per pass, not read by every write,
revising item 60: a label `EnsureLabels` makes still is found, as
it drops the map. **The milestone step** (section 11) takes the open
issues from the flag step's list and reads the closed ones only
when it created a milestone or at the process's first reading: a
closed issue it could still place is one closed without a milestone
since that reading, its brief not active then or a `SetMilestone`
refused, and waits for the next milestone created or the next
start of `apply`. **The Fake** refuses in Gitea's order, whose
handler checks mergeability (`405`, a refused merge among them)
before the head (`409`): section 10 listed the moved head first,
and the Fake checked a refused merge after it. `Relabel` with
`remove` equal to `add` removes the label, as Gitea does.
