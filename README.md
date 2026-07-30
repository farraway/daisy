# Daisy - Personal Productivity System

Daisy is a personal productivity system you operate by talking to an AI
agent. Your tasks live in a plain [todo.txt](daisy/docs/todotxt.md) file,
your work log in daily markdown journals, your projects in one markdown file
each — and an agent (Claude Code, Cursor) reads and maintains all of it for
you:

```
/daisy start a new day
/daisy add a task: review the deploy pipeline +infra
/daisy done "deploy pipeline"
/daisy log Shipped the fix after pairing with ~mel
/daisy how's +website going?
```

Every file is plain text in a git repository. Every change the system makes
is a commit. There is no database, no daemon, no app — just files, scripts,
and prompts.

## What You Get

- **A memory.** The journal records what you actually did, day by day, with
  the agent logging proactively as you work. Weeks later, "what did I do
  about X?" has an answer.
- **A trusted task list.** One canonical todo.txt with four priority levels
  — now, next, soon, someday — synced into a daily `today.md` working view.
- **Context that survives.** Project files hold the *why* and *how* — goals,
  decisions, open questions — so neither you nor the agent starts cold. Work
  tickets stay what they should be: communication, not memory.
- **Plans that outlive a session.** For larger changes, a `PLAN.md` records
  research, steps, and decisions; an interrupted session resumes from
  verified state instead of vibes.
- **A system that learns.** Corrections you give the agent are recorded and
  periodically compiled back into the prompts carrying its workflows —
  mistakes get fixed structurally, not re-corrected forever.

## Philosophy

**Plain text, forever.** Tasks, journals, projects, and plans are markdown
and todo.txt in git. Greppable, diffable, portable, and readable in fifty
years. No tool lock-in — delete Daisy and your data remains legible.

**AI-native, not AI-flavored.** Most productivity tools added an assistant
to a GUI. Daisy inverts this: the *agent is the interface*, and the system
is designed for it — file formats with strict, machine-checkable invariants;
workflows as scripts the agent invokes; instructions compiled per-context.
You talk; the agent runs `daisy new-day`, edits todo.txt, commits.

**Scripts for mechanism, instructions for judgment.** Anything that must be
exact (archiving a day, rotating journals, marking tasks done) is a shell
script with tests. Anything that requires judgment (what to log, how to
prioritize, when to ask) lives in the prompts and skills described below.
The agent is never trusted with format-critical mechanics, and scripts are
never asked to make judgment calls.

**Multiple contexts, concurrent use.** A "home" is one context's data —
tasks, journal, projects. One machine can run separate personal and work
homes, and several workspaces can share a home at once. Home resolution is
stateless and every mutation ends in a git commit, so concurrent sessions
interleave in the log instead of fighting over state. See
[daisy/docs/home-management.md](daisy/docs/home-management.md).

**A feedback loop, not a rulebook.** When the agent gets something wrong,
`daisy feedback` records it. Periodically, `daisy optimize` has an LLM
rewrite the relevant prompt's rules from the accumulated feedback — trained
on an 80/20 split, shown as a diff, applied only on approval. Those prompts
are a living artifact shaped by how the agent actually fails. The loop
reaches prompts today; extending it to cover skills is outstanding work, and
[daisy/docs/prompts-vs-skills.md](daisy/docs/prompts-vs-skills.md) explains
why that matters. See also
[daisy/docs/resources/prompt-learning-loop.md](daisy/docs/resources/prompt-learning-loop.md).

## Two Layers: Prompts and Skills

Daisy's instructions live in two layers, divided by one question: **does
this need to know your situation, or only the artifact in front of it?**

The **situated layer** is made of *prompts* — instruction files compiled
per-home into the agent's context. They know what your projects are, what
you did yesterday, how you work: `daisy` (tasks, journal, projects),
`plan`, `retrospective`, `github`, `socrates`, and `personal`. They read and
write the files Daisy owns, and they exist only where Daisy is running.

The **craft layer** is made of *skills* — self-contained capabilities the
agent reaches for by description. They know the work, not you: `css`, `js`,
`tdd`, `git`, `agents-md`. Because they assume nothing about Daisy, they
keep working in repositories that have never been initialized — your
conventions travel to every scratch repo and clone, not just the workspaces
you've set up.

