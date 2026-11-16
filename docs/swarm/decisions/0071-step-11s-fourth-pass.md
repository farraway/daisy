# 0071. Step 11's fourth pass

Moved verbatim from `docs/swarm.md` section 14, item 71, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Step 11's fourth pass.** **Host git fetches only from a clone
that stays inside itself** (7.6 step 1): a member could plant
alternates or link a ref or an objects directory to a host
repository's, and have its commits fetched and pushed; such a clone
is the failure *the clone reaches outside itself*; **the changed
paths are read with `-z`** (7.6 step 3), so a path git would quote
is matched as it is; **a pass whose `forge` (kind, api, repo,
token_env) changed since `apply` started is skipped** (sections 3
and 11), the lock, the adapter and the token being the start's;
**`unblock` to `swarm:reviewed` comments on the pull request**
(section 11), whose thread the worker reads; **the round limit's
second note is told by its start** (section 11), not by its words in
a quoted path.
