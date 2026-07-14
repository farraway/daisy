# Daisy - System Architecture & Internal Specifications

This AGENTS.md is for **Mode 4: meta work** — modifying the Daisy system itself (scripts, prompts, templates). It is auto-applied by Cursor and Claude Code when editing files in this directory.

**For user-facing workflows (Modes 1–3), see `prompts/daisy.md` (loaded via per-home AGENTS.md).**

---

## CRITICAL: Format Preservation Rules

When modifying any Daisy data files, preserve EXACT formatting.

### Todo.txt Format

- Active: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Completed: `x YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- Cancelled: `z YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- NEVER change date format (always `YYYY-MM-DD`)
- NEVER add/remove priority parentheses on active tasks
- NEVER preserve priority on completed/cancelled tasks
- NEVER reorder fields
- Preserve ALL spaces exactly

### Today.md Format

- Preserve markdown heading levels (`####`)
- Preserve checkbox format: `- [ ]`, `- [x]`, `- [z]` (note spaces)
- NEVER add/remove blank lines between sections
- Time format: `HHMM` (24-hour, no colons)
- Log entry format: `- HHMM - message` (note dashes and spaces)

### Journal.md Archive Rules

- Append daily entries; abridging ONLY during weekly review
- MAY be modified ONLY to: consolidate quiet days, update ~alias format, fix ordering errors
- NEVER delete stakeholder interactions, task progress, or retrospective content

---

## Script Reference

| Script | Purpose |
|--------|---------|
| `daisy.sh` (repo root) | CLI entry point — dispatches subcommands |
| `daisy-init.sh` | Initialize Daisy in a workspace with a specific home |
| `new-day.sh` | Archive yesterday, generate new today.md, rotate journal.md |
| `new-week.sh` | Archive completed tasks to done.txt + new day, rotate journal.md |
| `rotate.sh` | Archive journal.md day-blocks into rolling/closed window files |
| `done.sh` | Mark task complete in todo.txt and today.md |
| `log.sh` | Add timestamped log entry to today.md |
| `feedback.sh` | Record a prompt failure with workflow tag and description |
| `optimize.sh` | LLM-driven prompt optimization loop (train/test split, diff, approval, archives used entries) |
| `eval.sh` | List, display, or record eval case results for the optimize loop |
| `create-home.sh` | Create new home from template, optionally activate |
| `build-prompt.sh` | Generate home/{home}/AGENTS.md from include.txt |
| `common.sh` | Shared functions (resolve_home, require_env) |
| `commit.sh` | Git commit helper — stages and commits changes under home/ |
| `check-secrets.sh` | Report which API tokens/secrets are configured in .env.sh |
| `healthcheck.sh` | System validation |
| `files.sh` | Report resolved real paths for every per-home file/directory |
| `list.sh` | List active prompts (with trigger summary) and installed skills (with description) |
| `projects.sh` | List active or archived projects with resolved paths |
| `plan-new.sh` | Create a new Daisy plan file, symlink as PLAN.md (or, with `--spec`, an unregistered draft in the working directory) |
| `plan-pickup.sh` | Promote a local spec draft (from `plan-new.sh --spec`) into a tracked plan |
| `plan-archive.sh` | Archive the active Daisy plan, remove the PLAN.md symlink |

**Documentation policy:** every script in `daisy/scripts/` must have a row in this table, and — if it implements the `--healthcheck` contract — a corresponding entry in `healthcheck.sh`'s `HEALTHCHECK_SCRIPTS` array. `healthcheck.sh` includes an automated drift check for the table half of this; there is no automated check for the array half beyond that same script list (see `daisy/scripts/healthcheck.sh`).

Scripts do NOT yet implement: priority floor rules, task preservation from yesterday, quiet day consolidation, advanced sync validation, JIRA sync, or log audit during retrospective. These are handled by the agent following prompt instructions.

After modifying scripts or workflow logic, validate against [`daisy/docs/test-cases.md`](daisy/docs/test-cases.md).

---

## Development Heuristics

**Prototype manually before automating.** When adding a new workflow or modifying a complex script, do the operation by hand first — no scripts, no automation. The manual run reveals hidden constraints and invariants that are invisible in the abstract. Only after understanding what a correct manual execution looks like should you codify it into a script or prompt rule. This is especially true for multi-file operations (new-day, new-week) and sync workflows.

---

## AGENTS.md Build System

`build-prompt.sh` generates `home/{home}/AGENTS.md` from `home/{home}/include.txt`. Workspaces access it via `.daisy/AGENTS.md` symlink.

Usage: `daisy build [home-name]`

