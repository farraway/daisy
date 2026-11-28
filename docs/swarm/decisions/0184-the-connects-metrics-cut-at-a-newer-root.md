# 0184. The connect's metrics cut at a newer root

From Step S11 of the plan implementing the 2026-11-19 swarm roadmap (the
backlog item L476). Built in `docs/swarm.md` section 11 (`daisy swarm
connect`'s Metrics part), `prompts/swarm-connect.md` rule 2 and
`docs/cli.md`'s connect.

**Why.** The connect compares the swarm's runs "since the last model
or prompt change", which it took to be the clone's newest commit to
`swarm.json` or `prompts/`. A Daisy update changes what a member reads
too (the Task texts, praxis, the testing skill), and the clone records
none of it, so runs on two roots were compared as one.

**The rule.** Each run's metrics row names the root it ran on
(`root_sha`). The first of the swarm's runs on the root its newest run
names, when an earlier run names another, is a change as well; the
part is cut at whichever of it and the clone's commit is later, its
line naming that one, and `--json` carries it as `root_change`
(`at`, `root_sha`).

**Alternatives.** Reading the root's own history (a commit that
changes `prompts/` or `skills/`) would miss nothing a row does not
also show, but needs the installed root's git and its age; the rows
already name the root each run used. Cutting at every root change,
whatever it changed, counts an update that touched no prompt as a
prompt change; the rows cannot tell which updates did, and the
simplest reading of the item takes them all.
