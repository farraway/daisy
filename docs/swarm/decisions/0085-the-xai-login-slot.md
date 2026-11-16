# 0085. The `xai` login slot

Moved verbatim from `docs/swarm.md` section 14, item 85, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The `xai` login slot (Step 24 of the swarm build, 2026-11-09).**
The key item 82 chose is billed per token; the maintainer's xAI
subscription is not, so `keys.xai` takes `slots: <n>` as
`keys.openai` does, and the key form stays for a swarm without a
login (CI-style use). A provider declares one form or the other: both
is section 3's existing *a credential not of one kind*, no new
message. **The home** is the whole slot, as for `codex`, not
`auth.json` bind-mounted into a per-run home. `grok` writes far more
under its home than the login (section 8 lists the maintainer's
first slot), which argued for the narrow mount; but a single-file
bind mount breaks when the file is replaced by a rename, and how
`grok` rewrites `auth.json` on a refresh was not established (its
guide says only that it picks up external changes), so the
directory mount was kept: it is right either way, the clearing at
each launch already keeps a run from reading another's files, and
the secret check already reads `auth.json` alone, never the
sessions, logs or `memtrace/` the run leaves. The cost, a run's
transcripts left in the slot until its next launch, is `codex`'s
already. **The home variable** moved from the entrypoint to
`docker run --env <home>=/credential`, for both providers, read
from the adapter table's new `Home`: a second slot provider would
otherwise have needed a second unconditional export in the image
and a rebuild before its first run, and the Fake runner now sees
the variable; the entrypoint no longer names `CODEX_HOME` (an older
image's line sets the same value, harmless). **The hint** of *slot
refused* is the adapter table's `Login` with the slot as `Home`,
`GROK_HOME=<slot> grok login --device-auth` for `xai` (`grok login
--device-auth` signs in without a browser; with an empty home `grok
models` prints `You are not authenticated.`). **The wizard** asks no
credential question for any provider (section 11): both forms are
offered as `openai`'s are, by keeping the file's; the default an
added `xai` key takes stays the API key, item 82's, so a swarm
declared before this step reads the same; moving to the slot is the
maintainer's edit. *To confirm at the first `xai` slot run* (the
orchestrator's smoke after the maintainer's login): a review posted
with the marker under the slot, the refresh landing in the slot,
and no `grok` process outliving the container.
