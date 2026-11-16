# 0046. Hardening found reviewing revision 3's first half

Moved verbatim from `docs/swarm.md` section 14, item 46, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Hardening found reviewing revision 3's first half.** The check's
clone fetches the run's ref by name; a slot is re-read in the running
set under its lock, since one snapshot per pass can miss a container
whose `apply` died after the snapshot; an `auth.json` that is not a
regular file is refused, never followed; the secret check lists
object names only (`--no-object-names`). A slot is logged in with
`-c cli_auth_credentials_store=file`, since `codex`'s default store
may keep the login in the host's keyring and leave the slot empty;
the alternative, the default store, is what that risks.