Most prompts are also loaded frugally: only a short trigger stub sits in the
agent's context, and the full prompt is read on demand when its trigger
fires. Skills cost nothing until the agent reaches for one.

Where the boundary falls in less obvious cases — hybrids, capabilities that
hand off to a Daisy workflow, material tracking an upstream source — is
worked through in
[daisy/docs/prompts-vs-skills.md](daisy/docs/prompts-vs-skills.md).

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
- Per-tool agent configuration so agents automatically discover Daisy — see
  [Supported AI Tools](#supported-ai-tools)
- `.daisy/` entries in `.gitignore`

### 4. Verify

```bash
daisy healthcheck
```

Confirms `DAISY_ROOT`/`DAISY_HOME` resolve correctly, required files exist,
and every script's own health check passes.

A passing result is cached for 300 seconds, so the workflows that gate on it
(`new-day` and friends) don't re-scan the tree on every invocation. Set
`DAISY_HEALTHCHECK_TTL` to change the window, or to `0` to disable caching.
Run `daisy healthcheck --force` to bypass the cache and re-check now — worth
doing before publishing, since a cached pass reflects the tree as it was
when the check ran, not as it is. A failing check clears the cache
immediately.

### 5. Start Using Daisy

In an initialized workspace, just say:
```
/daisy start a new day
```

The agent will:
- Read `.daisy/AGENTS.md` automatically
- Archive yesterday's work (and rotate `journal.md` if it's grown past 5 entries)
- Create fresh `today.md` with prioritized tasks

## Supported AI Tools

`daisy init` configures every supported tool in the workspace at once:

| Tool | Daisy configuration | Invocation |
|------|--------------------|------------|
| Claude Code | `.claude/commands/daisy.md` (`/daisy` command), `.claude/settings.local.json` (permissions) | `/daisy <request>` |
| Cursor | `.cursor/rules/daisy.md`, `.cursorignore` entries | Address Daisy in chat: `Daisy, <request>` |

Craft-layer skills are installed once per machine, to `~/.claude/skills/`, by
`daisy install` — which is why they keep working in repositories that were
never initialized. A home may additionally define its own skills; those
`daisy init` copies into the workspace's `.claude/skills/`, with a matching
stub rule per skill for Cursor.

The examples in this README use the Claude Code form; in Cursor, replace
`/daisy <request>` with `Daisy, <request>`.

## Usage

In any initialized workspace:
```
/daisy start a new day
/daisy done [task]
/daisy log [message]
/daisy what are my tasks?
```

### CLI Commands

Every command also accepts `--help`/`-h` as its sole argument to print its own usage.

```bash
daisy build [home]            # Rebuild AGENTS.md for a home
daisy check-secrets           # Check which secrets/tokens are configured
daisy clean [-f]              # Remove Daisy from the current workspace
daisy commit "message"        # Stage and commit changes under home/
daisy done "task pattern"     # Mark task complete
daisy eval [<case>]           # List, display, or record an eval case result
daisy feedback "message"      # Record a prompt failure for optimization
daisy files                   # Show resolved real paths for every home file
daisy healthcheck [--force]   # System validation
daisy help                    # Show command list
daisy init work               # Initialize Daisy in a new workspace
daisy init personal           # Switch workspace to a different home
daisy install                 # Set up ~/bin/daisy and shell environment
daisy list                    # List active prompts and installed skills
daisy log Did thing A         # Add log entry (no quoting needed)
daisy new-day                 # Start a new day
daisy new-week                # Start a new week
daisy optimize [--workflow <n>] # Run prompt learning loop on collected feedback
daisy plan-archive            # Archive the active Daisy plan
daisy plan-new "<description>" # Create a new Daisy plan and symlink PLAN.md
daisy plan-pickup <path>      # Promote a spec draft into a tracked plan
daisy projects [--archived]   # List active or archived projects
daisy rotate                  # Rotate journal.md into archive window files (automatic; rarely needed by hand)
daisy status                  # Quick workspace summary
daisy tasks [--all|--done|--todo] "+project"  # Search todo.txt/done.txt (read-only)
daisy test [name-substring]   # Run the hermetic daisy/scripts test suite
```

