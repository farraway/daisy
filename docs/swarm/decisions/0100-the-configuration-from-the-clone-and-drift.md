# 0100. The configuration from the clone, and Drift

Moved verbatim from `docs/swarm.md` section 14, item 100, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The configuration from the clone, and Drift (the maintainer's
D4).** `swarm.json` and `prompts/` stay the working tree's, so an
edit applies at the next pass as before; `status` and `apply` warn
while they differ from the pushed head, compared by blob against the
mirror, `apply` once per change so a loop does not repeat it every
pass.
