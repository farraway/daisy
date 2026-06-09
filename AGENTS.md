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
| `new-day.sh` | Archive yesterday, generate new today.md |
| `new-week.sh` | Archive completed tasks to done.txt + new day |
| `done.sh` | Mark task complete in todo.txt and today.md |
| `log.sh` | Add timestamped log entry to today.md |
| `feedback.sh` | Record a prompt failure with workflow tag and description |
| `optimize.sh` | LLM-driven prompt optimization loop (train/test split, diff, approval) |
| `create-home.sh` | Create new home from template, optionally activate |
| `build-prompt.sh` | Generate home/{home}/AGENTS.md from include.txt |
| `common.sh` | Shared functions (resolve_home, require_env) |
| `healthcheck.sh` | System validation |

Scripts do NOT yet implement: priority floor rules, task preservation from yesterday, quiet day consolidation, advanced sync validation, project management commands, JIRA sync, or log audit during retrospective. These are handled by the agent following prompt instructions.

After modifying scripts or workflow logic, validate against [`daisy/docs/test-cases.md`](daisy/docs/test-cases.md).

---

## AGENTS.md Build System

`build-prompt.sh` generates `home/{home}/AGENTS.md` from `home/{home}/include.txt`. Workspaces access it via `.daisy/AGENTS.md` symlink.

Usage: `$DAISY_ROOT/daisy/scripts/build-prompt.sh [home-name]`

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
3. Rebuild: `$DAISY_ROOT/daisy/scripts/build-prompt.sh {home-name}`

The build script extracts everything between `## Trigger` and the next `#` or `##` heading for lazy stubs. If no `## Trigger` is found, falls back to full inclusion with a warning.

---

## Reference Documentation

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
