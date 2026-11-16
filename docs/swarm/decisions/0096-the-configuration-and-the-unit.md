# 0096. The configuration and the unit

Moved verbatim from `docs/swarm.md` section 14, item 96, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The configuration and the unit (Step 35 of the swarm build,
2026-11-09).** Three kept lines, by the maintainer's answers.
**The default `protected`** named the forges' workflows, `AGENTS.md`
and `CLAUDE.md`, but not `.claude/` and `.grok/`, from which Claude
Code and grok (with `--trust`) load settings, hooks and MCP servers,
so a worker's change there would run in a later member before any
gate; both join a new entry's default (section 3), and `init` keeps
an existing entry's list. **Question 4's default** was the first
product's origin, so `--defaults` never declared the swarm's own
repository its issues repository (item 92 left it so); it is now the
repository of the first clone directly under the workspace whose
`origin`'s `<repo>` begins `swarm-`, question 1's rule, on the
forge's host (a clone on another host is no repository of this
forge), else the first product's origin as before; the follow-up
then defaults to `yes`, as the clone lies in the workspace. **A
restart lost each running member's exit status, cost and provider
fault** (item 78): systemd's default stop timeout, 90 s, killed
`apply` mid-drain and the next `apply` collected the runs as
`ended`. The printed unit gains `TimeoutStopSec=<timeout>`, the
swarm's `timeout` in systemd's notation, as long as a member may
run, so a stop waits for the drain at the price of a stop lasting
up to a run; a collection that runs past it (a check, a push) is
still cut off and recovered as before. The unit is a template whose
settings are one workspace's, as `RestartSec` already was. No other
limit or default was added.
