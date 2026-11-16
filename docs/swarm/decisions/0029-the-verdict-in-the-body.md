# 0029. The verdict in the body

Moved verbatim from `docs/swarm.md` section 14, item 29, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The verdict in the body.** `reconcile` reads a swarm verdict only
from `VERDICT: …`, the first line after the marker. From revision 4
`Review` asks for the formal event first and falls back to
`event: COMMENT` on a `422` (item 47); since the swarm's account opens
the pull request it reviews, the fallback is the expected case.
Alternative: one forge account per persona, a Non-goal. A consequence
for the merge step: a branch protection rule requiring approvals
refuses `Merge` (T16) unless the maintainer has approved the head
(section 12). Step 39 ran its
reviews as comments on Gitea and read every verdict from the body.
