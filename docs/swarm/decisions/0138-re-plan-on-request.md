# 0138. Re-plan on request

Moved verbatim from `docs/swarm.md` section 14, item 138, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Re-plan on request (Step 20 of the plan for the inbox requests
F27–F35, 2026-11-15; S4).** A live swarm's watch log records a job
the maintainer wanted re-planned against a new ADR after a resume:
no entry of the maintainer's was newer than the architect's, so
the worker went on, and the re-plan took a `swarm:blocked` set by
hand and an `unblock` whose comment carried the direction. **The
request is a comment**, `Re-plan: <direction>` (`Re-plan.` with
none), posted by the maintainer's verb: a counted entry, so rule 3
runs the architect with no new mechanism, and a form the
maintainer can also type by hand. A no-direction re-plan still
needs a counted entry newer than the architect's word, hence the
fixed `Re-plan.`. Alternative: a label or a file in `<state>`, which
rule 3 does not read and the architect's prompt would not show.
**Between slices it reuses Step 9's revision** (the rulings): the
architect runs on `swarm:plan-1-accepted` in the worker's place
while a request follows the basis, its Task text naming the
request, and its run is collected as the rulings' is (`STANDS:`,
or a revision, T27 with the plan gate on), so merged slices stay
and the worker builds the next slice by the new basis.
Alternative: relabelling to `swarm:plan-0-posted`, as the plan's
text has it: the revision Task text there names no earlier pull
request, the plan gate's note would quote the old plan before the
architect ran, and with the plan gate off the revised plan would
wait there for `accept`, the collection moving nothing from
`swarm:plan-0-posted`. **Where the job moves** (T31): only where
the request alone cannot reach the architect, a job with no plan
(to `swarm:proposal-accepted`) and a review stage whose pull
request is no longer open (to `swarm:plan-1-accepted`, the
revision); at `swarm:proposal-accepted` and `swarm:plan-0-posted`
it is a comment the architect answers. **Nothing in flight is
interrupted:** a worker's run, a run not yet collected (read from
`<state>/runs` on the machine the verb runs on), an open pull
request at a stage whose role is not the architect (closed, its
slice's number stays taken, and the revision's Task text says so,
so its work moves to a later slice), and a question
waiting for its answer are refused with what to do first, rather
than waiting: a deferred request would need a record of its own,
and a comment posted while a member runs is not in its prompt and
is followed by the member's own entry, so it would never be
answered (the plan's Deferred, from Step 14). The verb never
closes a pull request: whether a slice's work is kept (merged) or
dropped (closed) is the maintainer's decision. **From a comment**, `/swarm replan <direction>`, the
command comment is no counted entry, so the verb posts the request
as its own comment, once (the verbs' guard against a second post);
the direction is the rest of the command's line, so it holds no
command line, which the terminal form refuses for the same reason.
Tests: `TestSwarmReplan`, `TestSwarmApplyCommandReplan`,
`TestSwarmReplanStages` and `TestSwarmReplanRefused`.
