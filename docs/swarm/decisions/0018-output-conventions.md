# 0018. Output conventions

Moved verbatim from `docs/swarm.md` section 14, item 18, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Output conventions:** the owner's batch writes any `Met:` lines
reporting briefs met, then `## <title>` per proposal, its body
opening with a `Brief:` line and a `Class:` line (section 7.5's
table is the full list); any role may write `BLOCKED: <reason>` to set `blocked`
(the plan's "set by any member" given a mechanism); a text over
60 000 bytes is truncated with a pointer to `runs/`.
