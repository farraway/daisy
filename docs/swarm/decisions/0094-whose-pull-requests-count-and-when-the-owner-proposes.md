# 0094. Whose pull requests count, and when the owner proposes

Moved verbatim from `docs/swarm.md` section 14, item 94, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Whose pull requests count, and when the owner proposes (Step 33 of
the swarm build, 2026-11-09).** Two kept lines, by the maintainer's
answers. **An untrusted account's pull request** on a
`swarm/<n>-<s>` branch with the `Refs` line was one of `Pulls(n)`,
whoever opened it (item 75's reading), so it could become the
issue's current pull request: labelled, read and reviewed as the
swarm's, and its branch, absent from the mirror, failing the
reviewer's clone at every pass while it stayed open. `Pulls` now
takes `openers` and keeps only the pull requests one of them opened
(any case); every caller passes `trusted`'s logins and the swarm's
own, so a maintainer's own pull request still counts and any other
login's never does (section 10). The filter is the adapter's, not
each caller's, so no reader of `Pulls` can see another login's.
**The no-brief batch** proposed from the whole resources tree (five
proposals on the first day, all closed): the owner's batch trigger
is now held while no brief under `resources` is `active` (section
6), writing nothing, and `status`'s `State:` line says ` — owner
batch held: no brief is active`, as it names `owner-held`'s reason.
The batch's Task text for an empty `# Briefs`, now never rendered,
is gone (7.4). No limit was added.