`include.txt` is prompts-only. Skills (`skills/`) have no manifest — every
skill in the merged root+home set always installs at `daisy init`; see
[`daisy/docs/prompts-vs-skills.md`](daisy/docs/prompts-vs-skills.md) for why
these are different systems.

### Lazy Loading Architecture

**Full inclusion** (default): entire prompt embedded in AGENTS.md.

**Lazy inclusion** (`~` prefix in include.txt): only the `## Trigger` section is embedded. The model reads the full file on demand when the trigger condition is met.

### include.txt Format

```
# Full inclusion
personal

# Lazy inclusion (trigger only)
~daisy
~retrospective
~github
~plan
```

### Creating a New Prompt

1. Create `prompts/{name}.md` with a `## Trigger` section as the first heading
2. Add `~{name}` (lazy) or `{name}` (full) to `home/{home}/include.txt`
3. Rebuild: `daisy build {home-name}`

The build script extracts everything between `## Trigger` and the next `#` or `##` heading for lazy stubs. If no `## Trigger` is found, falls back to full inclusion with a warning.

### Creating a New Skill

Before creating either a prompt or a skill, apply the portability test in
[`daisy/docs/prompts-vs-skills.md`](daisy/docs/prompts-vs-skills.md): would
this behave identically to a bare agent that never loaded Daisy's persona? If
yes, it's a skill, not a prompt.

1. Create `skills/{name}/SKILL.md` with `name`/`description` frontmatter (plus
   `disable-model-invocation: true` if it should only ever be invoked by name)
2. No manifest entry needed — every skill under `skills/` (root, then home,
   home overriding root by name) installs automatically at `daisy init`
3. Re-run `daisy init` in any workspace that should pick it up — skills are
   copied, not symlinked, so this is the only way to refresh one

---

## Reference Documentation

- [`daisy/docs/prompts-vs-skills.md`](daisy/docs/prompts-vs-skills.md) — The hat/skill distinction, the portability test, worked examples
- [`daisy/docs/task-format.md`](daisy/docs/task-format.md) — Task format regex, conversion rules, file organization
- [`daisy/docs/task-sync.md`](daisy/docs/task-sync.md) — Bidirectional sync rules, validation algorithm
- [`daisy/docs/templates.md`](daisy/docs/templates.md) — Template placeholders and formatting rules
- [`daisy/docs/workflows.md`](daisy/docs/workflows.md) — All workflow algorithms (new-day, new-week, done, cancel, add, priority, sync, etc.)
- [`daisy/docs/logging.md`](daisy/docs/logging.md) — Logging design rationale, triggers, abridged archival
- [`daisy/docs/projects.md`](daisy/docs/projects.md) — Project management architecture and algorithms
- [`daisy/docs/praxis.md`](daisy/docs/praxis.md) — Implementation quality reference: how we work, anti-patterns, extended guidance
- [`daisy/docs/retrospective.md`](daisy/docs/retrospective.md) — Retrospective guide and integration workflows
- [`daisy/docs/plan.md`](daisy/docs/plan.md) — Plan workflow reference
- [`daisy/docs/github.md`](daisy/docs/github.md) — GitHub operations reference
- [`daisy/docs/home-management.md`](daisy/docs/home-management.md) — Per-workspace home resolution, health checks
- [`daisy/docs/examples.md`](daisy/docs/examples.md) — Complete interaction walkthroughs
- [`daisy/docs/todotxt.md`](daisy/docs/todotxt.md) — Todo.txt format specification
- [`daisy/docs/test-cases.md`](daisy/docs/test-cases.md) — Validation test cases
- [`daisy/docs/resources/prompt-learning-loop.md`](daisy/docs/resources/prompt-learning-loop.md) — Arize talk: feedback loops, rules-section optimization, eval co-evolution
- [`daisy/docs/resources/infinite-software-crisis.md`](daisy/docs/resources/infinite-software-crisis.md) — Netflix/Nations: simple vs easy, essential vs accidental complexity, three-phase approach
- [`daisy/docs/resources/progressive-disclosure.md`](daisy/docs/resources/progressive-disclosure.md) — Developers Digest: lazy-load context on demand, tools as files, memory as markdown; validates Daisy's existing lazy loading architecture
- [`daisy/docs/resources/agents-in-complex-codebases.md`](daisy/docs/resources/agents-in-complex-codebases.md) — HumanLayer/Horthy: context quality over quantity, RPI calibration, step granularity, intentional compaction on repeated corrections

## See Also

- **`prompts/daisy.md`** — User-focused workflows (loaded via per-home AGENTS.md)
