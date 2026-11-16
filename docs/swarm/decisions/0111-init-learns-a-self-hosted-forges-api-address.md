# 0111. `init` learns a self-hosted forge's API address

Moved verbatim from `docs/swarm.md` section 14, item 111, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`init` learns a self-hosted forge's API address (Step 6b of
revision 6's plan, as built, 2026-11-10; the maintainer's choice,
from the instance smoke).** `init --defaults` in a folder holding a
product's clone with an SSH origin guessed `https://<host>/api/v1`,
and its first request failed (a TLS name mismatch) on a Gitea whose
API is served at `http://<host>:<port>/api/v1`: an SSH origin
carries neither the web scheme nor the web port. Question 3's
default is now, in order, `init --api <url>` (with or without
`--defaults`), then `$GITEA_API_URL` for kind `gitea` or
`$GITHUB_API_URL` for kind `github` when set and not empty, then
detection as before (section 10). With a `swarm.json` its
`forge.api` stays the default, ahead of the variable; `--api`
overrides it. The flag's value is checked as question 3's answer is
(section 3's `forge.api` row). Tests: the wizard's defaults, by
flag, variable, kind, file and interactively shown; `init --defaults`
over the Fakes opening the forge at the flag's address, else the
variable's, else the guess, and with a file keeping its own; the cli
sending every request of a new swarm's `init` to the address `--api`
names. The instance smoke runs again after it (the maintainer's).
