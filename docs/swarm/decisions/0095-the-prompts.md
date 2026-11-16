# 0095. The prompts

Moved verbatim from `docs/swarm.md` section 14, item 95, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The prompts (Step 34 of the swarm build, 2026-11-09).** Two kept
lines, by the maintainer's answers. **The worker sees one
repository, the architect every declared one**: a plan that said
to copy text from another repository could not be followed by a
worker whose clone is its slice's repository alone. The worker's
mounts are unchanged; the architect's Task text, writing a plan or
revising one, adds a sentence telling it to quote that text in the
plan when more than one repository is declared (7.4), and the
default architect persona said the same (section 4) until item 113
removed it. A swarm's own
`swarm/prompts/<role>.md` for the architect, which `init` keeps when present, is
the maintainer's to change. **The owner's and the architect's
`# Thread` stayed whole** (item 77), so the architect's prompt grew
with every plan revision as the reviewers' grew with the rounds. It
is compacted by item 77's rule (7.3): the entries not the swarm's
whole whatever their age, as direction; of the swarm's, the newest
whole, here each role's newest, since the issue thread holds the
owner's job, the architect's plan and the reconciler's notes side by
side, and the accepted plan with them, the text a revision revises
even under a newer `BLOCKED:`; every older swarm entry its heading
in place, `(superseded by run <k'>; content omitted)`, so the order
of the thread and the maintainer's answers to it stays legible. A
heading rather than a line under its own section, as the issue
thread has no rounds to name. No limit was added.
