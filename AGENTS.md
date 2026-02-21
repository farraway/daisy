# Daisy - System Architecture & Internal Specifications

This is the AGENTS.md for the daisy repository itself. It is auto-applied by Cursor when editing files within the daisy directory, providing system-level context for modifying the daisy productivity system (Mode 4: meta work).

**For user-facing workflows (Modes 1-3), see `prompts/daisy.md` (loaded via per-home AGENTS.md).**

## Purpose

This document is for:
- System designers working on daisy architecture
- Developers implementing new workflows or scripts
- Troubleshooting format or sync issues
- Understanding the detailed parsing and conversion algorithms

## Task Lifecycle

### State Diagram

```
  [created] ──(add task)──→ [active in todo.txt]
                                   │
                 ┌─────────────────┼─────────────────┐
                 │                 │                  │
           (done.sh)        (cancel)           (priority change)
                 │                 │                  │
                 ▼                 ▼                  ▼
        [completed in         [cancelled in      [repositioned in
         todo.txt]             todo.txt]          todo.txt]
         "x YYYY-MM-DD..."    "z YYYY-MM-DD..."
                 │                 │
                 │            (new-day.sh OR
                 │             new-week.sh)
                 │                 │
                 │                 ▼
                 │            [DELETED from
                 │             todo.txt]
                 │
            (new-week.sh ONLY)
                 │
                 ▼
        [archived in done.txt]
         removed from todo.txt
```

### Lifecycle Rules

1. **Completing ≠ Archiving.** `done.sh` marks a task complete (`x` prefix) but the task stays in `todo.txt`. They are only moved to `done.txt` during `new-week.sh`.

2. **Cancelling is soft-delete.** `cancel` marks a task with `z` prefix. Stays in `todo.txt` until `new-day.sh` or `new-week.sh` deletes it.

3. **`done.txt` is a write-once archive.** Tasks flow in during weekly archival and are never modified or moved back.

4. **`todo.txt` always reflects current reality.** Active tasks, recently completed (awaiting weekly archival), and cancelled (awaiting cleanup).

For detailed format specifications (regex patterns, conversion rules, file organization), see [`daisy/docs/task-format.md`](daisy/docs/task-format.md).

For synchronization rules and validation algorithms, see [`daisy/docs/task-sync.md`](daisy/docs/task-sync.md).

## CRITICAL: Format Preservation Rules

When modifying files, preserve EXACT formatting:

### Todo.txt Format Rules

- Active: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Completed: `x YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- Cancelled: `z YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- NEVER change date format (always `YYYY-MM-DD`)
- NEVER add/remove priority parentheses on active tasks
- NEVER preserve priority on completed/cancelled tasks
- NEVER reorder fields
- Preserve ALL spaces exactly

### Today.md Format Rules

- Preserve markdown heading levels (`####`)
- Preserve checkbox format: `- [ ]`, `- [x]`, `- [z]` (note spaces)
- NEVER add/remove blank lines between sections
- Time format: `HHMM` (24-hour, no colons)
- Log entry format: `- HHMM - message` (note dashes and spaces)

### Journal.md Archive Rules

- Append daily entries (curated/abridged during weekly review only)
- MAY be modified ONLY to:
  - Consolidate quiet days during weekly review
  - Update person references to canonical ~alias format
  - Fix chronological ordering errors
- NEVER delete stakeholder interactions or task progress
- NEVER modify retrospective content (represents historical perspective)

## Cross-File Consistency

When completing a task, ensure:
1. today.md checkbox changes `[ ]` → `[x]`
2. tasks/todo.txt gets `x YYYY-MM-DD` prefix
3. Priority STRIPPED from completed task
4. Completed task moves to END of todo.txt
5. Log entry added documenting completion

When referencing people:
1. Always use `~alias` from tasks/alias.txt
2. NEVER use bare names or emails

## Timestamps

- Use 24-hour format: `1430` not `2:30 PM`
- No colons in times: `1430` not `14:30`
- Default to Pacific Time unless specified otherwise

## Script Reference

| Script | Purpose |
|--------|---------|
| `daisy.sh` (repo root) | CLI entry point (`~/bin/daisy`) — dispatches subcommands |
| `daisy-init.sh` | Initialize Daisy in a workspace with a specific home |
| `new-day.sh` | Archive yesterday, generate new today.md |
| `new-week.sh` | Archive completed tasks to done.txt + new day |
| `done.sh` | Mark task complete in todo.txt and today.md |
| `log.sh` | Add timestamped log entry to today.md |
| `create-home.sh` | Create new home from template, optionally activate |
| `build-prompt.sh` | Generate home/{home}/AGENTS.md from include.txt |
| `common.sh` | Shared functions (resolve_home, require_env) |
| `healthcheck.sh` | System validation |

