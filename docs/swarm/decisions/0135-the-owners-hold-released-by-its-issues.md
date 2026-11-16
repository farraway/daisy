# 0135. The owner's hold released by its issues

Moved verbatim from `docs/swarm.md` section 14, item 135, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The owner's hold released by its issues (Step 17 of the plan
for the inbox requests F27–F35, 2026-11-15; S1).** The chessgrid
session's watch released the owner's batch with `swarm resume`
about nine times in two days, each after the issues the hold's
reason named had closed: the hold Daisy wrote said what it waited
on, and only an agent polling every 30 minutes acted on it. **What
the hold waits on is recorded by Daisy, not read from prose**: the
owner's Task text asks for one form, `BLOCKED: waiting on #<n>,
#<m>: <why>`, and the collection records the named issues still
open in `owner-held`'s second line (`0-owner-<k> waits #<n> …`);
parsing every `#<n>` from the reason was the other reading, but a
reason cites issues in passing (the live hold of 2026-11-15 names
#99 and #100 as what M2 waits for and #101 as the owner of a
contract, leaving open whether it waits on #101; the watch of
2026-11-14 found a hold that "still waited on closed #72"), so a
number in passing would hold or release on an issue the owner did
not wait on. **Only
the open ones are recorded**: a hold whose named issues were all
closed at its collection would be released at the next pass and the
owner run again into the same `BLOCKED:` every pass, so it is the
maintainer's, as is every hold in another form, a failure and
*proposed nothing*. **Closed is the forge's state**, read with
`Issue(m)` at each pass the hold stands (one read per waited issue,
stopping at the first open), not a label or a merged pull request.
**Where**: after the collections and before the pause, so a pass
that collects the last slice of a waited issue and closes it is not
what releases it (the merge step closes it at step 6, the next pass
releases), and a paused swarm's `status` stops naming a hold its
issues have released; the pause itself is the maintainer's, lifted
only by `swarm resume`. The release is a journal line,
`0-release-<k>`, `<k>` the batch run that held it, so `swarm
journal` and the connect's briefing show it as they show a command.
Tests: `TestSwarmApplyBatchReleasedByItsIssues` and
`TestWaitingOn`.
