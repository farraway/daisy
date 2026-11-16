# 0081. Conflicts handled by the swarm

Moved verbatim from `docs/swarm.md` section 14, item 81, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Conflicts handled by the swarm (Step 21 of the swarm build,
2026-11-08).** The first conflict came at the merge gate: an approved
pull request had forked before two other slices merged, Gitea
refused the merge (`405 Please try again later`, `mergeable: false`),
and the maintainer sent it back by hand with a comment and a relabel
to `swarm:review-3-changes-requested`. With the gate on nothing had
merged `base` into a branch before the merge. **Every worker run
merges `base` first**, gate on or off, and resolves the conflicts it
can; one it cannot is `BLOCKED: <base> conflicts in <files>`, the
existing `BLOCKED:` path (T17), since an unresolved conflict needs a
human. **The merge reads the forge's mergeability** (`Pull.Mergeable`,
read on Gitea from the `Pulls` list the merge already makes, no new
request; GitHub's list carries no `mergeable`, so its `Pulls` reads
each open swarm pull request again, section 10)
and, when the forge refuses a pull request it reports not mergeable,
sends it back (T25): the direction on the pull request, where the
worker reads, and the relabel, by the merge step and by `swarm merge`
alike, which prints `#<n> not mergeable: back to the worker` and
exits 0 instead of refusing. A refusal of a pull request reported
mergeable stays T16 for the step and a refusal for the verb.
**Unknown is not a conflict**: GitHub reports `null` while it
computes; Gitea reports a boolean, `false` while its checker
recomputes after a push to `base` as well as on a conflict (its
source, *to confirm* on the instance), so the one case Daisy can see,
its own merge step having merged into the same repository earlier in
the pass, is read as unsettled too, and the next pass tries again; a
`swarm merge` run seconds after another merge into the same base may
still send a clean pull request back, at the cost of one worker run
and one round. Where the step left a choice the smaller was taken:
the merge is attempted first and the mergeability read only on a
refusal (a forge that merges while it computes is not held up); the
direction names the branch and the base, neither forge naming the
files; the merge step's direction is the reconciler's note on the
pull request, posted once across a retry, and adds nothing to the
issue's thread; the Fake's `Unmergeable(pr)` became `SetMergeable(pr,
m)`, with `RefuseMerge(pr)` for a refusal of a mergeable pull
request. A pull request sent back and approved again without the
worker having merged `base` goes round again: no rule stops that
loop, the review rounds counting only requests for changes; a stop
is the maintainer's to decide (footprints in the plan and a
per-repository cap are deferred: one conflict in fourteen merges
did not earn them).
