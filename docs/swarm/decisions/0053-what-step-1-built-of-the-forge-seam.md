# 0053. What Step 1 built of the forge seam

Moved verbatim from `docs/swarm.md` section 14, item 53, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**What Step 1 built of the forge seam.** `Login` returns the account
(its email feeds `commit_email`) and `EnsureLabels` its created count
(`init` prints it); `init` reads each declared repository through
`Pulls(0)`, so no method is added; `Board`, the GitHub adapter, the
retries and the label id map wait for the steps that use them (section
10, As built); `Issues` filters by label itself, since Gitea answers
every issue to a filter naming no existing label.
