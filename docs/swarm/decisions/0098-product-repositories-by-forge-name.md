# 0098. Product repositories by forge name

Moved verbatim from `docs/swarm.md` section 14, item 98, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Product repositories by forge name (the maintainer's D2).**
`repos[].path` becomes `repos[].repo`, `<owner>/<name>`, beside the
`name` it repeats (the plan's form, which keeps every reader of
`name` as it is), and `ssh_command`, optional, as `core.sshCommand`
was read from the clone. Every declared repository is `<owner>`'s on
the swarm repository's forge (item 7), so its URL is the swarm
repository's clone's `origin` URL with the last path element
replaced, the rule the wiki's and the notes' URLs already followed,
and no URL field was added. The mirror's config records the URL and
the SSH command and every fetch and push reads them there, so no
host git reads a working tree; the swarm repository is reached with
its clone's own `core.sshCommand`.
