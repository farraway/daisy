# 0167. The system prompt's stable blocks first

From Step E1 of the plan implementing the 2026-11-19 swarm roadmap (the maintainer's direction of 2026-11-21: token efficiency before the swarm resumes). Built in that step: `docs/swarm.md` sections 4 (The swarm's notes about a repository), 7.3 (the layout, The order, `# Resources`) and 7.6 (`prompt_hash`, `config_hash`). Amends decisions [102](0102-repos-name-md.md), [108](0108-the-swarms-notes-about-each-repository-and-drift-as-built.md), [116](0116-the-architect-and-the-worker-read-daisys-praxis-and-testing.md), [164](0164-prompt-parts-added-to-a-roles-prompt.md) and [165](0165-a-configuration-hash.md).

**Why.** A provider's prompt cache keeps a prompt's prefix: a run whose
system prompt begins with the bytes an earlier run's began with reads
those from the cache, at a fraction of the input price. Every
`system.md` began with its header line, `<!-- daisy-swarm system:
<owner>/<repo>#<n> <role> run <k>, <time> -->`, which names the run and
its time, so no two runs shared a byte of prefix, and `# Task`, which
names the issue, came second, before the praxis, the testing skill,
the repository notes and the resources that every run of the role
reads alike.

**The order.** `system.md` writes the blocks every run of a role shares
first, in the order they had: `# Persona`, each `# Prompt part:
<path>`, `# Praxis` and `# Testing` (the architect's and the worker's),
each `# Repository notes: <name>`, `# Resources`; then the run's own:
its header line and `# Task`. Two runs of one role on any issues share
every byte up to the header, given the same persona, parts, root,
notes and resources. The header keeps its text and moves with the
Task, the one block that names the run; dropping it was the
alternative, but the file would no longer say which run it is, and
`prompt_hash`, the sha256 of the whole file, would no longer differ
between runs of one configuration. Keeping the header first and moving
only `# Task` last was the other: the header's run and time would
still differ at the first byte. `prompt.md` is unchanged: it is the
material, which differs from run to run whole.

**The configuration's hash follows.** `config_hash` (decision 165) is
the sha256 of the text from `# Persona` up to the first block after the
prompt parts: any later line, outside a fenced block, that begins `# `
and is not `# Prompt part: `, since the persona's and the parts' own
headings are shifted below `# ` and a fenced line is not read. In a
`system.md` written before this step that block is `# Task`, and since
then `# Praxis`, `# Repository notes: <name>` or `# Resources`; the
text between is the same, so one persona with its parts hashes alike in
either order and a reprice of an earlier run's kept `system.md` gives
the hash it gave. Naming the blocks that may follow the parts (`# Task`,
`# Praxis`, `# Testing`, `# Repository notes:`, `# Resources`) was the
alternative: a block added later between the parts and the Task would
fall into the hash until the list named it.
