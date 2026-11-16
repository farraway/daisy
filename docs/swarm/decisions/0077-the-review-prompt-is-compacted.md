# 0077. The review prompt is compacted

Moved verbatim from `docs/swarm.md` section 14, item 77, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The review prompt is compacted (Step 17 of the swarm build,
2026-11-08).** On the first two days one issue's adversary read
1.1M to 2.8M input tokens per review over seven rounds, each review
re-reading the whole pull-request thread and every earlier review;
the cost arrived before the loss of quality the sources warn of. A
prompt now carries the round's material (7.3), and a review prompt's
length grows by one line a round: on the Fake thread of
`TestRenderReviewCompact`, round 7's review prompt is 4,974 bytes
where it was 36,867, against a thread of 35,895. **The previous
round** is the newest round other than the current one, a round
being section 6's key, a head commit with the worker's answers
without a commit before its reviews, so a `NO CHANGE:` answer is a
round boundary; its reviews are shown, both reviewers' since the
worker answered both, as their headings with their `[blocking]`
lines, the lines the collection already reads as blocking. So the
other reviewer's review of a head before an answer without a
commit, which the commit-based rule of item 76 left out, is now
shown as the previous round's. **The worker's newest report** is the
newest of the pull request's entries that are the swarm's and marked
`role=worker`, its body or a comment; the swarm's comments before it
are the history the lines summarise, and the resolutions as the
worker stated them are in it. Where the step left a choice the
smaller was taken: the description stays in full above the thread,
as the slice's statement, constant in size; the rounds are numbered
over the whole pull request in the order of their first review, a
head one reviewer alone reviewed included, rather than since the
accepted plan as the Task text counts (which would need a second
sequence or a mark for the revision; the two differ after T11, and
once a head was reviewed by one reviewer alone);
a line's resolution is what followed the round, `revised in <sha>`
or `answered without a commit`, Daisy's fact rather than the
worker's claim; and "the issue thread's older entries", which a
reviewer's prompt does not hold (the table of 7.3), is read as the
pull-request thread's older swarm entries, the owner's and the
architect's `# Thread` staying whole (until item 95 compacted it). The maintainer's entries are
shown in full whatever their age: they are direction. The architect
keeps every round's blocking findings, without the bodies, as it
reworks the plan from them, and the worker the latest round in full.
