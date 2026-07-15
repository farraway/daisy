# Daisy - Personal Productivity System

A lightweight productivity system combining todo.txt task management with daily markdown journaling, designed for AI-assisted workflows and concurrent use across sessions.

## Install

### 1. Clone or Download

```bash
git clone <repo-url> daisy
cd daisy
```

### 2. Install

```bash
./daisy.sh install
```

This will:
- Create a `~/bin/daisy` symlink
- Add `DAISY_ROOT` and `DAISY_DEFAULT_HOME` to your shell rc file (`.zshenv`, `.bashrc`, etc.)
- Interactively select a default home (or create one)
- Configure Claude Code permissions for `daisy` and `$DAISY_ROOT`

Then reload your shell: `source ~/.zshenv` (or open a new terminal).

### 3. Initialize Daisy in a Workspace

```bash
cd /path/to/your/project
daisy init work
```

This creates:
- `.daisy/` directory with symlinks to that home's tasks, journal, and projects
- `.cursor/rules/daisy.md` so agents automatically discover daisy
- `.daisy/` entries in `.gitignore`

### 4. Verify

```bash
daisy healthcheck
```

Confirms `DAISY_ROOT`/`DAISY_HOME` resolve correctly, required files exist, and every script's own health check passes. Run `daisy healthcheck --force` any time to bypass the per-session cache.

### 5. Start Using Daisy

With the cursor rule installed, just say:
```
Daisy, start a new day
```

The agent will:
- Read `.daisy/AGENTS.md` automatically
- Archive yesterday's work (and rotate `journal.md` if it's grown past 5 entries)
- Create fresh `today.md` with prioritized tasks

---

## Usage

**Already set up?** Quick reference for daily use.

Just address Daisy by name in any initialized workspace:
```
Daisy, start a new day
Daisy, done [task]
Daisy, log [message]
Daisy, what are my tasks?
```

### CLI Commands

Every command also accepts `--help`/`-h` as its sole argument to print its own usage.

```bash
daisy build [home]           # Rebuild AGENTS.md for a home
daisy check-secrets           # Check which secrets/tokens are configured
daisy clean [-f]              # Remove Daisy from the current workspace
daisy done "task pattern"     # Mark task complete
daisy eval [<case>]           # List, display, or record an eval case result
daisy feedback "message"      # Record a prompt failure for optimization
daisy files                   # Show resolved real paths for every home file
daisy healthcheck [--force]   # System validation
daisy help                    # Show command list
daisy init work                # Initialize Daisy in a new workspace
daisy init quiteclose          # Switch workspace to a different home
daisy install                 # Set up ~/bin/daisy and shell environment
daisy log Did thing A          # Add log entry (no quoting needed)
daisy new-day                 # Start a new day
daisy new-week                # Start a new week
daisy optimize [--workflow <n>] # Run prompt learning loop on collected feedback
daisy plan-archive             # Archive the active Daisy plan
daisy plan-new "<description>" # Create a new Daisy plan and symlink PLAN.md
daisy projects [--archived]   # List active or archived projects
daisy rotate                  # Rotate journal.md into archive window files (automatic; rarely needed by hand)
daisy status                  # Quick workspace summary
```

### Optional: API Authentication

For API integrations (Webex, JIRA, GitHub) when MCP servers are unavailable:

1. Copy the template to your workspace:
   ```bash
   cp $DAISY_ROOT/daisy/templates/env.sh.template .env.sh
   ```

2. Edit `.env.sh` and fill in your tokens

3. Verify configuration:
   ```bash
   daisy healthcheck
   ```

**Note:** MCP servers handle authentication automatically. `.env.sh` is only needed as a fallback.

## Features & Rationale

Mechanics for each of these are in Usage above; this section is the "why," each pointing to the doc with full detail rather than repeating it.

### Todo.txt tasks

Plain-text, priority-tagged tasks (`(A)`–`(D)`, or untagged for the inbox) in `tasks/todo.txt`. Plain text over a database or app because it's git-friendly, greppable, and outlives any particular tool. See [daisy/docs/todotxt.md](daisy/docs/todotxt.md) for the full format.

### Homes

A "home" is one person's or context's task/journal/project data (`home/{name}/`), resolved per-workspace via `.daisy/home` rather than a single global setting. This is what lets one machine run separate personal and work contexts — or several workspaces on the same context — without them interfering. Each workspace independently tracks its own home; there's no global state to manage. Create a new one and activate it in the current workspace with `daisy init --new sideprojects`; switch an existing workspace to a different home with `daisy init <home>` (also in the CLI Commands table above). See [daisy/docs/home-management.md](daisy/docs/home-management.md).

### Projects

A `+tag` in todo.txt captures *what* to do; a project file in `projects/{name}.md` captures *why* and *how* — goals, resources, decisions, open questions. Without that file, this context tends to migrate into JIRA, which turns JIRA into the management tool instead of the communication tool it should be. The project file is the source of truth for your own understanding; JIRA is the source of truth for the company's. See [daisy/docs/projects.md](daisy/docs/projects.md).

### Journaling & rotation

`today.md` is the day's working log; `new-day`/`new-week` archive it into `journal.md` losslessly. Left unbounded, `journal.md` grows forever (it reached 3225 lines / 50 day-blocks before rotation shipped) — `daisy rotate` runs automatically as part of `new-day`/`new-week` now, so `journal.md` always holds just the last 5 entries, with everything else distributed into a last-14 rolling file, month-to-date/year-to-date rolling files, and permanent closed archives (`journal-YYYYMM.md`, `journal-YYYY.md`) once a period completes. No state file tracks progress — whatever's beyond the retained last 5 *is* the work queue, which is what lets a first-run backfill and an ordinary single-day rotation use the exact same code path. See [daisy/docs/logging.md](daisy/docs/logging.md) for the abridging/archival design rationale.

