# 0073. The maintainer's labels

Moved verbatim from `docs/swarm.md` section 14, item 73, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The maintainer's labels (Step 14 of the swarm build,
2026-11-07).** Three labels beside the eight stages, which stay the
state machine, one per open issue (section 5). **`swarm:gated`, a
flag, never a stage**: on an open issue exactly when its one stage is
a gate that is on, or `blocked`, set and cleared by `apply`'s step
(8) after its collections and its merge step, cleared by the verbs
with their relabel; `status`'s "Waiting for you" lists by the same
predicate (one function, `reconcile.Gated`), so the forge shows what
the terminal says. The predicate is the stage and the gate alone: a
`planning` issue the maintainer has answered stays listed under
`plan`, and flagged, until it leaves the stage, where `status` listed
one only while the architect's entry was the thread's newest (a flag
that came and went within a stage would say whose turn it is in the
thread, which the role rule already decides). **`swarm:done` and
`swarm:rejected` are terminal**, set after the close in the stage's
place, the flag removed: `done` by T15 and the last slice's `merge`,
`rejected` by `close`; closing first means a refused relabel leaves
a closed issue with its stage, informational, where relabelling
first would leave an open issue with no stage that no rule and no
verb reaches. **Every read of an issue's one stage ignores the three**
(`forge.IsStage`); the lists of closed swarm issues read the
terminals beside the stages, or a job closed as `done` would drop out
of the batch prompt's `# Recently closed` and the owner could propose
it again; the batch prompt's line names a closed issue's terminal,
the owner's evidence of a job done or turned down. **`Relabel` with
`add` `""` only removes**, the adapter's smaller change beside a new
method: the flag is set with `remove` `""` and cleared with `add`
`""`. The swarm's account may not create organization labels on
Gitea (item 50), so the maintainer creates the three once, or
`apply` refuses at its start. Alternative considered: the flag as a
ninth stage (`gated` in place of the gate's stage), which would have
lost the stage the role rule reads.
