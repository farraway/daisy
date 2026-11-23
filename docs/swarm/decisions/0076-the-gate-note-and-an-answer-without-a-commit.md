# 0076. The gate note, and an answer without a commit

Moved verbatim from `docs/swarm.md` section 14, item 76, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The gate note, and an answer without a commit (Step 16 of the
swarm build, 2026-11-08).** The flag says that an issue waits; the
gate note says why and what to decide, so the maintainer does not
reconstruct it from the thread. It is posted by the flag step when
it adds the flag (and, from Step 27 of the swarm build, when the
issue is gated again with the flag on: item 89), after the relabel,
so a gating event has at most
one note (the alternative, the note first, would post it again on
each pass whose relabel is refused); a refused post leaves that
event without one, a warning. It is the reconciler's under its own
marker, `role=reconciler kind=gate`, with no run, since it reports on
none, and is never counted (section 6): the swarm's login is in
`trusted`, so the exclusion is by the marker, and it also keeps the
note out of every prompt and out of the notes that blocked the issue
(the round limit's second note stays the newest for `unblock`). Its
first line carries the gate in the text, `**reconciler:** gated at
<stage>: <why>.`, on the marker's second line, as the step's brief
asked. Where the step left a choice the smaller was taken: the
triage gate's "dependencies" are the issues the body names with
where each stands, the owner stating dependencies in prose (section
1's table); "an open issue already covers it" is the open issues
whose `Brief:` line names the same milestone; the plan's Risks are
the first line of each unindented line of the section; long lines
are cut at 120 characters; a block at a gated stage posts no note,
the flag being on already. **An answer without a commit:** a worker
on `review-3-changes-requested` that ends with no commit and
`NO CHANGE: <reason>` as its first line is collected as posted (T12)
rather than failed, as the first day after Step 15 asked (a revision
run that honestly needed none was blocked as *no commits*). For the
reviewers to judge it against the unchanged head, a review counts as
on the head only when it came after the newest answer: the trigger
rule relaunches both, the pair completes on the new reviews, and the
rounds count the head again, so `max_rounds` bounds a loop of answers
as it bounds a loop of pushes. The alternative, an empty commit
pushed by Daisy, would have changed the head with no change and put
a commit of Daisy's own on the swarm's branch.

**As built: a link kept whole by the cut (Step S7a of the plan
implementing the 2026-11-19 swarm roadmap, 2026-11-22).** A long line
was cut at its 119th character and `…`, so a blocked note's first line,
which names a pull request as `[<owner>/<name>#<m>](<url>)`, could end
mid-link, its text or its target broken on the forge. The cut now falls
before a Markdown link that would straddle the 119th character, leaving
the link out; a link that ends within the 119 stays, and a line with no
link is cut as before. Characters are runes, as before. Ending the line
after the link, past 120, was the alternative: it keeps the link, but a
link longer than the limit would make the bound no bound. A link that
opens the line is left out whole, so such a line reads `…` alone; a bare
URL is no Markdown link and is cut as text. Test: `TestCutKeepsALinkWhole`.

**As built: a block at a gated stage gets the blocked gate's note (Step
S7b of the plan implementing the 2026-11-19 swarm roadmap, 2026-11-22).**
"A block at a gated stage posts no note" left an issue flagged at the
triage, plan or merge gate and then blocked with its newest gate note
the stage's, whose `Decide:` line names the gate's verbs and not
`unblock`, and `status`'s Waiting for you with no note of its block (its
`CurrentGateNote` reads only the blocked gate's on a blocked issue). Now
the flag step, finding an issue flagged and blocked whose newest gate
note is not the blocked gate's, posts that note, once: it is a gating
event of its own, as a block lifted at a gated stage already was. The
blocked note mentions no one, as before, so the block is still one
notification. An issue flagged with no gate note on it (the flag set by
hand, or its first note refused) still gets none, blocked or not. The
cost is one `Issue(n)` per pass for each flagged and blocked issue,
which the step read for no such issue before. The alternative, posting
the blocked note when the block is set (in the collection or the merge
step), would have put a second writer of gate notes beside the flag
step. Tests: `TestSwarmBlockedWhileGatedGetsTheBlockedNote`,
`TestSwarmApplyGateNoteOnEachGating` (revised: its blocked issue now
gets the note).
