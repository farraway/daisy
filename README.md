# Daisy - Personal Productivity System

A lightweight productivity system combining todo.txt task management with daily markdown journaling, designed for AI-assisted workflows.

## Quick Start

**👋 New to daisy?** → **[5-Minute Quickstart Guide](docs/quickstart.md)**

**For AI Assistants:**
```
Load @daisy/prompt.md and start a new day
```

This single command loads all prompts, archives yesterday, and creates fresh `today.md` with prioritized tasks.

**Alternative commands:**
- `Load @daisy/prompt.md` - Initialize system only
- `Load @daisy/prompt.md and start a new week` - Archive completed tasks + new day

**Documentation:**
- **[Quickstart Guide](docs/quickstart.md)** - Get started in 5 minutes ⭐
- **[AI Workflow Guide](prompts/daisy.md)** - Complete system specification
- **[Detailed Examples](docs/examples/daisy.md)** - Interaction walkthroughs
- **[Test Cases](docs/test-cases.md)** - Validation test cases

**Manual prompt loading (if needed):**
- **Home-specific:** `@daisy/prompts/work.md` (for work contexts)
- **Utilities:** `@daisy/prompts/jira.md`, `@daisy/prompts/github.md`, `@daisy/prompts/webex.md`
- **Reflection:** `@daisy/prompts/retrospective.md`

## File Structure

```
daisy/
├── prompt.md            # Symlink → Active home's bootstrap prompt
├── journal.md           # Symlink → Active home's journal archive
├── today.md             # Symlink → Active home's current day journal
├── tasks/               # Symlink → Active home's task directory
│   ├── todo.txt         # Active and recently completed tasks
│   ├── done.txt         # Long-term task archive
│   └── alias.txt        # People/role aliases (~person format)
├── home/
│   ├── work/           # Work home data
│   │   ├── prompt.md    # Work bootstrap prompt
│   │   ├── journal/
│   │   ├── tasks/
│   │   └── perf/        # Performance reflections
│   └── personal/        # Personal home (example)
│       ├── prompt.md
│       ├── journal/
│       └── tasks/
├── prompts/
│   ├── daisy.md         # Core workflow instructions for AI
│   ├── work.md         # Work-specific augmentations
│   ├── jira.md          # JIRA utilities
│   ├── github.md        # GitHub utilities
│   ├── webex.md         # Webex API utilities
│   └── retrospective.md # Reflection guide
├── docs/
│   ├── quickstart.md    # 5-minute getting started guide
│   ├── todotxt.md       # Todo.txt format specification
│   ├── test-cases.md    # Validation test suite
│   └── examples/
│       └── daisy.md     # Detailed interaction examples
├── templates/
│   ├── journal-day.md   # Template for daily entries
│   ├── journal-week.md  # Template for weekly entries
│   └── home/            # Template for new homes
│       ├── prompt.md    # Bootstrap prompt template
│       ├── journal/     # Journal directory structure
│       └── tasks/       # Tasks directory structure
```

## Home Switching

The symlinks (`journal.md`, `today.md`, `tasks/`, `prompt.md`) point to the active home's data, enabling easy context switching (e.g., work ↔ personal).

**To switch homes:**

```bash
ln -sf home/personal/prompt.md prompt.md
ln -sf home/personal/journal/journal.md journal.md
ln -sf home/personal/journal/today.md today.md
ln -sf home/personal/tasks tasks
```

Each home has its own bootstrap prompt that specifies which prompts to load.

## Creating a New Home

To create a new home (e.g., "sideprojects"):

1. **Copy the template:**
   ```bash
   cp -r templates/home home/sideprojects
   ```

2. **Customize the bootstrap prompt:**
   - Edit `home/sideprojects/prompt.md`
   - Replace `[home]` with `sideprojects`
   - Replace `[Home Name]` with descriptive name
   - Choose which prompts to load (remove work.md, jira.md if not needed)

3. **Create symlinks:**
   ```bash
   ln -sf home/sideprojects/prompt.md prompt.md
   ln -sf home/sideprojects/journal/journal.md journal.md
   ln -sf home/sideprojects/journal/today.md today.md
   ln -sf home/sideprojects/tasks tasks
   ```

