# 0116. The architect and the worker read Daisy's praxis and testing skill

Moved verbatim from `docs/swarm.md` section 14, item 116, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The architect and the worker read Daisy's praxis and testing
skill (Step 5 of the plan that clears the Deferred list, 2026-11-11;
the maintainer's decision of 2026-11-10).** Daisy's own step agents
work by `prompts/praxis.md` and the `testing` skill (a claim per
step, red before green, a probe, a hostile review); no member's
`system.md` carried them. It now carries both, from the installed
root, after `# Task` under `# Praxis` and `# Testing` (7.3): the
architect's and the worker's only, the roles that plan and build,
as the maintainer chose; not the owner's, not the reviewers'. They
add about 3,100 words to each such run. Praxis was made to read
for any project first: its rules about Daisy's plan workflow (rule
8's `PLAN.md`, rules 14 and 15's homes, rule 12's and the review
section's `prompts/plan.md`) now say so, "under a Daisy plan in a
home", so one praxis serves Daisy and the swarm. The swarm's plan is
a standard Daisy plan in the issue thread (item 56), not one in a
home (section 1), so those sentences are not the members'.
The files are read at each launch rather than copied into the swarm
repository, so a member reads the praxis of the Daisy that runs it
and the maintainer keeps no copy; a root's file that cannot be read
is a machine's failure, the fault *prompt refused* (7.7), which
pauses the swarm at its second. Tests: `TestRenderPraxisAndTesting`
(the two blocks in the architect's and the worker's `system.md`, the
skill's frontmatter left out, and in no other role's),
`TestSwarmApplyPraxisFromTheRoot` (read in `$DAISY_ROOT`; praxis
removed there, `fault: prompt: system.md:`, nothing launched).
