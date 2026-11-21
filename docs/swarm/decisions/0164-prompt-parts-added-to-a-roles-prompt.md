# 0164. Prompt parts, added to a role's prompt

From Step R3-13 of the plan implementing the 2026-11-19 swarm roadmap (the maintainer's direction of 2026-11-21). Built: `docs/swarm.md` sections 3 (`prompt_parts`, its errors, its place in the file), 4 (Prompt parts), 7.3 (`# Prompt part: <path>`), 7.6 (`prompt_hash`) and 11 (`swarm prompt build`); the swarm skill's Reseeding the persona prompts.

**Why.** A swarm adds to a role's stance for its own products (CSS
rules, the browser a reviewer drives, a product's practice lines) by
editing `prompts/<role>.md`. An edited prompt is one `swarm prompt
build` keeps (decision
[117](0117-swarm-prompt-build-reseeds-the-prompts-from-the-defaults.md)),
so a newer Daisy's default reaches it only by a hand merge of each
diff, and the swarm's additions and Daisy's text are one file no one
can tell apart. The maintainer's direction: add to a role's prompt
rather than edit it, a part appended to the roles `swarm.json` names,
as a Daisy workspace's `AGENTS.md` combines prompts.

**The declaration.** `swarm.json` gains `prompt_parts`, a list of
`{"path": <path>, "roles": [<role>, …]}`, by convention
`prompts/parts/<name>.md`. Each part is added to the `system.md` of
every run of each role it names, after `prompts/<role>.md`, before
`# Task`, under its own heading `# Prompt part: <path>`, its headings
shifted as the persona's are, the parts in the list's order. It is
read where `prompts/<role>.md` is, in the clone's working tree at each
launch. Validation refuses a path that is empty, absolute or through
`..` (outside the clone), a file missing or empty there (not read at
`init`'s first validation, which skips the prompts), an empty `roles`,
and a role that is none of the six; each error names its field, as the
others do. `init` keeps the list from an existing file and writes none
of its own; `Format` writes it last, an entry a line.

**The hash.** A run's `prompt_hash` is the sha256 of its whole
`system.md`, which holds the parts, so a part's text changed changes
the hash. As recorded today that hash also differs from run to run
(the file's first line names the run and its time), which Step R3-12's
scorecard by configuration reads as one configuration a run; a hash
over the configuration's text alone is an open question of Step R3-12.

**Amended by decision [165](0165-a-configuration-hash.md) (Step
R3-12b).** That hash is recorded: `config_hash`, the sha256 of the
persona and its parts as `system.md` holds them, which the scorecard
groups by.

**`swarm prompt build`** is unchanged: it compares and writes
`prompts/<role>.md` alone, never a part. With the swarm's additions in
parts, a role's prompt stays Daisy's default and the verb replaces it
whole with a newer one; the swarm skill drafts the move of an edited
prompt's additions into a part, on the maintainer's yes.

**Alternatives.** Front matter in each part file naming its roles (the
list in `swarm.json` is one validated place, read with the rest of the
declaration, and a part's text stays the model's alone); a part
appended at the end of `system.md`, after `# Resources` (the persona
and its parts are one stance, read before the Task); reading the parts
at the pass's commit, as `repos/<name>.md` is (the persona prompt is
read in the working tree, and a part is its continuation).

Tests: `TestSwarmPromptParts`, `TestRenderPromptParts`,
`TestValidatePromptParts`, `TestFormatPromptParts`.

**Amended by decision
[167](0167-the-system-prompt-stable-blocks-first.md)
(Step E1 of the plan implementing the 2026-11-19 swarm roadmap).**
The parts still follow the persona, now before `# Praxis`, the repository notes and the resources, the run's header and `# Task` coming last.
