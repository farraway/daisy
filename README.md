# Daisy - Personal Productivity System

A lightweight productivity system combining todo.txt task management with daily markdown journaling, designed for AI-assisted workflows.

## Bootstrap (First-Time Setup)

**New installation?** Follow these steps to get started:

### 1. Clone or Download

```bash
git clone <repo-url> daisy
cd daisy
```

### 2. Set Environment Variables

Add to `~/.zshenv` (or `~/.bashrc`):
```bash
export DAISY_ROOT="/path/to/daisy"
export DAISY_HOME="$DAISY_ROOT/home/default"  # or choose a home
```

Then reload: `source ~/.zshenv`

### 3. Verify Installation

```bash
$DAISY_ROOT/scripts/healthcheck.sh
```

This checks that environment variables are set and all components are healthy.

### 4. Build Your Prompt

**Important:** Daisy uses a prompt composition system. You must build `prompt.md` before using:

```bash
$DAISY_ROOT/scripts/build-prompt.sh
```

This reads `$DAISY_HOME/include.txt` and concatenates all prompts into `prompt.md`.

**What this does:**
- Reads the list of prompts from your active home's `include.txt`
- Concatenates those prompts into a single `prompt.md` file
- This generated file is what the AI assistant loads

### 5. Start Using Daisy

In a new AI session:
```
Load @daisy/prompt.md and start a new day
```

The AI will:
- Load all your configured prompts
- Archive yesterday's work
- Create fresh `today.md` with prioritized tasks

---

## Quick Start (After Bootstrap)

## Quick Start (After Bootstrap)

**Already set up?** Quick reference for daily use:

**For AI Assistants:**
```
Load @daisy/prompt.md and start a new day
```

### Maintenance Commands

**Verify system health:**
```bash
$DAISY_ROOT/scripts/healthcheck.sh         # Uses cache
$DAISY_ROOT/scripts/healthcheck.sh --force # Full re-check
```

**Rebuild prompt after changes:**
```bash
$DAISY_ROOT/scripts/build-prompt.sh
```

Rebuild when:
- You modify `$DAISY_HOME/include.txt` (add/remove prompts)
- You edit source prompts in `prompts/`
- You switch homes

### Documentation

### Optional: API Authentication

For API integrations (Webex, JIRA, GitHub) when MCP servers are unavailable:

1. Copy the template to your workspace:
   ```bash
   cp $DAISY_ROOT/templates/env.sh.template .env.sh
   ```

2. Edit `.env.sh` and fill in your tokens:
   ```bash
   export DAISY_SECRET_WEBEX_API_TOKEN="your-token"
   export DAISY_SECRET_JIRA_API_TOKEN="your-token"
   export DAISY_SECRET_GITHUB_TOKEN="your-token"
   ```

3. Verify configuration:
   ```bash
   $DAISY_ROOT/scripts/check-secrets.sh
   ```

**Note:** MCP servers handle authentication automatically. `.env.sh` is only needed as a fallback.

### 5. Start Using Daisy

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
├── prompt.md            # Generated - concatenated prompts
├── journal.md           # Symlink → Active home's journal archive
├── today.md             # Symlink → Active home's current day journal
├── tasks/               # Symlink → Active home's task directory
│   ├── todo.txt         # Active and recently completed tasks
│   ├── done.txt         # Long-term task archive
│   └── alias.txt        # People/role aliases (~person format)
├── scripts/
│   ├── healthcheck.sh   # System validation (--force to re-run)
│   ├── build-prompt.sh  # Generate prompt.md from includes
│   ├── check-secrets.sh # Verify .env.sh configuration
│   └── daisy/           # Workflow scripts
│       ├── new-day.sh   # Start new day
│       ├── new-week.sh  # Start new week
│       ├── done.sh      # Mark task complete
│       └── log.sh       # Add log entry
├── home/
│   ├── work/           # Work home data
│   │   ├── include.txt  # List of prompts to load
│   │   ├── journal/
│   │   ├── tasks/
│   │   └── perf/        # Performance reflections
│   └── personal/        # Personal home (example)
│       ├── include.txt
│       ├── journal/
│       └── tasks/
├── prompts/
│   ├── daisy.md         # Core workflow instructions for AI
│   ├── daisy-admin.md   # Internal architecture (for system work)
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
│   ├── env.sh.template  # Environment variables template
│   ├── journal-day.md   # Template for daily entries
│   ├── journal-week.md  # Template for weekly entries
│   └── home/            # Template for new homes
│       ├── include.txt  # Prompts to load
│       ├── prompt.md    # (deprecated - use include.txt)
│       ├── journal/     # Journal directory structure
│       └── tasks/       # Tasks directory structure
```

## Home Switching

The symlinks (`journal.md`, `today.md`, `tasks/`) point to the active home's data, enabling easy context switching (e.g., work ↔ personal).

**To switch homes:**

```bash
ln -sf home/personal/journal/journal.md journal.md
ln -sf home/personal/journal/today.md today.md
ln -sf home/personal/tasks tasks
```

Then rebuild the prompt:
```bash
export DAISY_HOME="$DAISY_ROOT/home/personal"
$DAISY_ROOT/scripts/build-prompt.sh
```

Each home has an `include.txt` file that specifies which prompts to load.

## Creating a New Home

To create a new home (e.g., "sideprojects"):

1. **Copy the template:**
   ```bash
   cp -r templates/home home/sideprojects
   ```

2. **Customize the prompt includes:**
   - Edit `home/sideprojects/include.txt`
   - List the prompts you want to load (one per line)
   - Example:
     ```
     daisy
     retrospective
     github
     ```

3. **Create symlinks:**
   ```bash
   ln -sf home/sideprojects/journal/journal.md journal.md
   ln -sf home/sideprojects/journal/today.md today.md
   ln -sf home/sideprojects/tasks tasks
   ```

4. **Build prompt and start using:**
   ```bash
   export DAISY_HOME="$DAISY_ROOT/home/sideprojects"
   $DAISY_ROOT/scripts/build-prompt.sh
   ```
   
   Then load `@daisy/prompt.md` and start a new day

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
- **[AI Workflow Guide](prompts/daisy.md)** - User-focused workflows and commands
- **[Admin Guide](prompts/daisy-admin.md)** - Internal architecture and specifications
- **[Detailed Examples](docs/examples/daisy.md)** - Interaction walkthroughs
- **[Test Cases](docs/test-cases.md)** - Validation test cases
- [Todo.txt Specification](docs/todotxt.md) - Format reference
- [Work-Specific Workflows](prompts/work.md) - Work conventions
- [Retrospective Guide](prompts/retrospective.md) - Reflection prompts
