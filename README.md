# Daisy

Daisy is a productivity system for one person who works with coding
agents. Tasks, a work log and project notes live in plain text files under
git; an agent (Claude Code, Cursor or Codex) reads and maintains them
through one command-line tool, `daisy`, and a set of instruction files that
tell it how. On the same tool sits a second thing, built in 2026-11: a
swarm, which turns a written goal into issues, plans and pull requests on a
forge (a Gitea server holding issues and pull requests), with a person
deciding at its gates by default.

This document says what Daisy is for, what it does, why it is shaped the
way it is, how it is used, and whether to adopt it or take the ideas.

## Goals

Daisy uses four words in fixed senses. Each is defined here before anything
depends on it.

- A **root** is the Daisy repository itself: the Go source, the prompts,
  the skills, the templates and the documentation. It is one git
  repository, installed at `$DAISY_ROOT`, and it is public as it is: every
  file git would keep in it is published when it is pushed.
- A **home** is one context's data: tasks, journal, projects, plans and
  feedback, with the configuration that shapes the agent for that context.
  A home is its own git repository and lives wherever you put it; the root
  finds it by name through a registry file, never by a path derived from
  the root.
- A **workspace** is a directory you work in, usually a project's own
  repository, that has been bound to a home. The binding is a `.daisy/`
  directory holding the home's name, links into the home's files and the
  agent's instructions, generated for that home.
- An **identity** is the git author a workspace's own commits carry: a
  name, an email and, optionally, the SSH key to push with. An identity is
  defined in a home, so naming one when binding a workspace chooses two
  things at once: whose author the workspace's commits are, and which
  home's tasks and journal it sees.

With those four words, the goals:

1. **A record that outlives the session.** What you did, decided and left
   open is written down as you work, by the agent, in files you can read
   without any tool: a journal of days, a task list, one file per project,
   a plan per larger change. Weeks later the question "what happened with
   X?" has an answer in the home.
2. **Mechanism in commands, judgment in instructions.** Anything that must
   be exact (archiving a day, rotating the journal, marking a task done,
   committing) is a `daisy` command with tests. Anything that needs
   judgment (what to log, how to prioritise, when to ask) is written in
   prompts and skills the agent reads. The agent is not asked to do by
   hand what a command does exactly, and a command is not asked to decide.
3. **Several contexts on one machine, concurrently.** A personal home and a
   work home with separate histories; several workspaces sharing one home
   at once; a workspace committing as a different author from the home it
   reads. No daemon for the home and no lock: each command that changes
   the home ends in one commit in its repository (a few leave their write
   for the next commit), so sessions interleave in the log.
4. **Instructions that learn from their failures.** A correction given to
   the agent is recorded in the home and, periodically, compiled into the
   prompt or skill that failed, as a rule or a rewrite the user approves,
   so the same mistake is fixed in the instructions rather than corrected
   again by hand.
5. **A public root with nothing personal in it.** The code is published as
   it is, so personal data lives only in homes, and a check before every
   root commit scans for identifying terms derived from the machine.
6. **Work delegated to agents without losing the thread.** A plan is
   executed one step per fresh agent, each step reviewed by another agent
   to a stated gate (a fixed list of what counts as a finding) with a cap
   on passes; a swarm takes a brief (a written goal, defined below) through
   issues, plans and pull requests on a forge, with the person who runs
   it, the maintainer, deciding at the gates by default, and every exchange
   on the forge where it can be read later.

Non-goals: a graphical application; a database; a hosted service; a swarm
that merges its own work by default; an agent loop of Daisy's own (the
swarm's members are the vendors' agent CLIs, and only the pipeline around
them is Daisy's); a sync with external task trackers beyond what the agent
does by instruction.

## Features

What the binary and the home layout do, each traceable to a command or a
file. `daisy help` lists every command; `docs/cli.md` is the contract
behind each one.

**The home's files.** A home is created from `templates/home/` by
`daisy home new <name>` and registered in `~/.config/daisy/homes`:

