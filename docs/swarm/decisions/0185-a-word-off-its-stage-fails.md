# 0185. A word off its stage fails the run

From Step S12 of the plan implementing the 2026-11-19 swarm roadmap (the
backlog item L358; the maintainer's yes to the triage's question 8, and
of 2026-11-20 to its Open Question 1, that an owner's off-stage
`QUESTION:` is covered too). Built in `docs/swarm.md` sections 5 (T17),
7.5 (the output's first line) and 7.7 (the failures).

**Why.** The collection read any output that was not `BLOCKED:`, a
question it may ask or an answer, as the role's own text. An
architect's `STANDS:`, offered only at `swarm:plan-1-accepted`, written
at `swarm:proposal-accepted` was posted as the plan and relabelled as
one, to the plan gate or, with it off, to the worker; at
`swarm:plan-0-posted` it stood as the newest word with no plan behind
it. An architect's or owner's `QUESTION:` outside a question run was
posted the same way, the owner's as its answer to the proposal.

**The rule.** Such an output is the failure of the work *off its
stage*, T17: `failed: <word> off its stage`, the reconciler's note
naming the word and the stage, `swarm:blocked` set, nothing else
posted. It is read before every other reading of the output but
`BLOCKED:` and the secret check, so neither a slice check nor a plan
write sees it. Its metrics cause is empty, no word of the nine fitting.

**Alternatives.** Posting the text without a relabel leaves the issue
with no note, the role's own entry the newest, so nothing reruns it;
the reading before this step handed a plan-less "plan" to the gate or
the worker. Both were put to the maintainer, who chose the failure,
which is a new block and so was the maintainer's to make. The word in
the reason keeps its colon, as the output writes it, so the note quotes
what the maintainer finds in `out.md`.
