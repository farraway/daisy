# 0040. Two credential kinds, and login slots

Moved verbatim from `docs/swarm.md` section 14, item 40, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Two credential kinds, and login slots** (Step 39, checklist items 3
and 8: `claude` ran on `CLAUDE_CODE_OAUTH_TOKEN` as its env line,
`codex` on a copied `auth.json` in a writable mount). Chosen: `keys`
gives each provider `env` or `slots`; a slot is a separate login under
`$DAISY_LOCAL/swarm/credentials/`, shared by every swarm on the
machine, locked for its run, cleared of all but `auth.json` and
mounted itself. Rejected: the entrypoint writing `auth.json` from one
variable into a tmpfs (the refresh dies with the container, and copies
of one login break each other); the host's `~/.codex` (item 41). The
default is what Step 39 ran. A slot's lock ends with its run, so
`Ops.FS().Lock` (item 32) returns a release.
