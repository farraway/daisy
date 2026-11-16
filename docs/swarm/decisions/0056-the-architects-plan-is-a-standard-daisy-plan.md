# 0056. The architect's plan is a standard Daisy plan

Moved verbatim from `docs/swarm.md` section 14, item 56, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The architect's plan is a standard Daisy plan** (revision 5; the
synthesis's G1, the research document as the highest-leverage moment
of the process, and the maintainer's rule that the swarm's plan is a
Daisy plan). The sections of `templates/plan.md` in order, `##
Research` first (the files and seams the job touches, what connects
to what, what the change affects, each line naming a path), read by
the maintainer before the plan at G2; the Steps are the slices, a
numbered list so that 7.5's slice lines parse as before (the
template's checkboxes are a step agent's marks; a slice's mark is its
pull request), each with its files, signatures, claim and the test
that proves it, which the worker follows as a step agent follows a
plan; `## Test Plan` is the Acceptance the worker reports on and the
reviewers check. The adversary checks that each named test exists on
the head and fails on `base` (the synthesis's G2: a test that passes on the
pre-change code proves nothing, the probe of Daisy's testing skill),
a missing or passing one a blocking finding. Alternative for the
research: a separate research run before the plan, one more pass per
job.
