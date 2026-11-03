# Daisy - Personal Productivity System

Daisy is a personal productivity system you operate by talking to an AI
agent. Your tasks live in a plain [todo.txt](docs/todotxt.md) file,
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
is a commit. There is no database, no daemon, no app — just files, one CLI,
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
workflows as commands the agent invokes; instructions compiled per-context.
You talk; the agent runs `daisy new-day`, edits todo.txt, commits.

**Commands for mechanism, instructions for judgment.** Anything that must be
exact (archiving a day, rotating journals, marking tasks done) is a `daisy`
command with tests. Anything that requires judgment (what to log, how to
prioritize, when to ask) lives in the prompts and skills described below.
The agent is never trusted with format-critical mechanics, and commands are
never asked to make judgment calls.

**Multiple contexts, concurrent use.** A "home" is one context's data —
tasks, journal, projects — in its own git repository, separate from Daisy's
code. One machine can run separate personal and work homes, several
workspaces can share a home at once, and each workspace's own repository
commits as the identity you bind it with. Home resolution is stateless and
every mutation ends in a git commit in the home's repository, so concurrent
sessions interleave in the log instead of fighting over state. See
[docs/home-management.md](docs/home-management.md).

**A feedback loop, not a rulebook.** When the agent gets something wrong,
`daisy feedback` records it. At New Week, `daisy optimize` briefs the agent
in your session: each prompt or skill with pending feedback, its entries and
its current rules, and every one whose rules have grown past ten, flagged
for a rewrite. The agent proposes new rules (or a rewrite) in the
conversation; `daisy optimize apply` shows the diff, writes what you
approve, archives the entries it addressed and commits — no API key, no
second model. Those
prompts are a living artifact shaped by how the agent actually fails. The
loop reaches both layers — an entry tagged `prompts/<name>` or
`skills/<name>` trains that file, and
[docs/prompts-vs-skills.md](docs/prompts-vs-skills.md) explains
why the distinction matters. See also
[docs/resources/prompt-learning-loop.md](docs/resources/prompt-learning-loop.md).

## Two Layers: Prompts and Skills

Daisy's instructions live in two layers, divided by one question: **does
this need to know your situation, or only the artifact in front of it?**

The **situated layer** is made of *prompts* — instruction files compiled
per-home into the agent's context. They know what your projects are, what
you did yesterday, how you work: `daisy` (tasks, journal, projects),
`plan`, `praxis`, `retrospective`, `github`, and `socrates`. They read and
write the files Daisy owns, and they exist only where Daisy is running.

The **craft layer** is made of *skills* — self-contained capabilities the
agent reaches for by description. They know the work, not you: `agents-md`,
`css`, `git`, `js`, `testing`, `transcribe`. Because they assume nothing
about Daisy, they keep working in repositories that have never been
initialized — your conventions travel to every scratch repo and clone, not
just the workspaces you've set up.

Most prompts are also loaded frugally: only a short trigger stub sits in the
agent's context, and the full prompt is read on demand when its trigger
fires. Skills cost nothing until the agent reaches for one.

Where the boundary falls in less obvious cases — hybrids, capabilities that
hand off to a Daisy workflow, material tracking an upstream source — is
worked through in
[docs/prompts-vs-skills.md](docs/prompts-vs-skills.md).

## Install

### 1. Clone

```bash
git clone <repo-url> daisy
cd daisy
```

### 2. Install

```bash
make build        # the Go binary, bin/daisy (Go 1.27, standard library only)
bin/daisy install
```

This will:
- Create a `~/bin/daisy` symlink to `bin/daisy` (`install` does not build it)
- Install the root skills into `~/.claude/skills/`
- Add `DAISY_ROOT` to your shell rc file (`.zshenv`, `.bashrc`, etc.)
- Configure Claude Code permissions for `daisy` and `$DAISY_ROOT`

Then reload your shell: `source ~/.zshenv` (or open a new terminal), and
create a home if you have none: `daisy home new <name>` makes one, its own
git repository, by default at `~/.local/daisy/homes/<name>`, and registers it
in `~/.config/daisy/homes`.

### 3. Initialize Daisy in a Workspace

```bash
cd /path/to/your/project
daisy init work
```

`work` is an identity: a registered home's name (its default identity, from
the home's `gitconfig`), or the name of an `identities/<id>` file in exactly
one registered home. The workspace binds to that home, and the workspace's
own repository commits as that identity. This creates:
- `.daisy/` directory with symlinks to that home's tasks, journal, projects,
  plans and feedback
- The identity's `user.name`/`user.email` (and SSH key, if it has one) in the
  workspace repository's local git config
