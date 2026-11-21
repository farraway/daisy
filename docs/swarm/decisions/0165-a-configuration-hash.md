# 0165. A configuration hash

From Step R3-12b of the plan implementing the 2026-11-19 swarm roadmap (2026-11-21, from Step R3-12's open question). Built: `docs/swarm.md` sections 7.6 (The run's record, `config_hash`) and 11 (swarm metrics: the reprice, The scorecard's By configuration); `docs/cli.md`'s `metrics add` columns and `swarm metrics` stdout.

**Why.** Step R3-12's scorecard groups a role's runs by configuration
so that two setups compare, and it grouped them by `prompt_hash`, the
sha256 of the run's whole `system.md`. That file's first line names the
issue, role, run and time, and its Task, repository notes and resources
follow the issue and stage, so every run had a hash of its own and the
scorecard a line per run: no two runs of one setup read together.

**The column.** The metrics CSV gains a last column, `config_hash`: the
sha256 of the configuration's text in the run's `<run>/io/system.md`,
from its `# Persona` line up to its `# Task` line, the role's prompt
and its prompt parts (decision
[164](0164-prompt-parts-added-to-a-roles-prompt.md)) under their
headings, as Render writes them alike for every run of the role. A line
in a fenced block is neither heading (a persona's own headings are
shifted, a fenced line is not). It is made at the collection, as
`prompt_hash` is, and empty when the file is gone or holds no such
lines. `prompt_hash` stays as it was. A CSV under the header before it
(up to `cause`) is read with the cell empty and its header line
rewritten at the next append, as each column before it.

**Earlier rows.** `--backfill` makes the cell as a collection does;
`--reprice` makes it from the run's kept `system.md` and supersedes a
row whose cell differs, so a row recorded before the column gets the
hash a new run of the same text records. A run without the file keeps
its row's cell, and the scorecard groups a role's rows without one, of
one provider, model and effort, as one line, `config unknown`.

**The scorecard** groups by role, provider, model, effort and
`config_hash`, the line reading `config <hash>` (its first 12
characters) in place of `prompt <hash>`.

**Alternatives.** A result line written at the launch naming the hash
of the persona and parts as read (one more line to keep in step with
system.md, and a backfill would still have to read system.md for runs
launched before it); a hash of the persona file and the part files
as they stand in the clone at the collection (a file edited between the
launch and the collection would be credited to the wrong run, and an
old run's text could not be recovered); the persona, the parts and
praxis (praxis and the testing skill are the root's, which `root_sha`
names, not the maintainer's configuration); `prompt_hash` redefined in
place (its rows' meaning would change under them, and the open
question asked for a second hash).

Tests: `TestSwarmConfigHash`, `TestConfigHash`,
`TestCauseHeaderReadAndExtended`, `TestScorecardByConfiguration`.

**Amended by decision
[167](0167-the-system-prompt-stable-blocks-first.md)
(Step E1 of the plan implementing the 2026-11-19 swarm roadmap).**
The region runs from `# Persona` to the first block after the prompt parts, `# Task` in a `system.md` written before that step and `# Praxis`, `# Repository notes: <name>` or `# Resources` since, so the hash of one persona with its parts is unchanged.