```
{home}/
├── gitconfig          # the home's default identity: name=, email=, optional sshkey=
├── identities/        # optional: further identities of this home, one file each, same format
├── include.txt        # which prompts load into the generated AGENTS.md, and how
├── prompts/           # optional: the home's own prompts; one at a root prompt's name overrides it
├── skills/            # the home's own skills, installed into each workspace bound to it
├── templates/         # optional journal-day.md, journal-week.md used in place of the root's
├── tasks/             # todo.txt, done.txt, alias.txt
├── journal/           # today.md, journal.md and the rotated window files
├── projects/          # one file per active project, and an _archive directory
├── plans/             # the plan files, and an archive directory
└── feedback/          # feedback.md, archive.md
```

**Tasks.** `tasks/todo.txt` is the one task list, in
[todo.txt](docs/todotxt.md) form with four priorities: `(A)` now, `(B)`
next, `(C)` soon, `(D)` someday; a task without one is inbox, to be
prioritised. `daisy done
<pattern>` completes a task in both `todo.txt` and the day's `today.md`;
`daisy tasks` searches the workspace's projects' tasks (`--all` widens to
every project, `--todo` or `--done` selects a file). Adding a task, changing
a priority and cancelling are done by the agent, by instruction.

**The day and the week.** `daisy new-day` archives yesterday's `today.md`
into `journal.md` and writes a fresh `today.md` from the day template with
the current tasks; `daisy new-week` does the same with the week template,
which adds a retrospective, and moves completed tasks to `done.txt`.
`daisy log <message>` appends a timestamped line to today's log. `daisy
rotate` keeps `journal.md` at its last five days and moves the rest into
dated window files, each with a table of contents and a summary; `new-day`
and `new-week` run it when needed.

**Projects.** One file per project in `projects/`, created from
`templates/project.md`, holding the why and the how: goals, decisions,
open questions, deferred work. A `+tag` on a task names its project.
`daisy projects` lists the active ones (`--archived` the archived), and
`daisy status` reports the workspace's home, identity, projects, today's
counts and the active plan.

**Plans.** `daisy plan new <description>` creates a plan file in the home
from `templates/plan.md` and links it as `PLAN.md` in the workspace;
`--spec` writes a draft in the workspace instead, outside the home, which
`daisy plan pickup <path>` later copies into the home as a plan; `daisy
plan archive` archives the active plan. The plan
workflow the agent follows (research, steps, one fresh agent per step,
review to a gate, a cap of five passes) is `prompts/plan.md`; its
reference is `docs/plan.md`.

**Homes and identities.** `daisy home new`, `daisy home add`, `daisy home
list` and `daisy home remove` manage the registry; `daisy identity list`
prints every identity on the machine and its home. `daisy init <identity>`
binds the workspace you stand in; `daisy clean` unbinds it, putting the
git configuration back as it was. `daisy files` prints every home path the
workspace resolves to.

**Prompts and skills.** `daisy prompt build` rebuilds a home's generated
`AGENTS.md` (or the bound workspace's `.daisy/AGENTS.md`) from
`include.txt`; `daisy list` shows the active prompts and installed skills;
`daisy help <skill>` prints a skill's description and rules. `daisy install` links
`~/bin/daisy`, installs the root's skills into `~/.claude/skills/`, adds
`DAISY_ROOT` to the shell rc file and grants Claude Code the permissions
`daisy` needs. The two layers and why they differ are the section
[Prompts vs. Skills](#prompts-vs-skills) below.

**The feedback loop.** `daisy feedback <text>` records a failure against a
prompt or skill in the home's `feedback/feedback.md`. `daisy optimize`
briefs the agent in your session: each prompt or skill with pending entries, its
entries, its current rules, and whether its rules have grown past ten and
want a rewrite. The agent proposes; `daisy optimize apply` shows the diff,
writes what you approve, archives the entries it addressed and commits.
`daisy eval` lists, shows or records the eval cases (test prompts for the
workflows) under `prompts/evals/`.

**The swarm.** `daisy swarm init` declares a swarm in a workspace: it
writes `<ws>/swarm/swarm.json` and a prompt per role under
`<ws>/swarm/prompts/`,
creates the swarm's labels on the forge (the nine stages, grouped and
numbered as the job gets on, `swarm:proposal-posted` to
`swarm:review-3-changes-requested`; the flags `swarm:gated` on what
waits for you, `swarm:blocked`, `swarm:paused`, `swarm:blocking` and
`swarm:class-architecture`; and the terminals `swarm:done` and `swarm:rejected`) and prints a systemd user
service. `daisy swarm apply --once` runs one pass of the
reconciler: it reads the forge, launches the members (one agent run each,
in its own container) whose turn it is, posts what they wrote and moves
the labels, then waits for its members; `apply` without `--once`, what
the service runs, loops itself until signalled, a pass every interval
whatever runs, each run collected by the pass after it ends. `daisy swarm status`
reports the stages, the gates, what waits for you, the running members,
throughput and the briefs, and writes nothing. `daisy swarm pause` and
`daisy swarm resume` hold and release launches, for the whole swarm or,
given an issue's number, for that issue alone. The eight verbs of the
maintainer, the person who runs the swarm, `triage`, `accept`, `class`
(a job routine or architecture), `approve`, `merge`, `unblock`, `block`
(hold one job until another issue is closed) and `close`, make your
decisions on the forge under a token of your own. The contract is `docs/swarm.md`; the agent's guide
over the verbs is the `swarm` skill (`skills/swarm/SKILL.md`).

**System.** `daisy commit <message>` commits the home's repository (or the
root's with `--root`); `daisy healthcheck` checks the installation, the
home, the day's file, each command's own health and the root's publication
hygiene; `daisy check-secrets` reports which tokens a workspace's `.env.sh`
sets, reading the file without running it; `daisy test` builds the binary,
runs `go test`, then drives the acceptance suite in `tests/` through it.

## Design decisions & intent

Each decision with its reason and what it costs. Most of them are young:
the split and the port date from the start of 2026-11 and the swarm from
its first week; lazy loading is the old one, from early 2026. The swarm is
the one bet, with a single live run behind it, and is marked as such.

**The root and the homes are separate repositories.** Before 2026-11 the
code and every home lived in one repository, published through a copying
step that stripped the homes out. The data is personal and the code is
public, and a home's history should be one person's, so each home became
its own repository and the root publishes as it is. The cost is a registry
(`~/.config/daisy/homes`, one `name=/path` line per home) and a binding
step per workspace; nothing in the root may derive a home's path from
`$DAISY_ROOT`, and the health check refuses a home that sits inside the
root's repository.

**An identity is a git author, chosen per workspace, defined in a home.**
One person has several personas: a personal account and a work account,
say, both reading the same tasks. So a home holds a default identity
(`gitconfig`) and may hold more (`identities/<id>`), and `daisy init` names
one. The identity sets `user.name`, `user.email` and, with a key,
`core.sshCommand` in the workspace's repository, and `daisy clean` puts
back what was there. It does not change who commits in the home: Daisy's
own commits there carry the home repository's configuration. The cost is
that an identity is only an author: a prompt or skill overlay per identity
was considered and set aside.

**The root is public as it is, and the health check is the gate.** There
is no mirror and no sanitising pass, so any file the root would commit is
public. `daisy healthcheck` scans every publishable file for terms derived
from the machine at run time, never from a list in the root (which would
itself leak): every registered home's name, the OS user, the git author
(`user.name`, `user.email`) configured in the root's repository and in
each home's, and `$HOME/` paths. It runs before every root
commit. The trade-off is stated in the root's `AGENTS.md`: the scan is a
net, not the standard. It cannot catch personal content that contains no
known term, so system files are written as if already public.

**The command line is a contract.** `docs/cli.md` pins each command's
arguments, exit codes (0 success, 1 refused, 2 usage), the strings the
acceptance suite asserts, what it writes and what it commits. The suite in
`tests/cases/` is black-box: it drives the binary and asserts on output,
files and commit counts, so it pinned the first implementation, in bash,
and now pins the Go (next decision). The
cost is a document to keep true; a Go test checks that every command the
documents name is a row of the command table and that every path they name
exists.

**Every side effect goes through one value.** `internal/ops` is the `Ops`
interface: the file system, processes, the clock, environment, HTTP and the
approval prompt. `Real` does them; `ReadOnly` refuses every write, so a
command declared read-only (`status`, `files`, `tasks`, `swarm status`)
cannot write by accident; `Fake` lets a test build its own machine, with a
temporary root, home, registry and workspace, and pin what git reads. The
rule is `cli → commands → domain packages → ops`, and nothing above `ops`
touches the file system, starts a process or reads the clock itself. The
cost is that every new side effect is added in one place first.

**Bash to Go.** The first Daisy was 5,500 lines of bash across 25 scripts,
requiring bash 4, jq, curl and GNU tools (so a second bash on macOS). The
port, in 2026-11, made one binary built from Go 1.27's standard library
alone (at run time it needs git, and the swarm needs Docker and the
vendors' CLIs), functional tests that call the program's `Run` directly,
and the acceptance suite as the oracle: where the suite pins the bash's
output and files, the Go matches them byte for byte. The cost is a build
step, `make build`, before `daisy install`.

**The feedback loop runs in the agent's session and writes only the
home.** The bash `optimize` called a model API from a script with its own
key and a two-entry floor standing in for judgment. The redesign has the
binary brief the agent that is already in the session and apply what the
user approves, so there is no second model and no key. The loop writes a
home override, never a root file: a *layered* override holds only the
home's added rules, appended to the root's at build time so upstream
changes keep flowing; a *full* override is a copy, and the health check
says when the root has moved on. The cost is that a root prompt changes
only by the developer's hand, and a full override can fall behind.

**Prompts load lazily.** Most prompts sit in the agent's context as a
short trigger stub, the full text read when the trigger fires; a skill
costs its one-line description until the agent reaches for one. The reason
is context: the agent
should carry what the request needs, not everything Daisy knows. The cost
is one more rule for the author (a `## Trigger` section first).

