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

**Amended 2026-11-21: the output contract first** (the verdict fix of
the plan implementing the 2026-11-19 swarm roadmap; sections 7.3, The
order, 7.4, The output contract and The reminder, and 7.6,
`config_hash`). *What failed.* The order above put a reviewer's output
contract, that the first line be exactly `VERDICT: approve` or
`VERDICT: request-changes`, at the very end of a system prompt of some
28,000 characters, as one sentence in the middle of a Task paragraph of
some 2,500. On a smaller model, 11 of 11 reviews began with a correct
verdict line under the order before this decision, the Task right after
the persona, and 9 of 12 under this one: the other 3 began `## Review:
approved`, `## Review` and `## Review: round 5`, and each ended `failed:
bad verdict`, a stop for the maintainer. A larger model stayed at 33 of
33 under this order. The fault is the contract's place with a smaller
model, not the reading of the verdict, which is unchanged.

*The new order.* `# Persona`, then `# Output`, the role's output
contract: Daisy's text, one per role, naming the first lines its output
may or must begin with (for a reviewer the literal verdict lines, the
`[blocking]` and `[non-blocking]` finding lines, when to request
changes, and a template), then each `# Prompt part: <path>`, `# Praxis`
and `# Testing`, each `# Repository notes: <name>`, `# Resources`, the
header and `# Task` last. A Task whose first line is required (a
reviewer's, the architect's and the owner's on a question) ends, after
its closing paragraph, with a reminder restating it (`Begin your output
with the line "VERDICT: approve" or "VERDICT: request-changes".`), and
states it nowhere else; a contract asked for only in a case (`STANDS:`,
`NO CHANGE:`, a worker's or a reviewer's `QUESTION:`, every `BLOCKED:`)
keeps its sentence in the Task, which names the case, and the block
says it is optional. Since the block is the role's alone, it stays in
the prefix every run of the role shares, and the cache order holds.
`config_hash` leaves the block out, its bytes cut from the region
between `# Persona` and the first block after the parts, so one persona
with its parts hashes alike before and after the fix; hashing it with
them was the alternative, but it is Daisy's text, not the configuration
the hash names, as a Task text is (decision
[168](0168-a-delta-review.md)). Restating the verdict sentence at the
Task's end alone was the other: it leaves the contract at the end of
the prompt, where the smaller model missed it.
