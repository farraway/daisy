# 0117. `swarm prompt build` reseeds the prompts from the defaults

Moved verbatim from `docs/swarm.md` section 14, item 117, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`swarm prompt build` reseeds the prompts from the defaults (Step
7 of the plan that clears the Deferred list, 2026-11-12; the
maintainer's decision of that day).** `init` never overwrites a
prompt (section 2), so a change to a default (item 113's) reached
no running swarm, and a maintainer could not tell an untouched
prompt from an edited one. Layering the prompts at run time was
offered and not chosen. `prompt build`
(section 11) rewrites a prompt that is absent or an earlier default,
leaves the current default as it is, and keeps an edited prompt with
its diff printed and the exit 1, unless `--force`. "Earlier default"
is a table of hashes in the binary rather than a comparison with the
root's git history, so that a root installed without that history,
or a binary run with another root, still knows them; each change to
a default appends to it, which a test enforces. The writes are
committed and pushed before the edited prompts are reported, so one
edited prompt does not hold back the others. The default branch is
read from the clone's origin (`git ls-remote --symref`), not the
forge, so the verb needs no token; the swarm repository's mirror is
fetched after the push, as `init` does, so that `status` shows no
drift. `--dry-run` runs read-only and wins over `--force`, which it
previews. Tests: `TestSwarmPromptBuild` (over a scratch clone and a
bare remote: an earlier default rewritten and an absent prompt
written, committed by the clone's identity and pushed, the mirror
fetched; an edited prompt left byte for byte, its diff printed, the
exit 1 naming it; the dry run writing nothing; `--force` rewriting
it; a last run committing nothing), `TestComparePrompt` and
`TestDefaultHistoryHoldsEachCurrentDefault`.