**The swarm is a reconciler over a forge** (a bet: designed through
2026-11-06, built 2026-11-06 and 07, one product so far). The forge
(Gitea; a GitHub adapter is specified and waits for a run that needs it)
is the interface and the memory: an issue is a job, its
`swarm:*` label is its stage, the issue thread is the design conversation
and each pull request the implementation conversation. `swarm apply` reads
the forge, launches the members whose turn it is, posts what they wrote and
moves the label; the members are the vendors' own agent CLIs in fresh
containers with no memory, given their thread so far. Its parts, each
chosen for a reason:

- *The brief is the input.* What the maintainer gives a swarm is not a plan
  with steps but a brief: a goal with milestones, each with a persona, an
  exit criterion observable without reading code and the earlier work it
  forces, with a `Status` the maintainer flips. Four levels follow:
  brief, job (an issue, proposed by the owner toward the first unmet
  milestone), plan (the architect's, in the issue thread, a Daisy plan
  whose steps are slices) and slice (one pull request). Steps are the
  agents' to supply; a Daisy plan in the home is the maintainer's own
  instrument and is never given to the swarm.
- *Five roles, one from another vendor.* Owner, architect, worker,
  adversary and sympathetic reviewer, each a persona prompt the maintainer
  may edit plus a Task text Daisy fixes, so an edited persona cannot break
  the output contract. The adversary runs on a different vendor's model so
  that the two reviews do not come from one model. In the first live run
  the adversary found one real defect by reading the code, which the
  product's own test corpus had passed; one run does not show that the
  vendor difference was the cause.