### Plan workflow

For cross-file or higher-risk work, `/daisy plan` scaffolds a `PLAN.md` (Research → Goal → Steps → Risks → Decisions) before any implementation begins, and `/daisy execute` works through it step by step, checking each off and recording deviations as it goes. This exists so a large change gets researched and agreed before code is written, and so a session that gets interrupted can resume from verified state rather than from memory. See [daisy/docs/plan.md](daisy/docs/plan.md).

### Feedback / optimize loop

`daisy feedback` records a behavioral correction; `daisy optimize` periodically has an LLM rewrite the relevant prompt's `## Rules` section from accumulated feedback, on an 80/20 train/test split, with a diff shown before anything is applied. Once a run applies, the entries it used move out of `feedback.md` into `feedback/archive.md` — so `feedback.md` always contains only what's still unprocessed, with no cutoff timestamp to cross-reference. This loop is scoped to *agent behavioral* corrections only; requests that need actual script/tool changes are triaged out to a project + `todo.txt` task instead, since a Rules-section rewrite can't build a new command. See [daisy/docs/resources/prompt-learning-loop.md](daisy/docs/resources/prompt-learning-loop.md).

### Concurrency model

Daisy is designed to be used from multiple sessions/workspaces sharing a home at once. Nothing coordinates this with a lock — instead, home resolution is fully stateless (recomputed from `.daisy/home` on every invocation, no daemon, no shared process state), and every mutation ends in a git commit, so commits from concurrent sessions simply interleave in the log rather than needing a merge. Most mutations append or fully regenerate a file from source-of-truth rather than patching, which tolerates that interleaving well. The real limit: there's no protection against two processes writing the *same* file at the *same* instant — low-probability for how one person actually works across a few sessions, but a real gap, not a guarantee.

## File Structure

### Daisy Repo

```
daisy/                          # Repository root ($DAISY_ROOT)
├── AGENTS.md                   # System architecture & internal specs (auto-applied by Cursor/Claude Code)
├── README.md                   # This file
├── daisy.sh                    # CLI entry point (symlinked to ~/bin/daisy)
├── daisy/                      # Distributable system files
│   ├── scripts/                # daisy-init, new-day, new-week, rotate, done, log,
│   │                           # feedback, optimize, eval, build-prompt, create-home,
│   │                           # healthcheck, common, commit, check-secrets,
│   │                           # files, projects, plan-new, plan-archive
│   ├── templates/
│   │   ├── journal-day.md      # Daily today.md template
│   │   ├── journal-week.md     # Weekly today.md template (adds retrospective)
│   │   ├── project.md          # Template for project files
│   │   ├── plan.md             # Template for PLAN.md
│   │   ├── env.sh.template     # Environment variables template
│   │   ├── cursor-rule.mdc, cursor-rule-logging.mdc, cursor-rule-plan.mdc
│   │   ├── claude-command.md
│   │   └── home/                # Template for new homes (tasks/, journal/, feedback/, projects/, plans/)
│   └── docs/                   # Detailed reference documentation
│       ├── task-format.md, task-sync.md, todotxt.md, templates.md
│       ├── workflows.md, logging.md, projects.md, home-management.md
│       ├── plan.md, praxis.md, retrospective.md, github.md
│       ├── examples.md, test-cases.md
│       └── resources/          # External-talk summaries informing the design
├── home/
│   ├── {home-name}/             # One directory per home
│   │   ├── AGENTS.md            # Generated prompt (git-ignored; rebuild with `daisy build`)
│   │   ├── include.txt          # Which prompts load into AGENTS.md, and how
│   │   ├── prompts/             # Home-specific prompt augmentations
│   │   ├── journal/              # today.md, journal.md, rotated archive files
│   │   ├── tasks/                # todo.txt, done.txt, alias.txt
│   │   ├── projects/             # One file per active project, plus _archive/
│   │   ├── feedback/             # feedback.md, archive.md
│   │   └── plans/                # PLAN.md history, plus archive/
└── prompts/                     # Shared prompts (daisy.md, plan.md, retrospective.md, github.md, ...)
    └── evals/                   # Eval cases for the optimize loop
```

### Workspace Layout (after `daisy init work`)

```
workspace/
├── .daisy/
│   ├── home                 # "work" (plain text)
│   ├── AGENTS.md            # generated prompt for this home
│   ├── tasks/                → daisy/home/work/tasks/
│   ├── today.md              → daisy/home/work/journal/today.md
│   ├── journal.md            → daisy/home/work/journal/journal.md
│   ├── projects/             → daisy/home/work/projects/
│   └── plans/                → daisy/home/work/plans/
├── .cursor/rules/
│   └── daisy.md
└── PLAN.md                   → .daisy/plans/{current}.plan.md (only while a plan is active)
```

Run `daisy files` any time to print every one of these paths, fully resolved, for the active home.

Different workspaces can use different homes concurrently.

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

See [daisy/docs/todotxt.md](daisy/docs/todotxt.md) for complete format specification.

## Philosophy

- **Plain text** - Git-friendly, searchable, future-proof
- **AI-native** - Designed for conversational AI assistance
- **Flexible** - Adapt to your workflow, not vice versa
- **Reflective** - Daily retrospectives drive continuous improvement
- **Multi-context** - Multiple homes, multiple workspaces, designed for concurrent use, not just tolerating it

## See Also

- **[AI Workflow Guide](prompts/daisy.md)** - User-focused workflows and commands
- **[Admin Guide](AGENTS.md)** - Internal architecture and specifications
- **[Detailed Examples](daisy/docs/examples.md)** - Interaction walkthroughs
- [Todo.txt Specification](daisy/docs/todotxt.md) - Format reference