### Working Practices

Mechanics live in the CLI; these are the design choices behind daily use:

- **Projects over tickets.** A `+tag` in todo.txt captures *what*; a project
  file in `projects/{name}.md` captures *why* and *how*. External trackers
  receive one-way status pushes; your project file remains the source of
  truth for your own understanding. See [daisy/docs/projects.md](daisy/docs/projects.md).
- **Bounded journals.** `new-day`/`new-week` archive `today.md` into
  `journal.md` losslessly; rotation keeps `journal.md` at the last 5 entries
  and distributes the rest into rolling and permanent archive files. See
  [daisy/docs/logging.md](daisy/docs/logging.md).
- **Plan before large changes.** `/daisy plan` scaffolds Research → Goal →
  Steps → Decisions before any implementation; `/daisy execute` works
  through it step by step; `/daisy resume` picks up an interrupted session
  from verified state. See [daisy/docs/plan.md](daisy/docs/plan.md).
- **Concurrency by design.** No locks: stateless home resolution plus
  commit-per-mutation lets sessions interleave. Some sort of lock may
  eventually prove useful, but local usage (even with multiple concurrent
  agent sessions) reveals no need for one at this stage.

### Optional: API Authentication

For API integrations (Webex, JIRA, GitHub) when MCP servers are unavailable,
copy `$DAISY_ROOT/daisy/templates/env.sh.template` to `.env.sh` in your
workspace, fill in tokens, and verify with `daisy healthcheck`. MCP servers
handle authentication automatically; `.env.sh` is only a fallback.

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
│   │                           # healthcheck, common, commit, check-secrets, files,
│   │                           # list, tasks, projects, plan-new, plan-pickup,
│   │                           # plan-archive, test
│   ├── templates/
│   │   ├── journal-day.md      # Daily today.md template
│   │   ├── journal-week.md     # Weekly today.md template (adds retrospective)
│   │   ├── project.md          # Template for project files
│   │   ├── plan.md             # Template for PLAN.md
│   │   ├── env.sh.template     # Environment variables template
│   │   ├── cursor-rule.mdc, cursor-rule-logging.mdc, cursor-rule-plan.mdc
│   │   ├── claude-command.md
│   │   └── home/               # Template for new homes (tasks/, journal/, feedback/, projects/, plans/)
│   └── docs/                   # Detailed reference documentation
│       ├── task-format.md, task-sync.md, todotxt.md, templates.md
│       ├── workflows.md, logging.md, projects.md, home-management.md
│       ├── plan.md, praxis.md, retrospective.md, github.md
│       ├── prompts-vs-skills.md, examples.md, test-cases.md
│       └── resources/          # External-talk summaries informing the design
├── home/
│   ├── {home-name}/            # One directory per home
│   │   ├── AGENTS.md           # Generated prompt (git-ignored; rebuild with `daisy build`)
│   │   ├── include.txt         # Which prompts load into AGENTS.md, and how
│   │   ├── prompts/            # Home-specific prompt augmentations
│   │   ├── journal/            # today.md, journal.md, rotated archive files
│   │   ├── tasks/              # todo.txt, done.txt, alias.txt
│   │   ├── projects/           # One file per active project, plus _archive/
│   │   ├── feedback/           # feedback.md, archive.md
│   │   └── plans/              # PLAN.md history, plus archive/
├── prompts/                    # Situated layer: daisy.md, plan.md, retrospective.md, github.md, ...
│   └── evals/                  # Eval cases for the optimize loop
└── skills/                     # Craft layer: css, js, tdd, git, agents-md (see Two Layers)
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

## See Also

- **[AI Workflow Guide](prompts/daisy.md)** - User-focused workflows and commands
- **[Admin Guide](AGENTS.md)** - Internal architecture and specifications
- **[Prompts vs. Skills](daisy/docs/prompts-vs-skills.md)** - Where the layer boundary falls, and why
- **[Detailed Examples](daisy/docs/examples.md)** - Interaction walkthroughs
- [Todo.txt Specification](daisy/docs/todotxt.md) - Format reference
