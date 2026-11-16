# 0088. The launch and the notes

Moved verbatim from `docs/swarm.md` section 14, item 88, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The launch and the notes (Step 26 of the swarm build,
2026-11-09).** **A resource is checked at each launch** (7.1 step
3, 7.2): it was resolved again at the launch without section 3's
checks, so a link changed between the pass's validation and the
launch was mounted wherever it led; the launch now makes the checks
again on what each entry resolves to and mounts that, and an entry
that fails them is the swarm fault *resource refused* (7.7), so two
in a row pause the swarm with the entry named. **A clone that fails
at a launch** removed the run unrecorded and was the pass's error,
so the pair's second reviewer's issue stayed at review-1-in-progress
with nothing to collect; it is now the swarm fault *clone failed*,
`launch-failed` and collected at once as a refused launch is, the
issue taken home (T22). Both are faults rather than failures of the
work: neither is the member's doing. **The notes' host clone** is
left at the run's base after `notes kept` (the run's pages out of its
tree), and made again when absent at the notes step, which a clone
deleted between a launch and its collection held back until a pass
launched something. **A `notes` that is not a directory** on the
swarm's own repository gives `notes unchanged` for a run whose copy
holds no file, where every owner's and architect's run was `notes
kept` with a warning and a kept copy. **A page whose name holds a
control character** is no note, so the index has one line per note.
