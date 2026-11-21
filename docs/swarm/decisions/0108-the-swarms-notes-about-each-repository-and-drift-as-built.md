# 0108. The swarm's notes about each repository, and Drift, as built

Moved verbatim from `docs/swarm.md` section 14, item 108, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The swarm's notes about each repository, and Drift, as built
(Step 5 of revision 6's plan, 2026-11-10).** A launch in a swarm
repository's clone reads `repos/<name>.md` in the swarm repository's
mirror at the pass's commit (`ls-tree`, then `cat-file blob` of a
regular file; a link, a directory, an absent file or a swarm
repository with no commit fetched gives nothing) for each repository
the run works in: the worker's slice's, a reviewer's pull request's,
the owner's (its batch included) and the architect's every declared
one in `repos`' order; each becomes `# Repository notes: <name>`
after `# Task`, its text trimmed of blank lines at its ends and its
headings shifted one level, as the persona's are; a file present but
empty gives its heading alone. **Spec amended (section 4):** the
plan's Step 5 names the seed's facts as the repository's check
*and its protected paths*, where Step 1's section 4 named the
instruction files and the check; the seed carries all three, each
as `swarm.json` declares it, and its text is shown in section 4.
`init` in a clone writes each absent `repos/<name>.md` after the
prompts, never overwriting one (Step 6 adds it to the commit it
pushes, with the other seeded files); its readiness facts need the
declared repositories' mirrors, so **the Order of section 11's init
is amended**: those mirrors are fetched before the files, and the
swarm repository's after the push, so that it holds what `init`
pushed and `status` right after shows no drift. Drift compares
by blob, as item 100 says; the clone's files are those `git
ls-files --cached --others --exclude-standard` names under
`swarm.json` and `prompts/` (a file git ignores never drifts), a
link by its target's text. **Spec amended (7.1, step 1, and section
11, Drift):** `apply` compares at the end of each pass, and a pass
that fetched nothing (one that launched nothing and did not test
the owner's trigger, a paused or held swarm's) fetches the swarm
repository's mirror then, for Drift alone; without it an idle
swarm would never see the push that clears its drift. The memory
of the last drifting set is the `apply` process's. With no commit
in the mirror nothing is compared; a fetch or comparison that
fails is one `⚠ Drift: <error>` line. The old layout has neither
`repos/<name>.md` nor Drift. Tests: the clone fixture's `status`
and `apply` start from a pushed `init` whose mirror holds it, as
Step 6's `init` will leave them.

**Amended by decision
[167](0167-the-system-prompt-stable-blocks-first.md)
(Step E1 of the plan implementing the 2026-11-19 swarm roadmap).**
The repository notes are added to `system.md` before the resources and the run's `# Task`, which now come last.
