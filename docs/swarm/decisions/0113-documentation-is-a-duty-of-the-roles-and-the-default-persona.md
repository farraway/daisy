# 0113. Documentation is a duty of the roles, and the default persona prompts lose their copies of the Task text

Moved verbatim from `docs/swarm.md` section 14, item 113, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Documentation is a duty of the roles, and the default persona
prompts lose their copies of the Task text (Step 1 of the plan that
clears the Deferred list, 2026-11-11; the maintainer's decision of
that day).** No default persona prompt and no Task text named
documentation, so no member kept the documents a change makes
wrong. A sixth role was weighed and refused: it costs a stage and
its labels (section 5), its transitions, a sixth key in `personas`
(section 3), the role rule, the merge gate and one more paid run
per pull request; its commits land after the worker's, so they
restart review or merge unreviewed; and it sees the diff the worker
sees. The duty goes to the roles that already hold the slice
(section 4): the architect names, in each slice's acceptance
claims, the documents the slice changes; the worker updates them,
and any other document its change makes wrong, in the slice; the
adversary's defects include a document the change makes wrong.
Drift across pull requests is a job the owner can propose from a
brief. The same step removes the two sentences of the default
prompts that repeated the Task text, against section 4's rule that a
persona prompt states a stance only: the architect's "The worker
sees only the repository its slice changes…" (its Task text says it
when more than one repository is declared, 7.4 and item 95) and the
worker's "Do not push" sentence (every worker text says `do not
push`, 7.4). The defaults reach a new swarm only: `init` never
overwrites an existing `prompts/<role>.md` (section 2), so a live
swarm's prompts are the maintainer's to edit (until item 117's
`swarm prompt build`, which rewrites one still at an earlier
default). Test:
`TestDefaultPromptsAreSectionFours` (section 4's prompts are
`declare.DefaultPrompts`, verbatim).
