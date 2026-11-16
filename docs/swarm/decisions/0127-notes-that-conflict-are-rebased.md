# 0127. Notes that conflict are rebased

Moved verbatim from `docs/swarm.md` section 14, item 127, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Notes that conflict are rebased (Step 8 of the plan for the
inbox requests F27–F35, 2026-11-14; F32).** A live swarm made nine
`swarm/notes-…` branches with several architects planning at once.
Replayed on a copy of its notes, six were a run's section inserted
where another run had just inserted one (a note's end, mostly), and
three were both runs changing a note's `Checked:` line. The pick
was already a rebase of the run's commit onto the head; git's
three-way merge reports two insertions at one place as a conflict,
so a rebase alone would have changed nothing. **The rule**: a conflict whose base
part is empty (both sides inserted lines at one place) keeps both,
the head's lines first, as the maintainer merged those branches by
hand (the two runs added separate sections); a conflict whose base
part holds a line is the same lines changed on both sides, which
only a reader can settle (the request: where `main`'s version is
newer, it supersedes), and goes to the branch as before. The merge
is read in git's `diff3` form, whose base part says which of the two
it is (`zdiff3`, or the default form, would hide it); a side holding
a line that reads as a marker goes to the branch rather than be
misread. Alternatives: git's `union` merge driver for `notes/`
(keeps both sides of the same lines too, so two `Checked:` lines),
or `-X theirs` (drops the head's section). **The retry**: a push
refused while the head moved since the fetch is put on the new head
and pushed again, as often as the head moves, each retry following
another writer's commit, so there is no limit to choose; one refused
with the head unmoved stays `notes kept: push rejected`. The
alternatives, one retry a pass and the collection left for the next
pass (as the journal keeps its lines), would hold the run's other
steps a pass or need a step pending past `collected`. **No race in
a pass**: the journal's push uses the same clone, at the pass's end,
after every collection, in one process. **`status`** lists the
branches from the swarm repository's mirror (`for-each-ref`, let
through ReadOnly), so one merged and deleted leaves the list at the
next fetch; the runs' `notes branch` lines would list it forever.
Tests: `TestSwarmApplyResearchNotesRebased` (two architects in one
pass adding sections to one note, both on `main`, the second's
commit the run's on the first's; two changing one line, the
second's branch listed by `status` and gone once deleted),
`TestSwarmApplyResearchNotesPushRefused` (a journal commit landing
between the notes' fetch and push, the commit put on it; a hook's
refusal with the head unmoved kept) and `TestKeepBothInserts`.