- Per-tool agent configuration so agents automatically discover Daisy — see
  [Supported AI Tools](#supported-ai-tools)
- `.daisy/` entries in `.gitignore`

### 4. Verify

```bash
daisy healthcheck
```

Confirms `DAISY_ROOT` is set, the home resolves through the registry and
is in a git repository, required files exist, no published file carries an
identifying term, and every command's own health check passes.

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
| Cursor | `.cursor/rules/daisy.mdc` (plus `daisy-logging.mdc`, `daisy-plan.mdc`), `.cursorignore` entries | Address Daisy in chat: `Daisy, <request>` |

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
daisy check-secrets           # Check which secrets/tokens are configured
daisy clean [-f]              # Remove Daisy from the current workspace
daisy commit "msg"            # Commit the home's repository (--root "msg": the root's)
daisy done "task pattern"     # Mark task complete
daisy eval [<case>] [--record] # List, display, or record an eval case result
daisy feedback "message"      # Record a prompt failure for optimization
daisy files                   # Show resolved real paths for every home file
daisy healthcheck [--force]   # System validation
daisy help [<command>|<skill>] # Show command list, one command's usage, or a skill's detail
daisy help --markdown         # Print the Command Reference block for AGENTS.md
daisy home list               # Registered homes and where each lives
daisy home new <name> [path]  # Create a home (own git repo) and register it
daisy home add <name> <path>  # Register an existing home; `remove` unregisters
daisy identity list           # Identities across the registered homes
daisy init <identity>         # Bind the workspace to an identity and its home
daisy init personal           # Re-run to switch home or identity
daisy install [--update]      # Set up ~/bin/daisy, root skills, permissions, shell rc
daisy list                    # List active prompts and installed skills
daisy log Did thing A         # Add log entry (no quoting needed)
daisy new-day                 # Start a new day
daisy new-week                # Start a new week
daisy optimize [--workflow <n>] # Brief the agent: pending feedback per target, rewrite flags
daisy optimize apply --workflow <n> --rules <file> # Apply an approved proposal (or --rewrite <file>)
daisy plan archive            # Archive the active Daisy plan
daisy plan new "<description>" # Create a new Daisy plan and symlink PLAN.md
daisy plan pickup <path>      # Promote a spec draft into a tracked plan
daisy projects [--archived]   # List active or archived projects
daisy prompt build [<home>]   # Rebuild AGENTS.md (a home's, or this workspace's)
daisy rotate                  # Rotate journal.md into archive window files (automatic; rarely needed by hand)
daisy status                  # Quick workspace summary
daisy tasks [--all] [--todo|--done] [pattern]  # Search this workspace's tasks; --all widens to every project (read-only)
daisy test [filter]           # Build, run go test, then the hermetic acceptance suite
```

### Working Practices

Mechanics live in the CLI; these are the design choices behind daily use:

- **Projects over tickets.** A `+tag` in todo.txt captures *what*; a project
  file in `projects/{name}.md` captures *why* and *how*. External trackers
  receive one-way status pushes; your project file remains the source of
  truth for your own understanding. See [docs/projects.md](docs/projects.md).
- **Bounded journals.** `new-day`/`new-week` archive `today.md` into
  `journal.md` losslessly; rotation keeps `journal.md` at the last 5 entries
  and distributes the rest into rolling and permanent archive files. See
  [docs/logging.md](docs/logging.md).
- **Plan before large changes.** `/daisy plan` scaffolds Research → Goal →
  Steps → Decisions before any implementation; `/daisy execute` works
  through it one fresh sub-agent per step, each reviewed and verified
  before the next; `/daisy resume` picks up an interrupted session
  from verified state. See [docs/plan.md](docs/plan.md).
- **Concurrency by design.** No locks: stateless home resolution plus
  commit-per-mutation lets sessions interleave. Some sort of lock may
  eventually prove useful, but local usage (even with multiple concurrent
  agent sessions) reveals no need for one at this stage.

### Optional: API Authentication

For API integrations (Webex, JIRA, GitHub) when MCP servers are unavailable,
copy `$DAISY_ROOT/templates/env.sh.template` to `.env.sh` in your
workspace, fill in tokens, and verify with `daisy check-secrets`. MCP servers
handle authentication automatically; `.env.sh` is only a fallback.

## File Structure

Daisy's code and your data are separate git repositories: the root holds
the code and is published as it is; each home holds one context's data and
lives wherever you put it. The root finds homes by name through a registry.

### Daisy Repo

```
daisy/                          # Repository root ($DAISY_ROOT)
├── AGENTS.md                   # System architecture & internal specs (auto-applied by Cursor/Claude Code)
├── README.md                   # This file
├── cmd/daisy/                  # main.go, the binary: bin/daisy (symlinked to ~/bin/daisy); `make build`; the functional tests
├── internal/                   # The Go packages behind it (ops, env, cli, commands, ...)
├── Makefile                    # build, check (gofmt, vet, tidy, go test -race), test (`daisy test`)
├── images/member/Dockerfile    # The swarm's member image (docs/swarm.md; built at Step 28)
├── templates/
│   ├── journal-day.md          # Daily today.md template
│   ├── journal-week.md         # Weekly today.md template (adds retrospective)
│   ├── project.md              # Template for project files
│   ├── plan.md                 # Template for PLAN.md
│   ├── env.sh.template         # Environment variables template
│   ├── cursor-rule.mdc, cursor-rule-logging.mdc, cursor-rule-plan.mdc
│   ├── claude-command.md
│   └── home/                   # Template for new homes (copied by `daisy home new`)
├── docs/                       # Detailed reference documentation
│   ├── cli.md                  # The `daisy` CLI contract
│   ├── swarm.md                # The swarm's specification
│   ├── task-format.md, task-sync.md, todotxt.md, templates.md
│   ├── workflows.md, logging.md, projects.md, home-management.md
│   ├── plan.md, praxis.md, retrospective.md, github.md
│   ├── prompts-vs-skills.md, examples.md, test-cases.md, testing.md
│   └── resources/              # External-talk summaries informing the design
├── tests/                      # The acceptance suite (`daisy test`): run.sh, lib/, cases/
├── prompts/                    # Situated layer: daisy.md, plan.md, retrospective.md, github.md, ...
│   └── evals/                  # Eval cases for the feedback loop (`daisy eval`)
└── skills/                     # Craft layer: agents-md, css, git, js, testing, transcribe (see Two Layers)
```

### Per-Machine State and Homes

```
~/.config/daisy/                # $DAISY_CONFIG
└── homes                       # The registry: name=/absolute/path, one home per line
~/.local/daisy/                 # $DAISY_LOCAL
└── homes/{home-name}/          # Where `daisy home new` puts a home by default
```

A home is one git repository, created from `templates/home/`:

```
{home}/
├── .gitignore                  # AGENTS.md and the healthcheck stamp
├── AGENTS.md                   # Generated prompt (rebuild with `daisy prompt build`)
├── include.txt                 # Which prompts load into AGENTS.md, and how
├── gitconfig                   # The home's default identity: name=, email=, optional sshkey=
├── identities/                 # Optional: further identities of this home, one file each (same format)
├── prompts/                    # Optional, yours to add: home prompts shadow the root's by name
├── skills/                     # Home-specific skills, installed into each workspace
├── templates/                  # Optional journal-day.md, journal-week.md: used in place of the root's
├── journal/                    # today.md, journal.md, rotated archive files
├── tasks/                      # todo.txt, done.txt, alias.txt
├── projects/                   # One file per active project, plus _archive/
├── feedback/                   # feedback.md, archive.md
└── plans/                      # PLAN.md history, plus archive/
```

### Workspace Layout (after `daisy init work`)

```
workspace/
├── .daisy/
│   ├── home                 # "work" (plain text): the home's registered name
│   ├── identity             # only when the identity is not the home's default
│   ├── gitconfig            # the identity init applied; clean unsets against it
│   ├── AGENTS.md            # generated prompt for this home
│   ├── tasks/                → {home}/tasks/
│   ├── today.md              → {home}/journal/today.md
│   ├── journal.md            → {home}/journal/journal.md
│   ├── projects/             → {home}/projects/
│   ├── plans/                → {home}/plans/
│   └── feedback.md           → {home}/feedback/feedback.md
├── .cursor/rules/
│   └── daisy.mdc, daisy-logging.mdc, daisy-plan.mdc
└── PLAN.md                   → .daisy/plans/{current}.plan.md (only while a plan is active)
```

Run `daisy files` any time to print every one of these paths, fully resolved, for the active home and identity.

Different workspaces can use different homes, or the same home under
different identities, concurrently.

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

## See Also

- **[AI Workflow Guide](prompts/daisy.md)** - User-focused workflows and commands
- **[Admin Guide](AGENTS.md)** - Internal architecture and specifications
- **[Prompts vs. Skills](docs/prompts-vs-skills.md)** - Where the layer boundary falls, and why
- **[Detailed Examples](docs/examples.md)** - Interaction walkthroughs
- **[Testing](docs/testing.md)** - The `testing` skill's long form: vocabulary, literature, the design worked through
- [Todo.txt Specification](docs/todotxt.md) - Format reference
