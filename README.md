# Daisy - Personal Productivity System

A lightweight productivity system combining todo.txt task management with daily markdown journaling, designed for AI-assisted workflows.

## Bootstrap (First-Time Setup)

### 1. Clone or Download

```bash
git clone <repo-url> daisy
cd daisy
```

### 2. Set Environment Variable

Add to `~/.zshenv` (or `~/.bashrc`):
```bash
export DAISY_ROOT="/path/to/daisy"
```

Then reload: `source ~/.zshenv`

### 3. Add `daisy-init` to your PATH

```bash
ln -s $DAISY_ROOT/scripts/daisy-init.sh ~/bin/daisy-init
```

### 4. Build Prompts for Your Home

```bash
$DAISY_ROOT/scripts/build-prompt.sh cisco    # or whichever home
```

This generates `home/work/AGENTS.md` from the home's `include.txt`.

### 5. Initialize Daisy in a Workspace

```bash
cd /path/to/your/project
daisy-init cisco
```

This creates:
- `daisy/` symlink to the shared daisy repo
- `.daisy/` directory with symlinks to the work home's tasks, journal, and projects
- `.cursor/rules/daisy.md` so agents automatically discover daisy
- `.daisy/` and `daisy` entries in `.gitignore`

### 6. Start Using Daisy

With the cursor rule installed, just say:
```
Daisy, start a new day
```

The agent will:
- Read `.daisy/AGENTS.md` automatically
- Archive yesterday's work
- Create fresh `today.md` with prioritized tasks

---

## Quick Start (After Bootstrap)

**Already set up?** Quick reference for daily use.

Just address Daisy by name in any initialized workspace:
```
Daisy, start a new day
Daisy, done [task]
Daisy, log [message]
Daisy, what are my tasks?
```

### Maintenance Commands

**Verify system health:**
```bash
$DAISY_ROOT/scripts/healthcheck.sh
```

**Rebuild prompt after editing prompts:**
```bash
$DAISY_ROOT/scripts/build-prompt.sh cisco
```

**Initialize Daisy in a new workspace:**
```bash
cd /path/to/new/project
daisy-init cisco
```

**Switch a workspace to a different home:**
```bash
daisy-init personal
```

### Optional: API Authentication

For API integrations (Webex, JIRA, GitHub) when MCP servers are unavailable:

1. Copy the template to your workspace:
   ```bash
   cp $DAISY_ROOT/templates/env.sh.template .env.sh
   ```

2. Edit `.env.sh` and fill in your tokens

3. Verify configuration:
   ```bash
   $DAISY_ROOT/scripts/check-secrets.sh
   ```

**Note:** MCP servers handle authentication automatically. `.env.sh` is only needed as a fallback.

## File Structure

### Daisy Repo

```
daisy/
├── AGENTS.md               # System architecture & internal specs (auto-applied by Cursor)
├── scripts/
│   ├── daisy-init.sh       # Initialize daisy in a workspace (symlink to ~/bin)
│   ├── build-prompt.sh     # Generate home/{home}/AGENTS.md
│   ├── healthcheck.sh      # System validation
│   ├── check-secrets.sh    # Verify .env.sh configuration
│   ├── commit.sh           # Auto-commit helper
│   └── daisy/              # Workflow scripts
│       ├── common.sh       # Shared functions (resolve_home, require_env)
│       ├── new-day.sh      # Start new day
│       ├── new-week.sh     # Start new week
│       ├── done.sh         # Mark task complete
│       ├── log.sh          # Add log entry
│       ├── create-home.sh  # Create new home from template
│       └── switch-home.sh  # (deprecated - use daisy-init)
├── home/
│   ├── work/              # Work home
│   │   ├── AGENTS.md       # Generated prompt (git-ignored)
│   │   ├── include.txt     # List of prompts to load
│   │   ├── journal/
│   │   ├── tasks/
│   │   ├── projects/
│   │   └── perf/
│   └── personal/         # Personal home
│       ├── AGENTS.md
│       ├── include.txt
│       ├── journal/
│       ├── tasks/
│       └── projects/
├── prompts/
│   ├── daisy.md            # Core workflow instructions for AI
│   ├── agents-md.md        # Guide for writing AGENTS.md files
│   ├── work.md            # Work-specific augmentations
│   ├──       # Work internal GitHub Enterprise
│   ├── github.md           # Public GitHub
│   ├── jira.md             # JIRA utilities
│   ├── webex.md            # Webex API utilities
│   └── retrospective.md    # Reflection guide
├── templates/
│   ├── cursor-rule.md      # Cursor rule for workspace integration
│   ├── project.md          # Template for project files
│   ├── journal-day.md      # Template for daily entries
│   ├── journal-week.md     # Template for weekly entries
│   ├── env.sh.template     # Environment variables template
│   └── home/               # Template for new homes
└── docs/
    ├── quickstart.md
    ├── todotxt.md
    ├── test-cases.md
    └── daisy/              # Detailed reference documentation
        ├── task-format.md  # Task format regex and conversion rules
        ├── task-sync.md    # Bidirectional sync rules
        ├── templates.md    # Template placeholder specs
        ├── workflows.md    # Status, add task, change priority algorithms
        ├── logging.md      # Logging design and archival rules
        ├── projects.md     # Project management architecture
        ├── home-management.md  # Per-workspace home resolution
        └── examples.md     # Interaction walkthroughs
```

### Workspace Layout (after `daisy-init cisco`)

```
workspace/
├── daisy/                  → $DAISY_ROOT (shared repo)
├── .daisy/
│   ├── home                # "work" (plain text)
│   ├── AGENTS.md           → daisy/home/work/AGENTS.md
│   ├── tasks/              → daisy/home/work/tasks/
│   ├── today.md            → daisy/home/work/journal/today.md
│   ├── journal.md          → daisy/home/work/journal/journal.md
│   └── projects/           → daisy/home/work/projects/
├── .cursor/rules/
│   └── daisy.md            → daisy/templates/cursor-rule.md
```

Different workspaces can use different homes concurrently.

## Homes

Homes isolate task/journal/project data for different contexts (e.g., work vs personal).

**Create a new home:**
```bash
$DAISY_ROOT/scripts/daisy/create-home.sh sideprojects
```

**Use it in a workspace:**
```bash
cd /path/to/project
daisy-init sideprojects
```

**Switch an existing workspace to a different home:**
```bash
daisy-init personal
```

Each workspace independently tracks its own home via `.daisy/home`. No global state to manage.

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

See [docs/todotxt.md](docs/todotxt.md) for complete format specification.

## Philosophy

- **Plain text** - Git-friendly, searchable, future-proof
- **AI-native** - Designed for conversational AI assistance
- **Flexible** - Adapt to your workflow, not vice versa
- **Reflective** - Daily retrospectives drive continuous improvement
- **Multi-context** - Multiple homes, multiple workspaces, zero interference

## See Also

- **[Quickstart Guide](docs/quickstart.md)** - Get started in 5 minutes
- **[AI Workflow Guide](prompts/daisy.md)** - User-focused workflows and commands
- **[Admin Guide](AGENTS.md)** - Internal architecture and specifications
- **[Detailed Examples](docs/daisy/examples.md)** - Interaction walkthroughs
- [Todo.txt Specification](docs/todotxt.md) - Format reference
