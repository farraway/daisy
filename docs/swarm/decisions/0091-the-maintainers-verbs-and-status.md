# 0091. The maintainer's verbs and `status`

Moved verbatim from `docs/swarm.md` section 14, item 91, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The maintainer's verbs and `status` (Step 29 of the swarm build,
2026-11-09).** **The gated flag** (section 5): a verb cleared it with
every relabel and every unblock, so an issue still gated (unblocked at
a gate that is on, or relabelled while blocked) showed no flag on the
forge until the next pass set it again with a second gate note; a verb
now clears it only where section 5's predicate does not hold for the
labels it leaves, and a block lifted with the flag left on is a gating
event the next pass notes (`regated`), which a block lifted by hand
with the flag on also is now. A flag removal refused after the verb's
relabel is a warning, exit 0, where it was an error a re-run refused;
after a close's terminal it is an error naming the flag, where it
read `not relabelled`. **A comment posted once**: a verb whose write
after its comment was refused is run again, and a comment that is
already the thread's newest by the maintainer with the same text is
not posted again (`unblock`, `close`, `block`, `class`, `merge`'s
direction); a blank comment is none (`unblock` without one is
usage). **The refusals and warnings**: an issue with no stage label
is refused as such, not as no swarm issue; `block` warns, as
`unblock` does, when the maintainer's login is not in `trusted`;
`unblock` is refused at the round limit at `review-0-ready` too,
where T21's write was refused. **`class <n> architecture`** on an
architecture job without the flag sets it. **`.env.sh`** is kept for
the names asked alone, the maintainer's token dropped with the parse
where `apply` and `status` read it. **`status`** writes its report
whole or not at all; an approved issue whose current pull request was
closed unmerged waits under `pull closed`, not `merge`; the class is
named beside the items after `plan` only; a second `active` brief of
one file name is said to be mirrored to no milestone; a reconciler
note on an issue the maintainer moved during the run says it blocked
nothing (7.6) and counts as no block; and `auto` keeps an issue gone
from the forge's window through `<state>/auto` (section 2), the larger
of its record and its notes per issue: the alternative, every closed
issue read at each `status`, costs a read per closed issue for ever,
and a count from the record alone would drop the gates passed before
it was written.

**As built: where a verb's comment landed (Step SF-4 of the plan
implementing the 2026-11-19 swarm roadmap, 2026-11-22).** A verb's line
for its comment named the thread only when it was a pull request's:
`swarm unblock` printed `#<n> commented` for an issue at
`swarm:review-0-ready`, whose comment went to the issue, and `#<n>
commented on pull <name>#<m>` for one at
`swarm:review-3-changes-requested`, so the maintainer could not tell from
the first line where the comment was. Every verb's comment line now names
its thread: `#<n> commented on the issue` or `#<n> commented on pull
<name>#<m>`, and `#<n> comment already posted on the issue` or `… on pull
<name>#<m>` for a re-run. The lines come from the one function every verb
posts through, so `close`, `block`, `class`, `replan` and `merge`'s
direction say it too, and a comment command's answer with them. The
alternative was to name the issue in `unblock`'s line alone, the only
verb whose comment goes to either thread; it would have kept a second
form for the same act. Test: `TestSwarmUnblockSaysWhereItCommented`.
