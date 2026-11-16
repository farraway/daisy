# 0106. Product repositories by forge name, as built

Moved verbatim from `docs/swarm.md` section 14, item 106, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Product repositories by forge name, as built (Step 3 of revision
6's plan, 2026-11-10).** A clone's `swarm.json` names each declared
repository by `repo` and an optional `ssh_command`; the old layout's
by `path`, as before, and each layout refuses the other's fields
(`repos[0].path: unknown field` in a clone, `repos[0].repo: unknown
field`, `repos[0].ssh_command: unknown field` and `repos[0].path:
missing` in a workspace), since one parse reads both until Step 7;
a `"repo": ""` is read as absent, `missing`. The clone's `origin`
is validated as `forge.repo` before `repos`. Every mirror, in both
layouts, records `remote.origin.url` and `core.sshCommand` in its
own config (read and written with `--local`, so no global
`core.sshCommand` is taken for the mirror's) before each fetch and
each push, which name the remote `origin`: in a clone from the
clone's `origin` and `ssh_command`, in the old layout from the
workspace clone as each pass read it before. So a mirror an earlier
Daisy made, with no remote in its config, is filled in by its next
fetch or push, whichever comes first, and the live swarms in the old
layout run on unchanged. The notes' clone, which is no mirror, is
reached at the swarm repository's clone's `origin` with its
`core.sshCommand` passed as `-c`, as the old layout's was over the
first declared repository's. The wizard's detection in a clone reads
the folder holding it, where section 3's example puts the products'
clones beside the swarm repository's and where Step 6's `init` will
run; the swarm repository's own clone is left out of question 1's
default whatever its name; question 4's last default,
`<owner>/swarm-<name>` (item 103), is built here. `resources` stay
relative to the clone until Step 4, and the Go test of section 3's
example reads section 3 again, beside the old layout's example.