Current scripts do NOT yet implement: priority floor rules, task preservation from yesterday, quiet day consolidation, advanced sync validation, project management commands, JIRA sync from project files, or log audit during retrospective. These are handled by the agent following prompt instructions.

## Testing After Changes

After modifying scripts, templates, or workflow logic, validate against the test cases in [`daisy/docs/test-cases.md`](daisy/docs/test-cases.md). The test cases cover:

- New day/week generation (task extraction, template substitution)
- Task completion (cross-file consistency)
- Priority changes (repositioning in todo.txt and today.md)
- Sync validation

Run through the relevant scenarios mentally or by constructing sample inputs. Ensure output matches expected formats.

## AGENTS.md Build System

### Per-Home Output

`build-prompt.sh` generates `home/{home}/AGENTS.md` (not the repo root). Each home has its own AGENTS.md. Workspaces access it via `.daisy/AGENTS.md` symlink.

Usage: `$DAISY_ROOT/daisy/scripts/build-prompt.sh [home-name]`

If no home-name argument is given, the script resolves the home via `.daisy/home` or `$DAISY_HOME`.

### Lazy Loading Architecture

To keep `AGENTS.md` small and save tokens, prompts can be included in two modes:

**Full inclusion** (default): The entire prompt file is embedded in `AGENTS.md`. Used for core prompts that are always needed (e.g., `daisy`).

**Lazy inclusion** (`~` prefix in `include.txt`): Only the `## Trigger` section from the prompt file is embedded. The trigger section tells the agent when to read the full prompt on demand.

### include.txt Format

```
# Full inclusion (entire file in AGENTS.md)
daisy

# Lazy inclusion (trigger only, full file read on demand)
~retrospective
```

### Creating a New Prompt

1. Create `prompts/{name}.md`
2. Add a `## Trigger` section as the first heading (required for lazy loading):

```markdown
## Trigger

Read the full `daisy/prompts/{name}.md` when:
- {condition 1}
- {condition 2}
- {condition 3}

# {Prompt Title}
...rest of prompt...
```

3. Add `~{name}` (lazy) or `{name}` (full) to the appropriate `home/{home}/include.txt`
4. Rebuild: `$DAISY_ROOT/daisy/scripts/build-prompt.sh {home-name}`

The `build-prompt.sh` script extracts everything between `## Trigger` and the next heading (`#` or `##`). If no `## Trigger` section is found, the prompt falls back to full inclusion with a warning.

### Token Budget

With lazy loading, the work home `AGENTS.md` is approximately:
- **~500 lines** (trigger-only mode) vs **~1700 lines** (all-full mode)
- ~70% reduction in tokens per session
- Full prompt files are only loaded when their trigger conditions match

## Reference Documentation

Detailed algorithms, format specifications, and examples have been extracted into focused reference docs:

- [`daisy/docs/task-format.md`](daisy/docs/task-format.md) - Task format regex, conversion rules, file organization
- [`daisy/docs/task-sync.md`](daisy/docs/task-sync.md) - Bidirectional sync rules, validation algorithm
- [`daisy/docs/templates.md`](daisy/docs/templates.md) - Template placeholders and formatting rules
- [`daisy/docs/workflows.md`](daisy/docs/workflows.md) - Status, add task, change priority algorithms
- [`daisy/docs/logging.md`](daisy/docs/logging.md) - Logging design rationale, triggers, abridged archival
- [`daisy/docs/projects.md`](daisy/docs/projects.md) - Project management architecture and algorithms
- [`daisy/docs/home-management.md`](daisy/docs/home-management.md) - Per-workspace home resolution, health checks
- [`daisy/docs/examples.md`](daisy/docs/examples.md) - Complete interaction walkthroughs
- [`daisy/docs/todotxt.md`](daisy/docs/todotxt.md) - Todo.txt format specification
- [`daisy/docs/test-cases.md`](daisy/docs/test-cases.md) - Validation test cases (run after system changes)

## See Also

- **`prompts/daisy.md`** - User-focused workflows (loaded via per-home AGENTS.md)
