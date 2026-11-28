# 0178. The maintainer's swarm commits

From Step S2 of the plan implementing the 2026-11-19 swarm roadmap (the
backlog items L328 and L375, the second the maintainer's of 2026-11-18).
Built in `docs/swarm.md` section 11 (`daisy swarm init`, `daisy swarm
prompt build`) and `docs/cli.md` (Commits).

**Why.** `swarm prompt build` named in its commit message the `HEAD` of
`$DAISY_ROOT`, the root the binary runs with, while the defaults it
writes are compiled into the binary: a root pulled since the build, or
a binary built from a tree with changes, gave a message naming text it
did not write. It also checked only the forge fields before writing and
pushing, so a clone whose origin is another repository received the
prompts. And `swarm init` and `prompt build`, the two verbs that commit
in the swarm repository's clone, were the only committing commands that
took no `--co-author`, so an agent running them for the maintainer left
no trailer.

**The rule.** `prompt build`'s message names the binary's commit as
`daisy version` prints it, shortened to seven hex digits, ` (modified)`
kept; its clone's origin is checked, as section 3's validation checks
`init`'s, before any prompt is read. Both verbs are committing commands:
they take `--co-author "Name <email>"`, else `DAISY_CO_AUTHOR`, and end
their one commit with that `Co-Authored-By` trailer, as every other
committing command does. A member's commits are untouched: their trailer
is the launched model's (decision
[176](0176-the-co-author-is-the-models.md)); `swarm.json` holds no
co-author.

**Alternatives.** Keeping the root's `HEAD` and warning when it differs
from the build was not taken: the build is what wrote the text, so it is
what the message should name. Running the whole of `Validate` before a
`prompt build` would refuse a swarm whose prompts are missing or edited,
the cases the verb exists to repair; the origin check alone is the part
that guards the push.
