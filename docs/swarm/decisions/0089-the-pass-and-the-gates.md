# 0089. The pass and the gates

Moved verbatim from `docs/swarm.md` section 14, item 89, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The pass and the gates (Step 27 of the swarm build,
2026-11-09).** **A `Blocked by #<m>` naming no issue** (section 6,
rule 1): `Issue(m)`'s `404` was the pass's error, so one mistyped
number stopped every launch, and `status` failed; it now holds
nothing, with a warning, through `forge.ErrNotFound` (section 10).
**Two faults in a row** (7.7) are ordered by their end lines' times
to the nanosecond, a container's taken when its `docker run`
returned (section 2), not by run directory within a second, and
pause only when the newer is a fault this process collected since it
last looked: a retried collection of an older fault no longer pauses
again from two others the maintainer resumed past (item 62's
wording). **A gate note per gating event** (section 5): with the
flag left on, a revised plan at `plan-0-posted` (after a comment or
the round limit, T11) and a stage moved under the flag each get a
note, where only the flag's adding did, so the maintainer is not
left with a note quoting the plan the reviews failed; the cost is
one `Issue(n)` per pass for each flagged issue not blocked outside
the two class gates. **The blocked gate's note** quotes the
blocking note's first line cut at 120 characters, and names none
when a maintainer's comment or a gate note of another gate follows
it (its block lifted since); a block lifted and set again by hand
with neither between is still quoted by the older note. **The pull request's label** is copied again at the
pass's end for each issue the pass moved after step (5), one
`Pulls(n)` each. **T16 with no approved head** names no head. **The
loop** exits 130 on a second signal during the pass as during the
drain, and its watcher for that signal ends with it.
