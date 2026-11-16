# 0016. The plan's form

Moved verbatim from `docs/swarm.md` section 14, item 16, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The plan's form.** Interfaces (types, signatures, the main call
path), numbered slices with what passes after each, numbered
acceptance claims each with its check, and risks. The worker does not
change Acceptance and reports each item; the reviewers check each
against the diff and the check (*Why Software Factories Fail*: plan to
program design and vertical slices; *What It Actually Takes to Build a
Software Factory*: a validation contract written before the code). The
slice count is read from the plan for the merge step. From revision 5
the plan is a standard Daisy plan (item 56): `## Research` first, the
slices as its `## Steps`, each with its files, signatures, claim and
test, the program design in the steps, and the acceptance as its
`## Test Plan`; revision 1's four headings are gone.
