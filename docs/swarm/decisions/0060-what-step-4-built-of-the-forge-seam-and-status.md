# 0060. What Step 4 built of the forge seam and `status`

Moved verbatim from `docs/swarm.md` section 14, item 60, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**What Step 4 built of the forge seam and `status`.** The pairs
follow Gitea's Swagger where it differs from what revision 5 wrote:
the comment endpoints are read without paging (the Swagger gives them
no `page` or `limit`), the merge body's field is `do`, and an edit
answers `201` (section 10). `Reviews` leaves out an entry in
`PENDING` or `REQUEST_REVIEW`, neither a review anyone submitted, so
a request for a review is never a thread's newest entry. The label id
map is read afresh by each `CreateIssue` and `Relabel` rather than
held for the adapter's life, so a label `EnsureLabels` creates
mid-run is found; two list calls per write is the cost (revised by
item 90: the map is held per pass and dropped by `EnsureLabels`). The retries
move to the step that adds `Ops.After` (Step 5 of the swarm build,
the runner's), item 35 being where that hook is decided. `ReadOnly`
lets a `GET` or `HEAD` and `docker ps` through for every command it
wraps, not for `status` alone (item 31's choice, taken as written):
each is a read. `status` compares logins without regard to case, as
Gitea does, for the author check and `trusted` alike; it reads whose
turn a `planning` issue is from the newest counted entry (section 6)
with the code that the trigger rule extends at Step 5
(`internal/swarm/reconcile`), and the briefs with a reader the batch
prompt shares (`internal/swarm/brief`); its `Briefs:` block names the
`active` briefs, Step 8 adding their milestones.
