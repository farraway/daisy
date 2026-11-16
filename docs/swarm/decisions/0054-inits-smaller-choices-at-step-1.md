# 0054. `init`'s smaller choices at Step 1

Moved verbatim from `docs/swarm.md` section 14, item 54, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`init`'s smaller choices at Step 1.** A declared repository's
`origin` must also be on the forge's host; the service unit sets
`DAISY_ROOT`, which a user manager does not inherit from the shell;
the install line quotes an escaped unit name; the unit reads the
workspace as `%f`, since `%I` drops a path instance's leading `/`; `.env.sh` lines it
cannot read are named on stderr, as `check-secrets` names them.