- *Gates with the maintainer.* Triage, plan acceptance, merge and unblock
  are the maintainer's decisions by default; each of the first three can
  be handed to the swarm, which then requires a spending limit. Triage
  and plan acceptance hold only an architecture job: one the owner
  classes routine (cut from a brief's milestone, at most two slices, no
  public API or protected path changed) passes them by itself and waits only at the merge.
  A swarm that merges by default is a non-goal.
- *The swarm has its own forge account, and every entry it posts opens
  with a marker*, a fixed first line naming the role and the run. So the
  maintainer's words and the swarm's are told apart by authorship, and a
  member's text cannot pass as the maintainer's. The maintainer's writes
  go through eight verbs under a separate token, read by the verbs only and
  never by `init`, `apply` or `status`; a verb refuses when its token is
  the swarm's.
- *Mount only what the run owns.* A member reads untrusted text, so it
  gets one credential, its clone, the directories `swarm.json` lists as
  resources (`docs/` by default; read-only) and nothing of the host's;
  the repository's own check command, declared in `swarm.json`, runs in a
  keyless container with no network. The forge token never enters a container: Daisy reads the
  member's output file and posts it.
- *Build it rather than adopt one.* The reasons are five things the
  design rests on: a forge that may be self-hosted, a cross-vendor
  adversary, gates placed by design and the first three openable, Daisy's
  own plan
  and test discipline in the architect's and reviewers' tasks, and the
  members being the vendors' CLIs so that only the pipeline is Daisy's.

