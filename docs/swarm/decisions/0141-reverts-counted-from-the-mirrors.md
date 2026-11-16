# 0141. Reverts counted from the mirrors

From Step 29 of the plan remediating the 2026-11-15 design review (talk A13, 2026-11-16).

**A revert is the outcome metric, counted at read time from git
(Step 29 of the plan remediating the 2026-11-15 design review,
2026-11-16; talk A13).** The metrics measured effort (tokens, cost,
lines) and nothing about whether a merged change was right; a revert
is the cheapest observable sign that it was not. `swarm status`'s
Throughput gains `reverted <k> of <m> merged pull requests`, and
`daisy metrics` the same per swarm, beside a plan's reverted steps.
**What counts:** a swarm pull request's merge is a merge commit on
`base`'s first-parent line whose second parent is a head a worker run
pushed (its `pushed <sha>` line, in the repository its `branch … in
<name>` line names), since the forge merges at the approved head with a
merge commit; it is reverted when a commit on `base` holds git's own
line `This reverts commit <sha>` naming it. Matching the forge's merge
message (`Merge pull request '…' (#n) from swarm/…`) was the
alternative: it is the forge's wording, not the swarm's record, and
would need a parser per forge; the run's push is the swarm's own. A
squash or rebase by hand, a revert under another message, and a revert
of one commit inside the pull request are not counted, said so in
section 11. **Read from the mirrors, as last fetched, no network:** the
forge's `merge_commit_sha` would name squashes too, but `status` reads
the pull requests of the open and recently closed issues alone and
`daisy metrics` reads no forge; the mirrors hold every merge, and
`apply` fetches them each pass, so the line names the oldest mirror's
`FETCH_HEAD` time. **No column in the metrics CSV:** a revert happens
after the row is written, and the CSV is only appended to, so a column
would hold the count as of the row's writing or need a superseding row
per revert; computed when read, the count is always the mirrors' or
the repository's present one. `daisy metrics`, which reads no clone,
takes `base` from `swarm.json` on the swarm repository's default branch
in its mirror, reading that field alone.
