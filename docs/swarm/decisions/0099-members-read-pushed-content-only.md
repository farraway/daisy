# 0099. Members read pushed content only

Moved verbatim from `docs/swarm.md` section 14, item 99, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Members read pushed content only (the maintainer's D3).**
`resources` are `<path>` in the swarm repository or `<name>:<path>`
in a declared one, and `references` add an outside repository at a
commit or a tag, `{"name", "url", "pin", "paths"}`, fetched once
into its own mirror. Each run's resources are a checkout (`git
archive`) at the pass's commits under `<run>/resources`, one
read-only mount at `/work/.resources`, each at `<repository>/<path>`
so that no two collide (a reference's name may be neither a declared
name nor the swarm repository's); the swarm repository's paths are
read on its default branch, which the fetch reads with `ls-remote
--symref` as the notes' clone takes the forge's default, and a
declared repository's on `base`. `result` records `resource
<repository> <sha>`. The old `is under swarm/` and `.git` checks
fall away (the configuration is no resource directory, and a
checkout carries no `.git`); `.env.sh` and `.daisy` stay refused. A
brief's path is its path in the swarm repository, or `<name>:<path>`,
and the old workspace path `<repo>/<path>` names the same brief
until Step 7 (the maintainer's answer of 2026-11-10: no migrate verb,
the lines rewritten by hand or left). `status` reads the briefs and
Drift from the mirrors as the last pass left them, so `ReadOnly`
lets git's reading commands through, as it lets `docker ps`.
