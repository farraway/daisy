# 0120. Declare, forge and runner loose ends

Moved verbatim from `docs/swarm.md` section 14, item 120, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Declare, forge and runner loose ends (Step 20 of the plan that
clears the Deferred list, 2026-11-13).** **The wizard's forge
defaults** (section 10, Detection; section 11): with a `swarm.json`,
questions 3 and 5 took the file's API and token variable whatever
kind question 2 answered, and a kind other than the detected one
put the API on `https://<host>`, dropping an http origin's scheme
and port; both now follow the kind answered, by detection's rules.
**The model check** (section 3): a control or format character (a
zero-width space) passed as part of the one argv word. **Section
3's order**: `keys` is validated before `personas`, as the table
lists them; a persona missing or naming a provider without an
adapter is left to `personas`' own message. **Gitea's labels**
(section 10): an issue carrying the organization's label where the
repository has its own of that name kept it after `Relabel`'s
remove, which deleted the repository's id alone; it now deletes
each id. **Gitea's pages**: a page shorter than 50 was taken as the
last, which an instance with a lower `MAX_RESPONSE_ITEMS` answers
for every page; a list now ends at an empty page or one shorter
than an earlier page, one more request for a list shorter than 50.
**`forge.Fake`'s merge of a closed pull request** answers `404`
(`ErrNotFound`), not `405`, as the triage read Gitea's handler; the
recorded pairs do not cover it. **`init`** (section 11): a kept
prompt that is empty or a link to nothing was named *Kept* and
refused only at `apply`; it is warned of with the way out. A
variable of blanks alone was written to `<state>/env` and shadowed
the environment; it is written commented out, as an unset one is.
The unit's `Environment=` and `ExecStart=` are quoted words with
`\`, `"`, `%` (and in `ExecStart=`, `$`) escaped, so a root or a
binary path with a space, `%` or `$` stays itself, and the install
line says where to save the unit. `status`'s Running line reads the
runner's `Running` rather than a second parse of `docker ps`. Tests:
`TestWizardForgeAPI`, `TestWizardKindChangedFromTheFile`,
`TestParseOrigin` (`forgeAs`), `TestValidateNamesTheField`,
`TestGiteaRelabel`, `TestGiteaListPaging`,
`TestFakePullsReviewsAndMerge`, `TestSwarmInitWarnsOfAnUnusablePrompt`,
`TestSwarmInitEnvTakesBlanksAsUnset`, `TestSwarmUnitQuotesItsValues`
and `TestSwarmInitCreatesTheSwarmRepository`.
