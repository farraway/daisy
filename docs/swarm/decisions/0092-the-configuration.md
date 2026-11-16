# 0092. The configuration

Moved verbatim from `docs/swarm.md` section 14, item 92, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The configuration (Step 30 of the swarm build, 2026-11-09).**
**`wiki` needs a declared issues repository** (section 3): `wiki:
true` with a `forge.repo` none of `repos` was accepted, though Step
23's design refused the wiki on the swarm's own repository; the
configuration cannot tell the swarm's own from another undeclared
repository, so validation now refuses `wiki` for any issues
repository none of `repos`, and the wizard's question 35 refuses the
wiki's `yes` there with the same message. **Question 1's default**
(section 11) listed every repository directly under the workspace,
the swarm's own repository's clone with them, so `init --defaults`
declared it a product repository; leaving out the clone whose origin
is `forge.repo` would need `forge.repo` before question 1, whose
answer it defaults from, so the default leaves out a clone whose
`origin`'s repository name begins `swarm-`, section 3's convention
(`<owner>/swarm-<name>`); a repository so named that is a product is
typed into the answer. **Question 4's follow-up** defaulted to `yes`
on the file's `notes` whatever issues repository was just answered;
it now does so only when the file's `forge.repo` is that one.
