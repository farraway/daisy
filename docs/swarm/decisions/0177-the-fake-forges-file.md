# 0177. The fake forge's file

From Step T8a of the plan implementing the 2026-11-19 swarm roadmap (the maintainer's answer of 2026-11-18: a bash route to a fake forge, so that the acceptance suite covers `swarm init` and `swarm status`). Built in `docs/swarm.md` section 10 (`forge.Fake`; The fake forge's file, from Step T8b) and `docs/cli.md` (Environment read).

**Why.** `forge.Fake` lived in memory, so only a Go test holding it could
reach it: no run of the binary could, and the acceptance suite, which
drives the binary, covered no verb that reads the forge. A forge that a
file holds is one a run of the binary can open, and one a case can write
before the run and read after it.

**The rule.** `Save(path)` writes the Fake's state to a JSON file;
`LoadFake(o, path, account)` reads it back as the same forge. The token's
account and the clock are not the forge's, so the file holds neither:
`LoadFake` is given them. The maps keyed by a struct (the issues and pull
requests by repository and number, the statuses by repository and
commit, the protections by repository and branch) are lists in the file,
sorted, so that a case can write one by hand, and a field the file does
not name is the Fake's zero.

From Step T8b, `DAISY_FORGE=<file>` is the test seam: set, every swarm
verb opens the file's forge in place of the declared one, under the
token it reads, the file's `tokens` mapping each token to its account,
loading the file at each call and saving it after a write. `swarm.json`
never names it, and unset, nothing changes.

**Alternatives.** Go's gob encoding would round-trip the state with
less code, but no case could write or read it with a shell's tools.
Saving the state as it is in memory, maps with struct keys, does not
encode to JSON. Keeping the account in the file was not taken: the
same forge is opened under two tokens (the swarm's and the
maintainer's), and the account is the token's, not the forge's. A
`forge.kind` of `fake` in `swarm.json` would have put the seam in the
declaration a swarm runs on; an environment variable is set by the test
alone. Loading the file once per process and saving it at exit would
need an exit hook the verbs do not have, and two opens of it (the two
tokens) would each hold a copy, the later save losing the earlier's
writes.
