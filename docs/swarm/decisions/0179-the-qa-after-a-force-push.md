# 0179. The QA after a force-push, and its reproducing test

From Step S4 of the plan implementing the 2026-11-19 swarm roadmap (the
backlog item L384, on decision [151](0151-a-qa-run-after-merges.md)).
Built in `docs/swarm.md` sections 6 (The QA trigger) and 7.4 (the QA's
Task text).

**Why.** The QA trigger counts the first repository's merges since the
commit the last exploration explored. A force-push of `base` past that
commit left `git rev-list <base> ^<explored>` counting against a commit
off the branch, and once the mirror no longer held it, failing the whole
pass, every pass, with nothing for the maintainer to do but wait for a
QA run that could not come. And the QA reported each failure as steps
alone, so the fix's slice had no test of its own to start from.

**The rule.** When the branch no longer holds the explored commit
(`git merge-base --is-ancestor`, or the commit gone from the mirror),
the trigger counts from the branch's start, as before any exploration,
for the merges and the tags alike, and `apply` warns of it once a
process on stderr. The QA's Task text asks, with each failure, for a
browser test that reproduces it from a fresh start, failing now and
passing once it is fixed, in the browser test framework the repository
notes name, or else a Playwright test, in a fenced block for the fix's
slice to add.

**Alternatives.** Failing the pass with a message naming the commit was
not taken: a force-push is the product maintainer's act, and nothing in
the swarm can undo it. Counting from the merge base of the explored
commit and the branch would count the rewritten commits as new when they
are, by content, the old ones; the branch's start is the reading the
trigger already has for a first exploration. Naming no test framework
was not taken: the backlog item carries the maintainer's point of
2026-11-17 naming Playwright.
