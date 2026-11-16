# 0080. Gates by job class

Moved verbatim from `docs/swarm.md` section 14, item 80, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Gates by job class (Step 20 of the swarm build, 2026-11-08).** On
the first two days the swarm launched nothing for 27.7 of 40.4
hours, and every gap but the first was a gate waiting for the
maintainer; the maintainer's rule since is that gating is kept for
architectural decisions. The owner classes each proposal *routine*
or *architecture* (section 5, The job class); a routine job passes
G1 and G2 by itself and stops at G3; an architecture job keeps all
three, and `gates.triage` and `gates.plan` keep their meaning for
it. **A routine job needs a statement.** The one flag,
`swarm:class-architecture`, cannot tell a routine job from one
nobody classed: an issue without it is a routine proposal, or one
filed by the maintainer, or one open before this step, and the last
two must stay architecture, the safe side. So routine is read from a
positive statement, the owner's `Class:` line or a maintainer's
`Class: <class>` comment, the newest deciding, and the flag makes a
job architecture over any statement; the issues open at the install
carry no statement and keep every gate. The alternative, a second
label `swarm:class-routine`, was a second state to keep consistent
with the first. **Daisy passes the gate at step 8**, where the gate
is decided, rather than at the collection that reaches the stage
(as it does with a gate off): a relabel at the collection would miss
a job the maintainer makes routine later, a maintainer's issue, and
the plan a revision after T11 reposts. It passes only when the
stage's role has nothing to answer, so a maintainer's comment, and
after T11 the reviews, are answered first (otherwise T11 on a
routine job would hand the old plan straight back to the worker),
and not while the swarm is paused or the issue held, as the merge
step does not. **A class statement is no counted entry**, and the
blocker rule passes over it: counted, the verb's comment would wake
the owner, or have the architect revise its plan in answer to
`Class: routine`, and would lift a `Blocked by #<m>`.
**`owner.max_open_proposals` stays a brake**: with the triage gate
on, the batch ran once no issue was `proposal-posted`, and a routine
proposal leaves that stage in the pass that creates it, so the batch
would run every pass; the routine jobs Daisy triaged now count while
they wait at `proposal-accepted`, as every proposal counts with the
gate off, and an architecture job the maintainer triaged counts no
more than before. Where the step left a choice the smaller was
taken: the class line is read only right after the `Brief:` line,
so a proposal without a brief is architecture; the batch's Task text
gives the classes' definitions and the protected paths, which the
owner did not see before; the auto note is one line, `**reconciler:**
auto at <stage>: a routine job passes the <gate> gate by itself, to
<next stage>.`, under `role=reconciler kind=auto`, read as the gate
note is (no run, not counted, in no prompt); every gate note opens
its bullets with the class, and at the triage and plan gates offers
`swarm class <n> routine`; `status` puts the class beside every
waiting issue, `#<n> (<class>)`, and counts the auto notes on the
issues it reads rather than keeping a record under `<state>`;
`class` leaves an issue of the class as it is, refuses `routine` by
a login not in `trusted` (no rule would read its statement), and
works at any stage; a close clears the class flag with the others.
The sixteenth label is created by an owner of the organization once
(the swarm's account cannot, section 10): until it exists, `apply`
stops at its start with `EnsureLabels`' error naming it.
