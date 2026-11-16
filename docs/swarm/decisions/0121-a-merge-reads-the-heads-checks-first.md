# 0121. A merge reads the head's checks first

Moved verbatim from `docs/swarm.md` section 14, item 121, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A merge reads the head's checks first (Step 1 of the plan for
the inbox requests F27–F35, 2026-11-14; request F27).** Gitea
reports a pull request whose required checks are still running as
mergeable and refuses its merge (`405 Not all required status checks
successful`), so with one runner every merge that beat the CI was
T16, blocked for the maintainer though nothing was wrong. The forge
seam gains `Status` and `RequiresChecks` (section 10), and before
`Merge` the merge step and `swarm merge` read the approved head's
checks (section 12, The checks): a failed one is T26, back to the
worker with the contexts named on the pull request and no block; a
pending one waits for the next pass, as an unsettled mergeability
does; only a refusal with no check failed or pending stays T16 (or
T25). **No status at all** was the choice between three readings:
merging as before (a head whose checks have not started is then
blocked, F27's failure in a narrower window), waiting always (a
repository with no checks would never merge), or waiting only where
the base requires checks. The third is taken, its requirement read
from the branch (`enable_status_check`), which a reader of the
repository may read, rather than the branch protection; a forge
that reports none merges as before. Waiting sets no cap: a check
that never settles holds the merge as an unsettled mergeability
does, and a failed head answered without a commit is bounded by the
round limit. **The verb** waits for no pass: a person runs it, so a
pending head is refused with exit 1, nothing changed, and a failed
one is sent back as the step sends it. The judgement is made on the
contexts, not on the combined state, whose precedence among
`pending`, `warning` and `failure` the Swagger does not document. Tests: `TestSwarmMergeStepWaitsForChecks`,
`TestSwarmMergeStepChecksFailed`, `TestSwarmMergeStepChecksPassedRefused`,
`TestSwarmMaintainerMergeChecksFailed`, `TestSwarmMaintainerRefusals`,
`TestFakeChecks`, `TestGiteaStatus` and `TestGiteaRequiresChecks`.
