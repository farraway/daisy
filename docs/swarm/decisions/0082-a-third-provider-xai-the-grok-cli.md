# 0082. A third provider: `xai`, the `grok` CLI

Moved verbatim from `docs/swarm.md` section 14, item 82, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A third provider: `xai`, the `grok` CLI (Step 22 of the swarm
build, 2026-11-08).** The first two days ran the Anthropic quota
down with the owner, architect, worker and sympathetic all on it,
and one `codex` slot serialised the adversaries; the sympathetic
moves to xAI and `openai` takes a second slot. The adapter is read
from `grok` 1.0.70's `--help`, its `help` and the user guide it
ships, not from a run (none could be made without signing in). **The
argv** (section 9) names both prompt files in `-p`, as `codex`'s
does: `grok`'s system-prompt flags take text and the adapter is
given paths, and `--prompt-file` would send the thread itself as the
instruction, against section 8, rule 8. `--cwd <W>` as `codex`'s
`--cd`; `--trust` because `--add-dir` is refused for a folder `grok`
does not trust; `--no-subagents` so the usage is the run's whole;
`--always-approve`, the container being the sandbox. **Effort** is
`--reasoning-effort <E>`, passed through as `claude`'s is; the
levels a model takes are its own (the guide: `none` to `max`).
**The credential** is `XAI_API_KEY`, the variable the guide gives for
CI, as an `env` key. `grok` also keeps a login in `auth.json` under
`GROK_HOME`, which would make a slot as `codex`'s is (section 8);
the key was taken as the smaller change, and a slot for `xai` is
refused at validation until a run needs it. The key is billed per
token from the xAI console's credits, not a subscription's quota:
the move takes load off the Anthropic quota, and the `cost` line of
an `xai` run is spend. What xAI keeps of the code a run reads
follows the xAI account's own data settings, which are the
maintainer's to set. **The result** is one JSON object, read as
`claude`'s: `usage.input_tokens` is uncached only (the guide), as
`claude`'s is and unlike `codex`'s, which counts its cache in; the
cost only when the server stamped it whole. **The fault words** gain
five of `grok`'s own messages, from its binary's strings, in the one
list every adapter shares. **The image** fetches `grok`'s static
binary for `linux-x86_64` at 1.0.70 from the vendor's release URL in
a build stage, checks it against the SHA-256 the Dockerfile records
(the vendor's installer checks none; the checksum is of the file
the vendor serves, equal to the maintainer's installed release),
and copies it into the base, so no build argument names its version
and the update is a change to the Dockerfile. **The wizard** lists
the providers from the adapter table, gives a role moved to another
provider that provider's default model (`grok-4.5` for `xai`, the
guide's default), and adds a provider's default credential to
`keys` when a persona names a provider no key does: before, a role
moved to a provider without a key failed validation after the
questions. `noreply@x.ai` is the co-author's address by analogy
with the other two; `grok` is not known to write a trailer of its
own. The validation examples that named `xai` as a provider without
an adapter name `gemini`. *To confirm at the first `xai` run*: the
effort level for `grok-4.5`, `--add-dir` beside `--trust`, the
result's fields, a fault's message in the JSON error object, and
the review's verdict line and marker on the forge.
