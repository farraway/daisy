# 0093. The collection and the merge step

Moved verbatim from `docs/swarm.md` section 14, item 93, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The collection and the merge step (Step 32 of the swarm build,
2026-11-09).** Three kept lines, by the maintainer's numbers.
**A failed fetch retried for good**: a worker run's fetch from its
clone (7.6, step 1) that failed returned the error, and every pass
retried it with no end, the issue held at its in-progress stage.
Each failure is now recorded (`fetch failed: <error>`), the first
still the pass's error, and the second in a row is a failure of the
work, *fetch failed* (7.7): the issue blocked at the run's home with
the note naming git's error, the run collected. It holds after a
fault too (blocked, not relaunched on a clone that cannot be read),
and a run that failed already names its own failure, then the
fetch's. git's message may quote the clone's own text (a
`packed-refs` line), which no secret check read, so it is withheld
when it holds a secret the member was given, and the state
directory is written `<state>` (members read the thread, section 8).
**No rule bounded a send-back loop** (item 81): a head sent back by
T25, answered without a commit and approved again, was sent back
again, each lap a worker run and a round. The second time the same
head does not merge, told by a direction on it in the pull request's
thread before its last comment (the last being a retry's), the merge
step blocks the issue at `swarm:review-2-approved` with its note, as
the round limit's second time blocks (T11), and `swarm merge`
refuses, nothing changed (a verb's refusal is how the maintainer,
who is there, is told; it sets no flag on its own issue). A
direction counts when it is the swarm's marked note or a `trusted`
login's comment, so an untrusted account's copy blocks nothing.
**Gitea's `false` while its checker recomputes** (item 81, *to
confirm*): before T25 sends a pull request back, the merge step
fetches the repository's mirror and runs `git merge-tree` on it,
head against `base`; a clean merge waits for the next pass instead
of sending back, a conflict sends back with the conflicting files
named in the direction (whose first sentence changes for the merge
step's: `but it does not merge into <base>, …: they conflict in
<files>.`), and a head the mirror cannot read leaves the forge's
report standing, with a warning. The mirror is fetched by the step
because a pass that launches nothing has not fetched it. `swarm
merge` still reads the forge alone: the mirror is `apply`'s, and
the verb fetching it beside a running pass was not wanted for one
needless worker run. The retry's check, the pull request's last
comment being the direction, now matches any direction on the head
by the swarm rather than the exact text, since the files named may
differ between the two readings. No other limit was added.

**Amended by decision [169](0169-integration-control.md)** (R2, specified ahead of its build, Step 12 of the plan implementing the 2026-11-19 swarm roadmap): the merge step merges only a head whose merge check, the repository's check on the commit that merges it into `base`, has passed on `base` as it is now, the first such head of each repository at each pass; a failed one is T32 (`docs/swarm.md` section 12.1).