What the first job taught (2026-11-07, one issue from proposal to close in
2h44m, two slices, two review rounds on the first and one on the second,
about $7.60 at list price for the model calls that report a cost; the
cross-vendor reviewer's runs report none). Three corrections were needed:
the brief had to exist before the owner proposed, or it proposed from the
whole documentation tree; the adversary's instructions had to be bounded so
that it does not demand a check that cannot be made; and the maintainer's
label changes and merges, made by hand that morning, became the six verbs
that afternoon. The members ran for about thirty minutes of the 2h44m; the
rest was waiting, at the four gates and for the next five-minute pass, and
a pass that waits for a long run delays the others, which is open work. The cost of the design is
latency and polling; what it buys is that everything is on the forge, and
a person can return at any cadence and read what happened.

## Usage

### Install

```bash
git clone <repo-url> daisy
cd daisy
make build              # the Go binary, bin/daisy (Go 1.27, standard library only)
bin/daisy install
```

`install` links `~/bin/daisy` to `bin/daisy`, installs the root's skills
into `~/.claude/skills/`, adds `DAISY_ROOT` to your shell rc file and grants
Claude Code the permissions `daisy` needs. Open a new shell (or source the
rc file `install` named), then create a home if you have none:

```bash
daisy home new work
```

It asks for a location (default `~/.local/daisy/homes/work`) and the
git author for the home, makes the home its own repository with a first
commit, and registers it. `daisy home add <name> <path>` registers a home
that already exists.

### Bind a workspace

```bash
cd /path/to/your/project
daisy init work
```

`work` here is an identity. An identity is the author a workspace's
commits carry, and it belongs to a home, so naming one tells `init` which
home this workspace reads and writes and whose name is on its commits. The
simplest identity is a home's own name: `work` is the `work` home's default
identity, read from its `gitconfig`. A home may define further identities
as files in `identities/`; `daisy init <id>` finds such a file when exactly
one registered home defines it, and refuses when two do, naming both.

`init` writes `.daisy/` (the home's name, the identity when it is not the
home's default, a record of the git configuration it applied, links to the
home's tasks, journal, projects, plans and feedback, and the generated
`AGENTS.md`), sets the identity in the repository's local git
configuration, configures each supported agent tool and adds its own
entries to `.gitignore`. Run it again to switch home or identity; `daisy
clean` removes all of it and restores the git configuration.

| Tool | What `init` writes | How to address Daisy |
|------|--------------------|----------------------|
| Claude Code | `.claude/commands/daisy.md` (the `/daisy` command), `.claude/settings.local.json` (permissions), the home's skills in `.claude/skills/` | `/daisy <request>` |
| Cursor | `.cursor/rules/daisy.mdc`, `daisy-logging.mdc`, `daisy-plan.mdc`, `.cursorignore` entries | `Daisy, <request>` |
| Codex | `.agents/skills/daisy/SKILL.md` (the `daisy` skill), the home's skills in `.agents/skills/` | `$daisy <request>`, or by matching the skill's description |

The examples below use the Claude Code form.

### Check

```bash
daisy healthcheck
```

It confirms `DAISY_ROOT`, that the home resolves and is its own repository,
that the required files exist, that today's file is well formed, that each
command's own check passes and that no publishable file in the root carries
an identifying term. A pass is cached for 300 seconds (`DAISY_HEALTHCHECK_TTL`
changes the window; `0` disables it); `daisy healthcheck --force` re-checks
now, which is what to run before a root commit.

### The day

```
/daisy start a new day
/daisy add a task: review the deploy pipeline +infra
/daisy done "deploy pipeline"
/daisy log Shipped the fix after pairing with ~mel
/daisy how's +website going?
/daisy start a new week
```

The agent reads `.daisy/AGENTS.md`, runs `daisy new-day`, `daisy done`,
`daisy log` and `daisy new-week` for the mechanics, and edits `todo.txt`
and the project files for what needs judgment. People are written as
`~alias`, resolved through `tasks/alias.txt`. Each change the agent makes
through one of those commands ends in a commit in the home (`feedback` and
`plan new` leave theirs for the next); `daisy commit <message>` commits
edits made outside a command, the agent's own included. `prompts/daisy.md` is the instruction the agent
follows; `docs/examples.md` has walkthroughs.

### Plans

```
/daisy plan <description> +project
/daisy execute PLAN.md
/daisy resume PLAN.md
/daisy archive PLAN.md
```

`plan` researches first, then scaffolds Goal, Steps and Decisions and asks
before executing; `execute` runs each step in a fresh agent, reviewed by
another to a stated gate with a cap of five passes, verifying each before
the next; `resume` verifies the state of an interrupted plan before going
on. `prompts/plan.md` has the rules; `docs/plan.md` the reference.

### Feedback

```bash
daisy feedback --workflow daisy "logged the meeting without naming who was there"
daisy optimize                       # at the new week: the briefing
daisy optimize apply --workflow daisy --rules <file>
```

`feedback` records the entry; `optimize` briefs the agent in your session;
the agent writes its proposal to a file; `apply` shows the diff, asks, and
writes a home override (`--rules` adds rules; `--rewrite` replaces the
whole file).

### The swarm

In the workspace of the product the swarm is to work on, with the swarm's
forge token in `.env.sh` (copy `templates/env.sh.template`):

```bash
daisy swarm init                 # the wizard: forge, repositories, personas, gates
daisy swarm apply --once         # one pass; the printed service runs the loop, apply without --once
daisy swarm status               # what the swarm did, and what waits for you
```

Write a brief, a Markdown file under a directory `swarm.json` lists as a
resource (by convention in `<ws>/docs/briefs/`): a header with `Status:
active` and the brief's priority in words, then one `## ` heading per
milestone. The owner proposes jobs from it as issues; your decisions are
the verbs, each under your own token in `.env.sh` as
`DAISY_MAINTAINER_TOKEN`:

```bash
daisy swarm triage 12            # proposal-posted → proposal-accepted: the architect plans it
daisy swarm accept 12            # plan-0-posted → plan-1-accepted: the worker builds the next slice
daisy swarm approve 12           # a formal approving review, where a branch rule wants one before the merge
daisy swarm merge 12             # merge the approved pull request, then the next slice or close
daisy swarm unblock 12 <comment> # say what changed, then clear swarm:blocked: its stage runs again
daisy swarm block 12 --by 9      # hold #12's job until #9 is closed
daisy swarm pause 12             # hold #12 alone; daisy swarm resume 12 releases it
daisy swarm close 12
```

In an agent session in that workspace, the `swarm` skill reads `status`,
the briefs and the runs, explains what happened and what waits, drafts
briefs and configuration changes, and runs a verb only after you have
confirmed that write in words. It never reaches the forge except through
`daisy swarm` commands (`status` to read, the verbs to write); `init` and
`apply` are yours, from a terminal.

The contract for all of this is `docs/swarm.md`: the configuration file,
the labels and transitions, the roles' prompts and tasks, the container,
the credential rule, the forge and provider seams, and each verb's reads,
writes, output and exit codes.

### Workspace layout, after `daisy init work`

```
workspace/
├── .daisy/
│   ├── home                 # "work": the home's registered name
│   ├── identity             # only when the identity is not the home's default
│   ├── gitconfig            # the identity init applied; clean unsets against it
│   ├── AGENTS.md            # the prompt generated for this home
│   ├── tasks/                → {home}/tasks/
│   ├── today.md              → {home}/journal/today.md
│   ├── journal.md            → {home}/journal/journal.md
│   ├── projects/             → {home}/projects/
│   ├── plans/                → {home}/plans/
│   └── feedback.md           → {home}/feedback/feedback.md
├── swarm/                    # only where a swarm is declared: swarm.json, prompts/
└── PLAN.md                   → .daisy/plans/{current}.plan.md, while a plan is active
```

Beside these sit the tool files of the table above (`.claude/`, `.cursor/`,
`.agents/`) and `init`'s block in `.gitignore`. `daisy files` prints each
of the home paths resolved. Different workspaces may use different homes,
or one home under different identities, at the same time.

## Prompts vs. Skills

Daisy's instructions are of two kinds, and one question divides them:
**does this need to know your situation, or only the artifact in front of
it?**

A **prompt** (`prompts/`) is situated. It knows what your projects are,
what you did yesterday and how you work, and it reads and writes the files
Daisy owns: `daisy` (tasks, journal, projects), `plan`, `praxis`,
`retrospective`, `github` and `socrates`. A home's `include.txt` names the
prompts that load for it, and `daisy prompt build` compiles them into the
generated `AGENTS.md`: a name alone includes the whole prompt; a name with
a `~` prefix includes only its `## Trigger` section, and the agent reads
the full file when the trigger fires. A home may add prompts of its own and
may override a root prompt by name, in layered or full form (above).

A **skill** (`skills/`) is craft. It knows the work, not you. Six assume
nothing about Daisy at all: `agents-md`, `css`, `git`, `js`, `testing` and
`transcribe`; the seventh, `swarm`, is the maintainer's guide over a swarm
and knows Daisy's `swarm` verbs, but nothing of your tasks or journal. The
root's
skills are installed once per machine by `daisy install`, so they keep
working in repositories that were never bound; a home's skills are copied
into each workspace by `daisy init`. A skill is a directory with a
`SKILL.md` whose frontmatter description is what the agent matches a
request against; there is no manifest, and every skill in the root and the
home installs.

The two look alike (both are markdown with a trigger, both can load on
demand) and the mechanism is not what separates them. The question is
coupling, and it stays stable as the tooling changes; who refines a file
over time is a different question, and its answer does not move the file,
which is why the feedback loop reaches both layers. Where
the line falls in less obvious cases, with a worked example of splitting
one file, is [docs/prompts-vs-skills.md](docs/prompts-vs-skills.md), which
this section summarises and does not replace.

## Recommendations

In most cases: point your own agent at this repository and have it roll the
ideas into whatever you already use. The ideas travel without the tool: a
task list and a journal the agent maintains in plain text under git; the
line between what a command does exactly and what an instruction leaves to
judgment; a correction recorded and compiled back into the instruction that
failed; a plan executed one step per fresh agent, reviewed to a stated gate
with a cap; a swarm whose state is the forge's labels, whose reviews come
from two vendors and whose gates are a person's.

Adopt Daisy itself when the root and home split and the identity model are
what you want: several contexts with separate histories on one machine,
workspaces that commit as different authors from one set of tasks, and a
public code repository with a check that keeps personal terms out of it.
Expect a Go build, a registry file and a binding step per workspace.

Adopt the swarm only if the reasons for building it hold for you as well:
a forge the swarm has an adapter for (Gitea today; GitHub is specified, not
built); a second vendor's CLI for the adversary; a wish
to decide at the gates yourself. It has run on one product; its costs and
its open work are recorded above and in `docs/swarm.md`. Where it is not
what you want, the parts that stand alone are the brief's form, the
cross-vendor review pair and the marker that tells a machine's words from a
person's.

Further reading: `AGENTS.md` for the agent-facing account of the root
(layout, the `Ops` rule, publication hygiene, the command reference);
`docs/cli.md` for the command contract; `docs/home-management.md` for
roots, homes, identities and resolution; `docs/swarm.md` for the swarm;
`docs/testing.md` for the testing method the plans and the swarm's
reviewers follow.