4. **Start using:**
   ```
   Load @daisy/prompt.md and start a new day
   ```

## Todo.txt Format Quick Reference

```
(A) 2026-01-16 High priority task +Project @context
(B) 2026-01-16 Medium priority task +PROJ-5678 @jira
2026-01-16 Inbox task (needs prioritization)
(C) 2026-01-16 Normal task @github
(D) 2026-01-16 Someday task +backlog
```

**Priority levels:**
- `(A)` - Now (do today)
- `(B)` - Next (this week)
- `(C)` - Soon (default for most tasks)
- `(D)` - Someday (backlog)
- No priority - Inbox (needs triage)

**Common contexts:**
- `@jira` - JIRA tickets
- `@git` or `@github` - Pull requests
- `@meeting` - Meetings/calendar items
- `@review` - Needs attention

**Common projects:**
- `+WXSA-XXXXX` - Jira ticket keys
- `+FY26Q2` - Fiscal quarter tags
- `+repo-name` - Repository names

See [docs/todotxt.md](docs/todotxt.md) for complete format specification.

## People References

Use `~alias` to reference people consistently across all files. The alias is defined in `tasks/alias.txt`:

```
~jdoe Jane Doe <jdoe@example.com> #jane #manager
~deaclose Dean Close <deaclose@cisco.com> #me #dean
```

**Usage:**
- In tasks: `(B) 2026-01-16 Review design with ~jdoe @meeting`
- In logs: `1430 - Met with ~jdoe about architecture`
- In journal: `Discussed approach with ~jdoe, got approval`

This enables consistent cross-referencing and avoids ambiguity (first names, nicknames, etc.)

## Example Daily Entry

```markdown
### 2026-01-16 Thursday

#### Agenda
- 0930 Stand-up meeting
- 1400 Sprint planning
- 1530 Code review session

#### High-Priority Tasks
- [x] @jira Complete PROJ-1234 implementation +PROJ-1234 +FY26Q2
- [ ] @jira Review design doc for PROJ-1235 +PROJ-1235

#### Task Inbox
- [ ] Triage new tickets +INBOX

#### GitHub PRs
- [x] @git Review PR#1538 +WXSA-15770

#### Log
- 0930 - New day started
- 1045 - Traced PROJ-1234 to race condition in adapter init
- 1215 - Met with ~jdoe about approach, decided instance-based pattern
- 1530 - PR#1545 opened
- 1600 - PR#1545 merged

#### Retrospective
- **Successes:** Found root cause efficiently, collaborated well with ~jdoe
- **Misses:** Could have asked for help earlier in debugging
- **What would a Sage do next:** Document the race condition pattern for team
```

## Daily Workflow Summary

For detailed workflow instructions, see the **[Quickstart Guide](docs/quickstart.md)**.

**Essential commands:**
- `start a new day` - Archive yesterday, create fresh today.md
- `start a new week` - Archive completed tasks, start new day
- `done [task]` - Mark task complete
- `log [message]` - Add timestamped entry
- `check system` - Verify setup and file integrity

**AI behavior:**
- Proactively logs discoveries, decisions, milestones
- Helps with daily retrospective
- Curates journal archives for utility
- Maintains format consistency

## Philosophy

- **Plain text** - Git-friendly, searchable, future-proof
- **AI-native** - Designed for conversational AI assistance
- **Flexible** - Adapt to your workflow, not vice versa
- **Reflective** - Daily retrospectives drive continuous improvement
- **Multi-context** - Switch between work/personal/projects using homes

## See Also

- **[Quickstart Guide](docs/quickstart.md)** - Get started in 5 minutes ⭐
- **[AI Workflow Guide](prompts/daisy.md)** - Complete system specification (includes priority system)
- **[Detailed Examples](docs/examples/daisy.md)** - Interaction walkthroughs
- **[Test Cases](docs/test-cases.md)** - Validation test cases
- [Todo.txt Specification](docs/todotxt.md) - Format reference
- [Work-Specific Workflows](prompts/work.md) - Work conventions
- [Retrospective Guide](prompts/retrospective.md) - Reflection prompts
