# 0180. Three edges of the reconciler

From Step S5 of the plan implementing the 2026-11-19 swarm roadmap (the
backlog items L325, L326 and L343). Built in `docs/swarm.md` sections 6
(The re-plan), 7.3 (the testing skill without its frontmatter) and 7.4
(the open-slice sentence).

**Why.** Three readings were narrower than what they read. An architect
revising with its issue's pull request open had that head cloned only
when the pull request's repository was still declared, while the
open-slice sentence named `/job/work/<name>` whatever: removed from
`repos`, the sentence pointed at a folder the run lacks, and nothing
said so. The skill's frontmatter was cut on LF line ends with a newline
after the closing `---`, so a file written with CRLF, or ending on that
line, reached the model with its frontmatter. And a re-plan request
counted only when posted at a later time than the architect's newest
word, so one in the same second, which the forge can record, was missed.

**The rule.** The launch warns that the head is not cloned and goes on,
and the open-slice sentence says the head is not in `/job/work`, the
pull request's thread being what the run has of it. The frontmatter is
cut with the line ends read as LF and a closing `---` at the end of the
text accepted. A re-plan request counts when it follows the basis in
the thread's order.

**Alternatives.** Failing the architect's run as *bad slices* when its
pull request's repository is gone would be a new block, which is the
maintainer's to make (the backlog item names it); this adds none.
Comparing times with "not before" would take a request posted in the
basis's second before it in the thread; the thread's order is the one
the forge keeps.
