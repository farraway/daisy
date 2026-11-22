# The `daisy` CLI — Contract

This is the contract the Go binary meets, as built: the command tree, and
under it one entry per command with its arguments, flags, exit codes, the
stdout and stderr the acceptance suite asserts, the `Ops` wrapper it runs
under, the files it may write and the commits it makes.

**Status of each entry.** `as built` describes the binary as the suite
pins it; since Step 24 every command is Go and the bash (`daisy.sh` and
`daisy/scripts/*.sh`) is deleted, so a `Legacy` row is history: the bash
form a command replaced. Every entry is `as built (Step N)`, the step
that built it, the `swarm` verbs `as built (Steps 1, 4, 5, 6 and 10 of the
swarm build)`, each specified in [docs/swarm.md](swarm.md). `decided (D30, …)` marks a point the plan's
Architecture left open that this document settled, recorded in the plan's
D30. Where the bash has no
explicit exit code or message for a situation the entry says *unspecified;
Go may choose* rather than inventing one.

**Quoted strings.** A string in backticks followed by case numbers in square
brackets — `` `No task found matching:` [04] `` — is asserted verbatim by
those cases in `tests/cases/`; the suite is the authority, this
document quotes it. Strings without a case number are what the bash printed
and are not asserted; Go may reword them.

**Mapping rule.** Every command the bash had names exactly one entry (its
`Legacy` row); every case file names exactly one entry (its `Cases` row). A
case that runs several commands belongs to the command its name pins —
case 72 (`done`, `log`, `new-day`, root untouched) is `commit`'s; cases
76–79 (`init`, then `status`, `files`, `clean`) are `init`'s — and the other
entries cross-reference it in prose.

## Command tree

`daisy --help` as built, rendered from the `cli` table; `daisy
help --markdown` renders the same table as AGENTS.md's Command Reference.
The `Swarm` group (`swarm init`, `swarm prompt build`, `swarm apply`,
`swarm metrics`, `swarm status`, `swarm list`, `swarm journal`, `swarm connect`, `swarm pause`, `swarm resume`, `swarm restart`, and the maintainer's
`swarm triage`, `swarm accept`, `swarm class`, `swarm approve`, `swarm
merge`, `swarm unblock`, `swarm block`, `swarm replan`, `swarm close` and `swarm intervene`) joined the
table a verb at a time as each was built; all twenty-one are in it, and the tree never names a command the binary does
not run. Their entries below give docs/swarm.md's revision 6 (the swarm
repository as the swarm's root, run from its clone with no Daisy
workspace needed, the secrets in the swarm's `<state>/env`), which
Steps 2–6 of its plan built; Step 7 retired the old workspace layout,
which docs/swarm.md section 11, Moving a swarm into its repository,
still describes for a maintainer moving a swarm with an older Daisy.

```
Usage: daisy <command> [args...]

Workspace
  init [<identity> [path]]          Bind a workspace to an identity and its home (bare: re-initialize this one)
  clean [-f]                        Remove Daisy from this workspace
  status [--json]                   Home, identity, projects, today's counts, active plan (read-only)
  files [--json|<name>]             Resolved real paths for every home file, or the one path <name> names (plan, today, todo, …) (read-only)
  list [--json]                     Active prompts and installed skills (read-only)
  help [<command>|<skill>|--markdown]
                                    This help, detail for one command or skill, or the Command Reference (read-only)

Day
  new-day [--quick] [--force]       Start a new day, unless today has started (--quick: a week if one turned; --force: even so)
  new-week [--quick] [--force]      Start a new week, or the day when this week has started (--quick: as new-day --quick; --force: a week even so)
  log <message...>                  Add a log entry to today.md, quick-starting a stale day
  add <description> A|B|C|D|none    Add a task to todo.txt, and to today.md when (A) or (B)
  done <pattern>                    Mark a task complete
  cancel <pattern>                  Cancel a task; the next new day deletes it
  priority <pattern> A|B|C|D|none   Change a task's priority, and its section in today.md
  tasks [--all] [--todo|--done] [--json] [pattern]
                                    Search tasks (read-only)
  projects [--archived] [--json]    List active or archived projects (read-only)
  rotate                            Rotate journal.md into window files

Plans
  plan new [--spec] <description> [+project]
                                    Create a plan (or an unregistered spec draft)
  plan pickup <path>                Promote a spec draft into a tracked plan
  plan archive                      Archive the active plan
  metrics [--swarm <owner>/<repo>] [--since <time>] [--json]
                                    Summary of the home's metrics CSV, by kind and model (read-only)
  metrics add --step <s> [--plan <p>] [--range <r> [--repo <dir>]] [<flag> <value>...]
                                    Record a plan step: a row in the home's metrics CSV and in the plan's ## Metrics table

Homes and identities
  home new <name> [path]            Create a home; prompts for identity and location
  home add <name> <path>            Register an existing home
  home list                         Registered homes and where they live (read-only)
  home remove <name>                Unregister a home; its files stay where they are
  identity list                     Identities across registered homes (read-only)

Prompts and skills
  prompt build [<home>]             Rebuild AGENTS.md
  install [--update]                ~/bin/daisy and DAISY_ROOT; nothing under ~/.claude
  feedback [--workflow <n>] <text>  Record a prompt failure for optimization
  optimize [--workflow <n>]         Briefing: pending entries per target, current rules, rewrite flags (read-only)
  optimize apply --workflow <n> (--rules|--rewrite) <file> [--yes]
                                    Apply a proposal: diff, approve, archive, commit
  eval [--record] [<case> [<result>]]
                                    List, display, or record an eval case result

Swarm
  swarm init [--defaults] [--api <url>]
                                    Declare the swarm: swarm.json, prompts, labels
  swarm prompt build [--force] [--dry-run]
                                    Reseed prompts/ from this Daisy's defaults; an edited prompt is kept
  swarm apply [--once]              Reconcile the forge: launch members, post their output, move labels
  swarm metrics --backfill | --reprice [--dry-run] | --scorecard [--since <time>] [--until <time>]
                                    Write each collected run's missing metrics row (--backfill), supersede a row its output now reads otherwise (--reprice), or print a scorecard row per merged slice (--scorecard)
  swarm status [--json] [--waiting] Stages, gates, running members, what waits for you (--waiting: that alone) (read-only)
  swarm list [--json]               The swarms this machine runs: clone, home, pause; from anywhere (read-only)
  swarm journal [--json] [--issue <n>] | --mark <run>
                                    The journal since the line marked seen (--issue: one issue's lines); --mark moves the mark, its one write
  swarm connect [--json]            The connect briefing: journal by issue, waits with gate notes, questions, AUTHORITIES.md, metrics (read-only)
  swarm pause [<n>] [--type <type>] Launch no new member until resumed (with <n>, as the maintainer, on that issue; --type: record the intervention)
  swarm resume [<n>] [--type <type>]
                                    Launch members again and release the owner's held batch (with <n>, that issue; --type: record the intervention)
  swarm restart [--type <type>]     Have the running apply re-execute itself, onto the installed build, once no member runs (--type: record the intervention)
  swarm triage <n> [--type <type>]  As the maintainer: triage a proposal, proposal-posted to proposal-accepted
  swarm accept <n> [--type <type>]  As the maintainer: accept the plan, plan-0-posted to plan-1-accepted
  swarm class <n> [--type <type>] routine|architecture
                                    As the maintainer: set the job's class; Daisy passes a routine job's triage and plan
  swarm approve <n> [--type <type>] As the maintainer: approve the issue's open pull request
  swarm merge <n> [--type <type>]   As the maintainer: merge the approved pull request, then next slice or close
  swarm unblock <n> [--cause <word>] [--start base] [--type <type>] <comment...>
                                    As the maintainer: comment, then clear swarm:blocked from the issue (--cause: retag the stop's cause; --start base: the slice's next worker run starts as with no kept run)
  swarm block <n> --by <m> [--type <type>]
                                    As the maintainer: hold the issue's job until #m is closed
  swarm replan <n> [--type <type>] [direction...]
                                    As the maintainer: have the architect re-plan the job, from your direction
  swarm close <n> [--type <type>] [comment...]
                                    As the maintainer: close the issue, after the comment when given
  swarm intervene --type <type> [--issue <n>] <note...>
                                    As the maintainer: record an intervention no verb makes (a hand edit of swarm.json, a hand-published patch; --issue: on that issue)

System
  commit [--root] <message>         Commit the home repo (or the root repo)
  healthcheck [--force]             Run the system health check
  check-secrets                     Check which secrets/tokens are configured (read-only)
  test [filter]                     Run go test, then the hermetic acceptance suite
  version                           The commit this binary was built from; also --version (read-only)

Every command also accepts --help/-h (as the sole argument) to print its usage.
Every command that commits (its usage says so) also takes --co-author "Name <email>".

Environment:
  DAISY_ROOT    Path to the daisy repository (required)
```

## Renames

| Legacy (the bash) | As built | Step |
|---|---|---|
| `daisy build [home]` | `daisy prompt build [<home>]` | 20 |
| `plan` and its verb as one hyphenated word, three bash scripts | `daisy plan new [--spec] <description> [+project]`, `daisy plan pickup <path>`, `daisy plan archive` | 22 |
| `daisy commit --home <message>` | `daisy commit <message>` | 24 |
| `daisy commit --not-home <message>` | `daisy commit --root <message>` | 24 |
| `daisy init --new <home>` | removed: `daisy home new <home>`, then `daisy init <home>` | 21 |
| `daisy optimize [--workflow <n>] [--dry-run] [--force]`, `--list` | `daisy optimize [--workflow <n>]` (briefing) and `daisy optimize apply …` | 23 |
| — | `daisy identity list` (new) | 21 |

Every rename is applied to `prompts/`, `templates/`, `docs/` and `AGENTS.md`
in the step that ports the command; there are no aliases. Everything not in
this table keeps its name and every flag the suite asserts.

## Common behaviour

**Exit codes.** `cli.Run` returns 0 on success, 1 on failure, 2 on a usage
error (unknown command, unknown flag — before the positionals, a word
shaped like a long flag, `--` and then letters, digits and dashes, that is
not one of the command's flags, `Error: unknown flag: --specc`; any other
word starts the positionals, `tasks "-1 point"`, `done "--dry-run support"`
—, missing or surplus positional argument, and — for a command whose
arguments have places, every one but `log` and `feedback`, whose words are
one text — an empty word before a given one, `Error: empty argument`, so
that `plan new "" +proj` is not the description `+proj` *(D37, Step 22)*;
`swarm close`, `swarm unblock` and `swarm replan` refuse one in `<n>`'s place,
a positional word that is one of the command's own flags, since flags
lead, `Error: --co-author follows the arguments; flags lead: daisy log
--co-author <x> <message...>`, the command not run (the maintainer's swarm
verbs, which read `--type` and their own value flags right after `<n>`,
aside; `TestATrailingFlagIsRefused`), and a `--co-author` value that is not one line `Name <email>`, Commits below), with
the command's usage on stderr. The bash exits 1 for usage
errors too; no case asserts a usage exit code, so 2 is free to take
*(built in Step 17)*. A value the command refuses — an unknown identity, an
invalid name, an ambiguous pattern, no match — is a failure, exit 1, as the
cases pin it (`home add bad=name` exits 1 in case 68; `not a valid identity
name` [77] exits 1) *(decided, D30: Architecture says only "exit 2 and
usage on bad args")*.

**Unknown command.** Bash: `Unknown command: <name>`, a blank line and the
help text, all on stderr, exit 1. Same text, exit 2 *(decided, D30; built in
Step 17: the binary knows every command of either side, so an unknown name
never reaches the bash)*. Not asserted.

**`--help` / `-h`.** As the sole argument of any command prints that
command's usage on stdout and exits 0 without running it; the bash reads it
from the script's header comment, the binary from the `cli` table (`Usage:
daisy <usage>`, blank, the description, ending ` (read-only)` for a
command the table runs under `ReadOnly` (`Ops` wrappers below), blank,
`Example: daisy <command line>`, one real use of it, whose words a test
parses as the command's usage (`TestExamplesParse`); for a command that
commits, a blank line and `It commits: --co-author …`, Commits below; a
usage error prints the same block on stderr). A noun phrase without its
verb, of one word or more (`home`, `swarm prompt`), is a usage error,
`Error: missing command: daisy <phrase> <command>`, or with another word
`Error: unknown command: <phrase> <word>`, the phrase in full, listing
the usages of the commands under it; with `--help` alone it lists them on
stdout, exit 0 *(Step 11 of the plan remediating the 2026-11-15 design
review, P5)*. `daisy`, `daisy help`, `daisy --help`,
`daisy -h` print the help: the tree above, from the `cli` table, in its
groups and order (`plan new`, `plan pickup`, `plan archive`, `home new`,
`home add`, `home list` and `home remove` one row each) — the tree never
names a command the binary does not run. *The four forms and the sole-argument
rule are as built and kept (decided, D30: Architecture does not mention
them).*

**Workspace.** Most commands run inside a workspace: the nearest ancestor of
the working directory holding a `.daisy/` directory that is not the Daisy
root. Outside one the command prints `Not in a Daisy workspace` [78]
`(no .daisy/ directory found)` and `Run 'daisy init <identity>' to initialize
this workspace.` on stderr and exits 1. A directory whose binding is broken
is refused rather than walked past, exit 1, stderr naming the fault and
`daisy init` [78] as the repair: `.daisy` `not a directory` [78],
`symlink` [78], a leftover `.daisy.previous.4242` [78] from an interrupted
init, or `.daisy/` with no `home` file. Commands that need no workspace:
`install`, `init` (bare, it re-initializes the one it is in, and outside one
the identity is the missing argument), `home`, `identity list`, `help` (tree or command),
`prompt build <home>`, and `test`, whose bash dispatcher required one although the script read
only the root *(decided, D30; built in Step 24)*, and the twenty
`swarm` verbs, which find their swarm themselves (docs/swarm.md section
11, Finding the swarm: a swarm repository's clone; Step 2 of revision
6's plan, the old layout's workspace no longer read since its Step 7),
`swarm list` needing none: it lists every swarm this machine runs. `healthcheck` needs one
but resolves it itself, after `✓ DAISY_ROOT`, so outside one it prints the
refusal and then `✗ Cannot resolve home for this workspace` (the bash
dispatcher refused before the check began). `check-secrets` keeps the requirement — it reads the workspace's
`.env.sh` — but needs only the workspace, not a resolvable home, as the
bash, or, since Step 2 of revision 6's plan, a swarm (below).

**Home resolution.** `.daisy/home` names the home; the name is looked up in
the registry `$DAISY_CONFIG/homes` (default `~/.config/daisy/homes`). Errors,
exit 1, stderr: a name with no registry line — `not registered` [68], and
the fix `daisy home add second` [68] (the name substituted); a registry
path with a problem — `Error: Home '<name>' is registered at <path>, which
<problem>`, the problem being `is ` `not an absolute path` [68], `does not
exist`, `is the Daisy root` [68] `; the root ` `cannot be a home` [68] or
`contains the Daisy root` [68] `; the root cannot be inside a home`. With no
workspace there is no home: the bash's default-home fallback, which nothing
reachable through `daisy` read since Step 20, is deleted with the bash
(Step 24). The identity is `.daisy/identity`, else the
home's name; a value that is not a name is refused: `not an identity
name` [77], exit 1.

**Binding drift.** `status` and `files` (without a name) print one warning line per fault on
stderr (with `--json`, under their `warnings`, below), each ending with the fix `daisy init` [76] `<identity>`: a missing
or wrong link (naming the file, `today.md` [76]), a git key whose value is
not the recorded identity's (naming the key, `user.email` [76]; `differs`
[77]), an identity file that `does not match what init recorded` [77], a
bound identity file that is not a complete identity (naming the file), an
applied identity nothing claims (naming the author, `Pelican Moorhen` [79]).
A clean binding prints nothing on stderr (asserted empty in 76, 77, 79).

**`--json`.** A read-only command that takes `--json` among its leading
flags (as built, `status`, `files`, `list`, `tasks`, `projects`,
`metrics`, `swarm status`, `swarm list`, `swarm journal` and `swarm
connect`; a command
without it refuses it as an unknown flag, exit 2) prints, with it, its
report as one JSON
document on stdout and nothing else: an object, indented two spaces,
ending in a newline. The text stays the default, unchanged, and both
forms are printed from one report (the document only once it is read
whole; `list`'s text, as before, also from what it read before a
failure). The document
carries the facts the text prints, not its rendering: one key per label,
in `snake_case` (but the names a command already gives a thing, the
`files <name>` names, a stage's label); an issue is a number, a stage its
label (`swarm:plan-0-posted`), a path the real path the text prints, and
a fact two commands print under one key in both (`workspace_projects`). Every key
is always present: `null` where the text prints nothing for a value
that is absent (no current plan, no budget), `[]` where it lists none.
The keys are pinned by tests (`TestFilesJSON`, `TestSwarmStatusJSON`,
`TestSwarmStatusJSONThroughGitea`, `TestReadOnlyJSON`, `TestSwarmConnectThroughGitea`), so a change to one is a change to
this contract. Every document has `warnings`: the lines the command
would have written on stderr (drift, a secrets file's problem, a
blocker naming no issue), each as the text form prints it, `[]` when
none, so on success stderr carries nothing. A failure writes no
document, partial or whole: stdout stays empty, the exit code is the
text form's, and stderr carries the held warnings and then the error, as
without the flag (`tasks --json <pattern>` matching nothing is exit 1,
`No tasks match: <pattern>`, no document); a caller tells success by the exit code alone *(Step 9
of the plan remediating the 2026-11-15 design review, P1a; the other
read-only commands its Step 10, P1b)*.

**Environment read.** `DAISY_ROOT` (required: without it a command exits 1
with `Error: DAISY_ROOT not set` and the `~/.zshenv` line to add;
`install` alone finds its root without it, from the binary's own path,
Step 21, and refuses one naming another root, Step 35), `DAISY_CONFIG`, `DAISY_LOCAL`, `HOME`, `USER`,
`DAISY_HEALTHCHECK_TTL`, `DAISY_HEALTHCHECK_PASSED` (see `healthcheck`),
`DAISY_CO_AUTHOR` (every committing command without `--co-author`, see Commits below), `NO_COLOR` and `TERM` (Colour below), `SHELL` (`install` picks the rc file by it), `PATH` (`test` puts the
root's `bin/` first on it for the suite), the variables `check-secrets`
reports, and the swarm's: the variable `forge.token_env` names (`swarm
init`, `apply`, `status`; the maintainer's verbs only to compare it), each
used `keys` `env` (`apply`) and `DAISY_MAINTAINER_TOKEN` (the maintainer's
verbs, `apply` only to run a comment command through them, and `swarm
init` for its writes on the swarm repository), each when
the swarm's secrets file, `<state>/env` (docs/swarm.md section 3), does not assign it. Nothing else: `optimize` holds no API key and reads no
model or editor setting (Step 23, D12).

**Colour.** Daisy writes ANSI colour only to a stream that is a
terminal (`Ops.IsTerminal`, the terminal driver's answer, so a pipe, a
file or `/dev/null` is none), each stream asked for itself (a piped
stdout leaves stderr's colours, as clig.dev's Output section has it),
and never when `NO_COLOR` is present and not empty, whatever its value
(<https://no-color.org>: an empty `NO_COLOR` is no request), nor when
`TERM` is `dumb` (clig.dev). One function decides, `ops.Colour`; off it
the text is the same, the marks (`✓`, `⚠`, `✗`) kept and the escape codes
dropped. As built, `healthcheck`'s marks are the only colour Daisy
writes; a `--json` document carries none. No flag or variable of Daisy's
forces colour or takes it away for one command: `NO_COLOR=1 daisy <command>`
does the latter *(Step 15 of the plan remediating the 2026-11-15 design
review, P12)*.

**Next.** The commands on a newcomer's path end their success output, on
stdout, with a blank line and one line of `Next:` and the next step: a
command, or the prompt to give the agent (clig.dev, "suggest the next
command"): `install` names `home new` (or `home add` for a home that
exists) while no registered home's directory exists and `init <home>`,
the first registered home whose directory does, once one is, `home new` and `home
add` name `init <name>`, and `init <identity>` names `/daisy` in the agent.
A form a newcomer does not run prints none: `install --update` re-applies
an install already made (`make install-dev` runs it after a pull) and the
bare `init` refreshes a binding already made, so a `Next:` line there
would repeat a step already taken. With no home registered, `init` fails
naming both `home new` and `home add`, on stderr with the error
(`TestNewcomersNextLines`) *(Step 16 of the plan remediating the
2026-11-15 design review, P13)*.

**`--healthcheck` hook.** The commands with a `Health` function in the
`cli` table — `list`, `new-day`, `new-week`, `log`, `done`, `tasks`,
`projects`, `rotate`, `plan new` (`plans/` and `templates/plan.md`), `plan
pickup` (`plans/`), `plan archive` (`plans/archive/`), `home new` (the
template), `feedback` and `eval` (the workspace and its home resolve),
`optimize` (the home's `feedback/`) and `test` (`tests/run.sh` is there
and executable) — accept `--healthcheck` as the sole argument: check their
own preconditions, print nothing on success, exit 0; else an `Error: …`
line on stderr, with the indented line of the fix when the refusal has one
(`  Run 'daisy init <identity>' …` outside a workspace, `new-day`'s and
`new-week`'s `  Looked for: …`, case 87), exit 1. The hook is hidden from the help text. Three
cases pin it from outside (`done --healthcheck` [18], `tasks
--healthcheck` [30], `new-day --healthcheck` [87]). `healthcheck` calls the same functions in-process,
in table order, as its `Component: <command>` lines (the bash ran the
binary once per command and `files.sh`/`test.sh` as scripts; `files` has
no hook) *(decided, D30, Step 18; in-process since Step 24)*.

**`Ops` wrappers.** The `cli` table marks each command that runs under
`ReadOnly` as a whole (`ReadOnly`), and the cli wraps it, so the command
cannot write; its help, the tree and the Command Reference print the
mark as ` (read-only)` after its description, never written by hand
(`TestReadOnlyMarker`), and each entry's `Ops` row below starts with the
whole command's wrapper (`TestCliMdOpsIsTheMarker`) *(Step 11 of the plan
remediating the 2026-11-15 design review, P5)*. `ReadOnly` for `status`,
`files`, `list`, `projects`, `tasks`, `help` and `optimize`, the
briefing (Step 23), `metrics`, `version`,
and for `check-secrets`, `home list` and `identity list` (decided, D30),
and `swarm status`, for which `ReadOnly` lets a `GET` or `HEAD` request
and `docker ps` through (decision [31](swarm/decisions/0031-status-runs-under-a-widened-readonly.md)), and from
revision 6 git's reading commands on a mirror (decision [99](swarm/decisions/0099-members-read-pushed-content-only.md)), and `swarm
list` (Step 15 of the plan for the inbox requests F27-F35);
`Real` for everything else, `eval` included (one wrapper per command;
`--record` writes), `optimize apply`, and `healthcheck`, because a pass
writes the TTL stamp. A command that writes only in one form narrows
itself to `ReadOnly` in the others and is `Real` in the table: `swarm
journal` without `--mark` (Step 15 of that plan), `swarm prompt build`
with `--dry-run`.

**Home files.** A home file a command rewrites is written whole and atomically (a temporary file renamed into place) and, when it is a symbolic link (kept with the user's dotfiles, say), through the link: the target takes the content and the link stays. `log`, `done`, `add`, `cancel`, `priority`, `new-day`, `new-week` and `rotate` write `journal/today.md`, `journal/journal.md`, its window files, `tasks/todo.txt` and `tasks/done.txt` so (`TestDayWritesGoThroughLinks`), as `plan archive`, `feedback` and `metrics add` write theirs; the bash's `mv` replaced a linked `todo.txt`. A file that exists keeps its mode; a new one takes its mode under the umask, and so does a tree Daisy copies for the user to keep, a new home and each skill installed in a workspace, as the bash's `cp -r` made them (`TestCopiesTakeTheUmask`); `init`'s stage of `.daisy/` keeps the modes it copies.

**Commits.** A command that commits does so exactly once, in the home's
repository, as that repository's own git config — never in the root (case
72: `done`, `log` and `new-day` add one home commit each and the root
repository's commit count and `git status --porcelain` are unchanged).
As built, the committing commands are `done`, `add`, `cancel`,
`priority` *(Step 31 of the plan remediating the 2026-11-15 design
review)*, `log`, `new-day`, `new-week`,
`plan pickup`, `plan archive`, `optimize apply` *(Step 23)*, `commit`
and `home new` (the new home's first commit); every commit the binary
makes in a home, and `commit --root`'s, goes through `gitx.Repo.Commit`,
one commit per command *(built, Step 18; `home new`'s first commit, made
by its own `add -A` and `commit` from Step 21, since Step 10 of the plan
for the inbox requests F27–F35)*; `plan new`, `feedback`, `eval --record`,
`metrics add` and `rotate` run by hand write home files without committing them, and the next home commit sweeps
them up *(decided, D37 for `plan new`, D38 for the other two; `metrics add`
as `feedback`, Step 16a of the plan that clears the Deferred list)*. The root
is committed only by `commit --root`, by the developer's hand: `optimize
apply` never writes a root target, it writes the home's override of it
(D49). Each committing command takes `--co-author "Name <email>"` among
the flags leading its arguments, as an agent running Daisy gives its
model's line (`Claude Opus 5.5 <noreply@anthropic.com>`), as in
`daisy log --co-author "<line>" <message>`; without the flag,
`DAISY_CO_AUTHOR` gives the value, and the flag wins when both are set. With either, each
of these commits ends with one `Co-Authored-By: <value>` trailer, added
by git's `--trailer` under `trailer.ifexists=addIfDifferent`, pinned over
the user's own setting, so a message already holding it gets no second;
with neither, there is none (a person at the keyboard). A flag value that
is not one line `Name <email>` is a usage error before anything is
written: `Error: --co-author "<value>" is not one line 'Name <email>'`,
the usage, exit 2 (`TestCoAuthorFlag`). An environment value of another
shape is warned about on stderr, `⚠ DAISY_CO_AUTHOR "<value>" is not one
line 'Name <email>': committing without Co-Authored-By`, and left out:
the commit is made and the exit code is unchanged
(`TestHomeCommitsCoAuthor`). The flag is a command's, not global, and
only the committing commands take it (`daisy help <command>` says `It
commits`); any other refuses it as an unknown flag. It exists because an
agent's environment assignment (`DAISY_CO_AUTHOR=… daisy log …`) does not
match a permission rule for the bare `daisy` command, where the flag
does. The swarm's commits read neither: a member's carry its persona's
`co_author` (docs/swarm.md) *(Step 10 of the plan for the inbox requests
F27–F35; the flag, Step 7 of the plan remediating the 2026-11-15 design
review, P19)*.

## Workspace

### daisy init

| Field | Value |
|---|---|
| Status | as built (Step 21): Go (`internal/workspace`, `internal/identity`); `init --new` removed (`home new`, then `init`); the bare form as built (Step 12 of the plan that clears the Deferred list, 2026-11-12); the root's skills and its Read and Edit rules installed in the workspace, where `install` put them in `~/.claude` before (Step 13, 2026-11-12) |
| Legacy | `daisy init [--new] <identity> [path]` → the bash `daisy-init` script, deleted in Step 21 |
| Usage | `daisy init [<identity> [path]]` |
| Arguments | `<identity>`: a registered home's name (its default identity, `gitconfig`) or the name of `<home>/identities/<identity>` in exactly one registered home. `[path]`: the workspace directory, default `.`; it comes only after an identity (a path alone is read as the identity). No argument: the workspace the working directory is in (the nearest ancestor holding a binding, as Common behaviour's Workspace finds it) is re-initialized with the home its `.daisy/home` names and the identity its `.daisy/identity` names, else the home's default: what `daisy init <identity>` run there does, but with the binding's home, not the one the identity's name resolves to now (`TestBareInitReinitsTheBinding`); with no binding above the working directory, or one whose `.daisy/` names no home, the identity is the missing argument: `missing argument` [91], a line naming `daisy init <identity>` [91] (and `daisy identity list`), the usage, exit 2 [91], nothing written; with no binding and no home registered, the lines say so and name `daisy home new <name>` and `daisy home add <name> <path>`, then `daisy init <name>` (`TestNewcomersNextLines`). A third argument: usage on stderr, exit 2. After the root is updated, the bare form refreshes everything init writes (`.daisy/AGENTS.md`, the skills, the agent files, the allow rules); `prompt build` rebuilds only `AGENTS.md` and the materialised prompts |
| Exit codes | 0 bound (also when already bound, or re-bound to another identity); 1 refused, nothing written: for the bare form, a broken binding above the working directory (Common behaviour's Workspace messages) and a `.daisy/identity` that is not an identity name (names the file); identity in two homes (`identity 'pelicanmoor' is defined in more than one home: testhome, second` [78], `more than one home` [78]), unknown identity (names it, `nosuchone` [77], and lists the identities that exist, `pelicanmoor` [77]; with no home registered, `no home is registered` and the fix `Run 'daisy home new <identity>' to create one, or 'daisy home add <identity> <path>' to register an existing home`, `TestNewcomersNextLines`), `not a valid identity name` [77], a registered home that cannot be checked (names it and `does not exist` [78]), a named identity file that is incomplete (names the file `identities/pelicanmoor` [79] and the missing key `email` [79]), a malformed `sshkey` [79] (the default identity's too: it may be incomplete, never malformed), a `linked worktree` [76] (names the main checkout), a registered home's own repository (`is home '<name>''s own repository`: its commits carry the home's identity, `TestInitRefusesAHomesOwnRepository`), the directory holding the `Daisy root` [78], `.daisy` `not a directory` [77] or a `symlink` [77], two leftovers and no `.daisy/` (names both, `.daisy.previous.100` [77], `.daisy.previous.200` [77]), a settings file that is not a JSON object [77, 78], a directory where `.daisy/` holds a file of init's own [77, 78]; every refusal is decided before the first write — an unknown identity, an unreadable settings file, a file where init needs a directory or a directory where it writes a file, a skill (the root's or the home's) named like a core rule (`daisy`, `daisy-logging`, `daisy-plan`), a file of a skill (or of the root skill a layered one merges over) that cannot be read — so a refused init writes nothing, not even `git init`; 1 when a step of the final section fails: `did not finish` [77, 78] on stderr, the binding and the git keys rolled back byte for byte and the stage removed, while the Cursor rules, the Claude command, the skills, `.claude/settings.local.json` and the ignore lines written before that step stay as written, as the message says; `the previous git identity could not be put back` is added only when the keys, read again after a failed restore, differ from what they held (a failed `git config --unset-all` changes nothing, `TestInitFailedUnsetSaysWhatChanged`) |
| Stdout | `Initializing Daisy in: <path>`, `Home: testhome` [77], `Identity: testhome` [76] / `Identity: pelicanmoor` [77] (the build's own report is not printed; its warnings go to stderr); on a re-bind `Already initialized` [77] `with identity: <identity>` (`Already initialized with identity: pelicanmoor` [77, 91]; the bare form's first line, `Initializing Daisy in: ` [91], names the workspace, not the working directory) or the switch `pelicanmoor → testhome` [77]; the ✓ lines for each artifact, the skills' line followed, when init installed any, by `· Cursor reads each skill's copy in .claude/skills/ (its rule in .cursor/rules/ points there): removing .claude/ disables the skills in Cursor` (`TestInitAndCleanSayCursorSkillsReadClaude`; Cursor has no skills directory, so each skill's stub reads the Claude Code copy, `docs/prompts-vs-skills.md`, Where the generated file loads); `Bound .daisy/` [79] `to home: <home>`; a cleared previous identity names its keys (`core.sshCommand` [79]); `Done. Daisy is ready in this workspace.`; then, for `init <identity>` but not the bare form, a blank line and `Next: /daisy start a new day in Claude Code ('$daisy start a new day' in Codex; 'Daisy, start a new day' in Cursor)` (Common behaviour, Next; `TestNewcomersNextLines`) |
| Stderr | warnings, exit still 0: a root skill the workspace has its own of `is not installed` (Writes), `no complete default identity` [76] (the home's `gitconfig` is template or incomplete; no identity applied), an sshkey that is `not found` [79] (names it, `no-such-key` [79]), a reclaimed leftover `.daisy.previous.12345` [77], `No readable .daisy/gitconfig record` [79] of the previous identity, an aside that `could not be removed` [79]; errors as above |
| Ops | Real |
| Writes | `<ws>/.daisy/` as a whole (staged beside it, swapped in by rename): `home`, `identity` (non-default only), `gitconfig` (the applied triple), `gitconfig.before` (what the three keys held, once the previous identity was taken back, when any held a value), the six links `tasks today.md journal.md projects plans feedback.md` into the home, `AGENTS.md`, `version` (the stamp: the commit `daisy version` reports for the binary that ran init, its line and a newline, written only when the binary carries one; `status` and `healthcheck` compare it with their own, below; `TestInitStampsTheWorkspace`); `<ws>/.cursor/rules/daisy.mdc daisy-logging.mdc daisy-plan.mdc` (obsolete `daisy.md`, `daisy-logging.md` removed, case 32), `<ws>/.claude/commands/daisy.md`, `<ws>/.agents/skills/daisy/SKILL.md` (Codex's `daisy` skill, from `templates/agents-skill.md`, replaced whole; no root `AGENTS.md` block and no `.codex/` write, D54), `.daisy/prompts/<name>.md` for each layered prompt (as `prompt build`); `<ws>/.claude/skills/<name>/`, its twin `<ws>/.agents/skills/<name>/` (Codex's; both trees written, swept, guarded and removed by `clean` through one list, `skill.WorkspaceTrees`, `TestInitWritesCodexTwinTree`, case 90) and `.cursor/rules/<name>.mdc` for each skill of the root and of the home (by its directory's name, at any depth under `skills/`; a home skill wins a root skill's name: an override of a root skill installs as the root's skill directory, the home's other files over it, its `SKILL.md` the home's — merged when layered, `TestSkillOverrides`; a home skill of a root skill's name at another path installs alone; each root skill's copy is the root's directory byte for byte, `TestInitInstallsRootSkills`, case 90, every skill's copy taking its modes under the umask, as `cp -r` did, `TestCopiesTakeTheUmask`; a root skill whose name the workspace already uses for its own — `.claude/skills/<name>`, `.agents/skills/<name>` or `.cursor/rules/<name>.mdc` there, a file, a directory or a link alike, with no stub of init's claiming it — is not installed: stderr `The root's skill <name> is not installed: this workspace has its own <path>; rename or remove it, then re-run 'daisy init' to install the root's`, exit still 0, and none of its copies, stub or ignore lines is written, so `clean` leaves the workspace's own as it was (`TestInitKeepsAWorkspacesOwnSkillOfARootName`; a home skill's name is the home's choice and is installed over the workspace's, as before); a layer with no root skill refuses init; as a best-effort check, a path init writes that resolves into the root outside the workspace through a user-made link, such as `.claude/skills` or `.agents/skills` linked to the root's `skills/`, refuses init before any write, `TestInitRefusesASkillsLinkIntoTheRoot`) (a stub in exactly the form init writes — the one record of which skills init installed — whose skill neither the root nor the home has now is swept with its copy, case 33; any other rule is the user's; a link standing where a core Cursor rule or one of init's stubs goes is replaced by init's file, never written through, while a link carrying a root skill's name with no stub of init's is the workspace's own, as a file is, above), `<ws>/.claude/settings.local.json` (Daisy's allow rules: `Read(.daisy/**)`, `Edit(.daisy/**)`, `Read(/$DAISY_ROOT/**)` and `Edit(/$DAISY_ROOT/**)` (the `//`-anchored form, as `install` once wrote them in `~/.claude/settings.json`), `Bash(daisy:*)` [90]; the legacy `Bash(…/daisy/scripts/*)` removed), the entries missing from `<ws>/.gitignore` (`.cursorignore` among them, from Step 21) and `.cursorignore`, into the block under Daisy's header line (applying twice equals once); an entry a `.gitignore` of the repository already ignores is not added (`git check-ignore`, which names the rule that decides; an entry only this machine's `core.excludesFile` or `.git/info/exclude` ignores still is, a clone elsewhere lacking it, `TestInitSkipsOnlyWhatTheRepositoryIgnores`, and so is one under a directory that is a symbolic link, or a directory entry at one (`.claude` linked elsewhere, a skill's copy or its Codex twin linked; a file entry that is itself a link is still asked about), which is outside the repository's tree, `TestInitIgnoresThroughALinkedAgentDirectory`), so the Daisy root, whose `.gitignore` ignores every path init and `plan new` write there wholesale, gets no block, and `init`, `plan new` and `clean` leave it as git has it (`TestBindingTheRootLeavesItAsGitHasIt`); `git init` when the directory is not a repository top level, the new repository's first branch `main` (`git symbolic-ref HEAD refs/heads/main`) unless the user's git config sets `init.defaultBranch`, which then wins: read in the new repository, so the value is the one `git init` applied (a conditional include matched against the new `.git`; the config of a repository enclosing the directory is not the user's, and `git init` does not read it) (`TestInitStartsARepositoryOnMain`); local git config `user.name`, `user.email`, `core.sshCommand` (`ssh -i <key> -o IdentitiesOnly=yes`, only with `sshkey=`) — the previous identity's keys unset first where their values are still its, and what they held before that identity was applied put back |
| Commits | none |
| Cases | 32, 33, 76, 77, 78, 79, 90, 91; `TestInitStampsTheWorkspace`; `TestBareInitReinitsTheBinding`; `TestNewcomersNextLines`; `TestInitInstallsRootSkills`; `TestInitIgnoresWhatItWrites` (the `.gitignore` block ignores every path init and `plan new` write into the workspace, every entry is one of them or a stage or previous name with `*` for exactly its generated suffix, and none ignores a user's own file; each `.cursorignore` line re-includes an entry of the block, and `.daisy/`, `PLAN.md` and each core Cursor rule the block holds has one); `TestInitSkipsOnlyWhatTheRepositoryIgnores`; `TestInitIgnoresThroughALinkedAgentDirectory`; `TestBindingTheRootLeavesItAsGitHasIt` |

Case 76 also pins `init` under an inherited `GIT_DIR` (ignored, exit 0),
`log` under a hook's `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_COUNT` and
`GIT_AUTHOR_*`/`GIT_COMMITTER_*` (ignored; the commit lands in the home),
and that `init` in a subdirectory of a linked worktree is allowed.

### daisy clean

| Field | Value |
|---|---|
| Status | as built (Step 21): Go (`internal/workspace`) |
| Legacy | `daisy clean [-f]` → built in to `daisy.sh`, deleted in Step 21 |
| Usage | `daisy clean [-f]` (`--force` accepted as well; kept) |
| Arguments | without `-f`: prints `This will remove Daisy from: <path>`, `Home:`, `Identity:` and asks `Continue? [y/N]` through `Ops.Prompt` (on stderr, as the bash's `read -p`); anything but y/yes, end of input included, prints `Cancelled.` and exits 0 |
| Exit codes | 0 removed (also when nothing matched); 1 outside a workspace (beside the Daisy root, case 78); 1, before anything is removed, when `.claude/settings.local.json` is not a JSON object; 1 when a git key `could not unset` [79] — the record is kept for the retry, and the message is not `left as is` [79] |
| Stdout | `Cleared local git identity (user.name, user.email)` [76, 77, 79] naming exactly the keys cleared — `Cleared local git identity (user.name)` [76], `Cleared local git identity (user.email)` [79], `Cleared local git identity (user.name, user.email, core.sshCommand)` [79]; `left as is` [76] when nothing matched; `No readable .daisy/gitconfig record` [76] when there is no record and no identity file resolves; `Removed .daisy/`, `Removed <leftover>` (`.daisy.previous.555` [76]), `Removed Cursor rules`, `Removed Claude command`, `Removed N skill(s)…` and after it the line `init` prints beside the skills (`· Cursor reads each skill's copy in .claude/skills/…`, `TestInitAndCleanSayCursorSkillsReadClaude`), `Cleaned .gitignore`, `Removed empty .gitignore`, `Removed Daisy permissions from .claude/settings.local.json` or `Removed .claude/settings.local.json` (when only Daisy's rules were in it), `Cleaned .cursorignore`, `Removed empty .cursorignore`; `· Local git identity not checked: this workspace is not a repository` / `…this is a linked worktree and its config is the main checkout's` when the keys cannot be checked; `Done. Daisy has been removed from this workspace.` |
| Stderr | `could not unset` [79]; otherwise unspecified |
| Ops | Real |
| Writes | removes everything `init` wrote (above) but the repository a `git init` made, and only that: the `.gitignore`/`.cursorignore` block under Daisy's header (a user's own line that is also an entry stays), Daisy's allow rules (those of the root as `$DAISY_ROOT` names it now, and the legacy forms; the file goes only when nothing else is left in it), the skills init installed, the root's among them (those whose `.cursor/rules/<name>.mdc` is still a regular file in init's stub form, whatever the home holds now), the directories init writes into when they are left empty (below); unsets each of the three git keys when every value it holds is the recorded identity's (the home's identity file stands in for the record for a workspace bound before records existed) or is a `core.sshCommand` in Daisy's exact form (`ssh -i <path> -o IdentitiesOnly=yes`, which nothing but an init writes — taken with or without a record); a hand-set value is the user's and stays; a key it unsets gets back what it held before init (`.daisy/gitconfig.before`), so init then clean leaves the keys and `git status` as they were [76], but for what init keeps no record of: an ignore file with no final newline keeps the one init gave it; an ignore file that was empty before init, or a settings file that held nothing (`{}`) before it, is removed; a settings file it takes rules out of is re-indented; the directories init writes into (`.cursor`, `.cursor/rules`, `.claude`, `.claude/commands`, `.claude/skills`, `.agents`, `.agents/skills`, `.agents/skills/daisy`) go when empty, whether init made them or they were there before; reclaims `.daisy.stage.*` leftovers |
| Commits | none |
| Cases | none of its own; exercised by cases 76/77/79 (`init`'s) and 78; the inverse is `TestInitThenCleanRestores`; the stamp `.daisy/version` goes with `.daisy/` (`TestInitStampsTheWorkspace`) |

### daisy status

| Field | Value |
|---|---|
| Status | as built (Step 17): Go; `--json` since Step 10 of the plan remediating the 2026-11-15 design review |
| Legacy | `daisy status` → built in to `daisy.sh` (`cmd_status`), deleted in Step 24 |
| Usage | `daisy status [--json]` |
| Exit codes | 0; 1 outside a workspace or when the home does not resolve (`cannot be a home` [68], `not registered` [68]); 2 with any positional argument (the bash ignored them) |
| Stdout | line 1 `Daisy \| home: <home> \| identity: <identity> \| workspace: <path>` (asserted below; `home: second` [68], `workspace: ` [78]); `Projects: <+tags> (from .daisy/project)` / `(from directory name)` or `Projects: (none)` (`+chessgrid` [45]); blank; `Tasks:  N pending, N completed today`, `Log:    N entries`, `Backlog: N tasks in todo.txt`; `Plan:   <title>` and its path when `PLAN.md` is a symlink; or `  No today.md — run 'daisy new-day' to start.`. With `--json`: one document (below) |
| Stderr | drift warnings (Common behaviour); the stale-stamp notice, exit still 0, when the binary carries a commit (`daisy version`'s) and `.daisy/version` is not it: `⚠️  The workspace was initialized by Daisy <stamp>, not this one (<commit>): run 'daisy init' here to refresh it`, or, with no stamp (a workspace bound before init wrote one), `⚠️  The workspace has no .daisy/version stamp, so it was initialized before this Daisy (<commit>): run 'daisy init' here to refresh it`; none when the stamp is the commit, or the binary carries none (`TestStatusNoticesAStaleStamp`); empty on a clean, current binding; with `--json`, empty, the warnings under `warnings` |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 45; `TestStatusCountsAndPlan`, `TestStatusNoticesAStaleStamp`, `TestReadOnlyJSON` |

The first line is asserted with the pipes: `home: testhome | identity: testhome` [76]
and `home: testhome | identity: pelicanmoor` [77].

`status --json` (Common behaviour, `--json`) has `home`, `identity` and
`workspace`, the first line's three; `workspace_projects`, the Projects
line, as `files --json` gives it; `today`, `{"pending", "completed",
"log_entries"}`, `null` without `today.md`; `backlog`, the open tasks in
`todo.txt`, `null` without it; `plan`, `{"title", "path"}`, `null` when
`PLAN.md` is not a link; and `warnings`. The text stops at a missing
`today.md`; the document still gives `backlog` and `plan`:

```json
{
  "home": "work",
  "identity": "work",
  "workspace": "/srv/ws/chessgrid.260807",
  "workspace_projects": {"names": ["chessgrid"], "source": "convention"},
  "today": {"pending": 2, "completed": 1, "log_entries": 2},
  "backlog": 2,
  "plan": {"title": "The Plan", "path": "/srv/homes/work/plans/261116.the-plan.plan.md"},
  "warnings": []
}
```

### daisy files

| Field | Value |
|---|---|
| Status | as built (Step 17): Go; `<name>` since Step 6 of the plan remediating the 2026-11-15 design review, `--json` since its Step 9 |
| Legacy | `daisy files` → `files.sh`, deleted in Step 24 with its `--healthcheck` hook (it checked only that the home resolves; `files` has none) |
| Usage | `daisy files [--json\|<name>]` |
| Arguments | `<name>`, one of `identity`, `today`, `journal`, `todo`, `done`, `alias`, `projects`, `projects-archive`, `feedback`, `plans`, `plans-archive`, `plan`, `metrics`, `prompts`, `agents`, in the listing's order: one per row it always prints (the identity file, `today.md`, `journal.md`, `todo.txt`, `done.txt`, `alias.txt`, projects `active` and `archived`, `feedback.md`, `plans/` and `archive/`, `metrics.csv`, the home's `prompts/`, `AGENTS.md`) and `plan`, its `current plan`; each its file's stem or its directory's, `-archive` after the directory an archive sits in; a row whose name varies or that is printed only when its file exists (the journal's windows, `pre-rotation`, the feedback `archive.md`) has none |
| Exit codes | 0 (a project that does not exist is not fatal, case 44); 1 outside a workspace, and for `plan` when the workspace has no current plan: no `PLAN.md` (`Error: no current plan: this workspace has no PLAN.md` and `  Start one: daisy plan new <description> [+project]`), a `PLAN.md` that is not a link (move it aside, then the same), a broken link (remove it, then the same; when the binding link `.daisy/plans` has drifted, that drift is named first, with `Re-run 'daisy init <identity>'`, since `PLAN.md` reaches the home through it; another link's drift is the listing's to report), or a link to a directory or to a file not named `*.plan.md` (`Error: no current plan: <ws>/PLAN.md links to <target>, which is not a plan file (*.plan.md)`, then remove it, then the same) *(decided: a well-formed request the workspace cannot answer is a failure, Common behaviour's 1, not a usage error)*; 2 for a name not in the list (`Error: unknown name: <name>; one of <the names, in the order above>`, then the usage), a second argument, or `--json` with a name (`Error: --json lists every path; with <name>, its path alone is printed`, then the usage: the one path is already the whole output) |
| Stdout | with `<name>`: that row's real path and a newline, nothing else, whether or not the file exists yet; for `plan`, the plan file `PLAN.md` resolves to (`TestFilesNamePrintsOnePath`). Without: the same first line as `status` (`identity: testhome` [76], `identity: pelicanmoor` [77]); sections `Identity:` (the resolved identity file: the home's `gitconfig` [76] or `identities/pelicanmoor` [77], ` (missing)` when absent), `Journal:`, `Tasks:`, `Projects:` (`workspace` row: `+chessgrid` [43] `(from ` `directory name` [43] `)`, `+chessgrid +jobsearch` [43] in file order, `+jobsearch` [42] `(from ` `.daisy/project` [42] `)`, or `(none)` (`none` [44])), `Feedback:`, `Plans:` (with `current plan` when `PLAN.md` links to a plan file, as `files plan` finds one), `Metrics:` (`metrics.csv`, which `metrics add` writes, listed whether or not it exists yet), `Prompts:`; each row `  %-12s <real path>`; last `AGENTS.md:    <real path>` (`%-14s`). With `--json`: the listing as one document (below) |
| Stderr | with `--json`: empty on success, drift and a missing project's warning under `warnings`. With `<name>`: empty on success, even when the binding has drifted (every name's path but `plan`'s comes from the home, not through the `.daisy/` links; the listing reports drift); the error on a failure. Without: empty when the convention default matches nothing (asserted `""`, case 44); a declared project with no file is named (`typoed-name` [44]); drift warnings |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 42, 43, 44; `TestFilesListsRealPaths`, `TestFilesNamePrintsOnePath`, `TestFilesJSON` |

`files --json` (Common behaviour, `--json`) has `home`, `identity` and
`workspace`, the first line's three; `paths`, every `<name>` above and
its real path, `files <name>`'s (`identity`'s too when the file is
missing, `identity_missing` then `true`, the text's `(missing)`), but
`plan`, `null` wherever `files plan` finds no current plan (no
`PLAN.md`, one that is not a link, a broken link, a link to a directory
or to a file not named `*.plan.md`); `pre_rotation` and `feedback_archive`, the rows printed
only when their file exists, `null` when it does not; `journal_windows`,
the window rows in the text's order, each `{"name", "path"}` since its
name varies; `workspace_projects`, `{"names", "source"}`, `source` one
of `explicit` (`.daisy/project`), `convention` (the directory name) and
`none`; and `warnings`. Abridged:

```json
{
  "home": "work",
  "identity": "work",
  "workspace": "/srv/ws/chessgrid.260807",
  "paths": {
    "identity": "/srv/homes/work/gitconfig",
    "today": "/srv/homes/work/journal/today.md",
    "todo": "/srv/homes/work/tasks/todo.txt",
    "plan": null,
    "agents": "/srv/homes/work/AGENTS.md"
  },
  "identity_missing": false,
  "pre_rotation": null,
  "journal_windows": [
    {"name": "journal-2025.md", "path": "/srv/homes/work/journal/journal-2025.md"}
  ],
  "feedback_archive": null,
  "workspace_projects": {"names": ["chessgrid"], "source": "convention"},
  "warnings": []
}
```

### daisy list

| Field | Value |
|---|---|
| Status | as built (Step 20): Go; `--json` since Step 10 of the plan remediating the 2026-11-15 design review |
| Legacy | `daisy list` → the bash `list` script, deleted in Step 20 |
| Usage | `daisy list [--full] [--json]` (`--full` prints each skill's `description` instead of `short_description`; not in the help line, kept as an unasserted flag) |
| Exit codes | 0; 1 outside a workspace, for an `include.txt` that cannot be read or a line of it with an unmatched quote (where the bash died mid-listing), or a prompt or SKILL.md that cannot be read, stdout then holding the listing read before the failure (`TestListPrintsWhatItReadBeforeAFailure`; with `--json`, nothing); 2 with any other argument |
| Stdout | `Prompts (<home>):` then one `  %-20s <trigger summary or (loaded with .daisy/AGENTS.md)>` per `include.txt` entry (the width in bytes, as the bash's `printf`), `(missing: <path>)` for an absent prompt, `(no include.txt found)`; the summary is the trigger's first line that is not `Read the full…` and not empty once a leading `- ` is dropped, cut to 97 characters and `...` past 100, else `(lazy, no trigger summary)`; a home override then ` [layered]`, ` [full]` or ` [full, behind]` (the root changed since the copy, `feedback/.overrides`) [88], skills too; blank; `Skills:` then one `  %-20s <text>` per installed skill, user-level (`~/.claude/skills`) then workspace (`.claude/skills`), workspace overriding by name, in byte order of name (the bash's `sort` collated by locale); `<text>` is `short_description`, else `description`, one surrounding `"` dropped; or `(none installed — run: daisy init <home>)`. With `--json`: one document (below) |
| Stderr | the refusals above |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 85; `TestList`, `TestListPrintsWhatItReadBeforeAFailure`, `TestReadOnlyJSON` |

`list --json` (Common behaviour, `--json`) has `home`; `prompts`, one
per `include.txt` entry in its order, `null` without `include.txt`, each
`name`, `lazy`, `trigger` (a lazy prompt's summary as the text prints
it, `null` for one without and for a prompt included in full), `missing`
(the path looked up, `null` when found), `override` (`layered`, `full`
or `null`) and `behind` (`true` for ` [full, behind]`); `skills`, in the
text's order, each `name`, `description` (the text's, `--full`
included), `override` and `behind`; and `warnings`:

```json
{
  "home": "work",
  "prompts": [
    {"name": "daisy", "lazy": false, "trigger": null, "missing": null, "override": null, "behind": false},
    {"name": "plan", "lazy": true, "trigger": "When the user says plan.", "missing": null, "override": "full", "behind": true}
  ],
  "skills": [{"name": "testing", "description": "Claims, seams, fakes, probes", "override": null, "behind": false}],
  "warnings": []
}
```

### daisy help

| Field | Value |
|---|---|
| Status | as built (Step 20): Go. The tree (`daisy`, `daisy help`, `--help`, `-h`) is rendered from the `cli` table; so, since Step 24, is AGENTS.md's Command Reference (`--markdown`) |
| Legacy | `daisy help [<command>\|<skill>]` → built in to `daisy.sh` (`show_help`, `show_command_help` and the skill lookup), deleted in Step 24 |
| Usage | `daisy help [<command>\|<skill>\|--markdown]`; `daisy`, `daisy --help`, `daisy -h` are `daisy help` |
| Exit codes | 0; 1 for a name that is neither (`Unknown command or skill: <name>` and `Run 'daisy help' for commands, 'daisy list' for skills.` on stderr; the bash died silently there) or outside a workspace for a skill; 2 for surplus words |
| Stdout | no argument: the tree. A command name (its words: `help prompt build`): exactly what `daisy <command> --help` prints, the block Common behaviour's `--help` states, from the table — then, after the whole block, when a skill of that name exists (installed or source; none looked up outside a workspace), a note — the command's help, not the skill's (not `Collide politely` [52]), and the word `skill` [52]: blank, `Note: a skill named '<name>' also exists; the command name takes precedence.`, `  Read it with: daisy list, or open its SKILL.md directly.` A noun phrase, as the dispatcher reads one (`help prompt`, `help swarm`, `help swarm prompt`; a command's whole name wins, `help swarm prompt build`): the usage of each command under it, the skill note only for a one-word noun; a word past the longest phrase or name is surplus, exit 2 (`help swarm prompt x`). A skill name: `<name>  (<SKILL.md path>)`, blank, the full `description:` value (`when to stop making them` [51]; the line after `description: `, `: ` inside it kept), blank, the `## Rules` section up to the next `#` or `##` heading, `optimize`'s boundary (`Measure before cutting` [51]; until Step 23 an `# ` heading and what followed it to the next `## ` were printed as rules), `Other sections: <H2 titles>` joined by `, ` (`Worked example` [51]). The installed copy (workspace over user) is preferred over the source (home over root); a home source that is a layered override is shown merged (`TestSkillOverrides`). `--markdown` as the only word: the Command Reference block for AGENTS.md — the BEGIN marker line, a `\| Group \| Command \| What it does \|` table with one row per table command in table order (each usage in backticks after `daisy `, `\|` escaped), the END marker line — which `healthcheck` compares with the root's AGENTS.md (6b) |
| Ops | ReadOnly (the skill lookup and the note) |
| Writes | none |
| Commits | none |
| Cases | 51, 52 |

## Day

### daisy new-day

| Field | Value |
|---|---|
| Status | as built (Step 19): Go; the `optimize --list` nudge removed with `--list` (D30, built in Step 23); the template resolved home-first (D48, Step 36) |
| Legacy | `daisy new-day` → the bash `new-day` script, deleted in Step 19 |
| Usage | `daisy new-day [--quick] [--force]` (no argument; any is usage on stderr, exit 2 — the bash ignored them) |
| Period started | a start rotates only into a period not yet started: when `today.md`'s first `### YYYY-MM-DD` heading is today or later, the day has started, and `new-day` prints `✅ Day already started: today.md is dated <YYYY-MM-DD>; nothing rotated`, writes nothing and commits nothing (exit 0), before the template and `todo.txt` are looked for (after the health check). A `today.md` that is absent or has no such heading starts the day. A `daisy log` quick start is such a start, so the New Day workflow's `daisy new-day` after one finds the day started (`TestStartSkipsAPeriodStarted`). `--force` skips this check and starts the day all the same, as any start does: `today.md` is archived into `journal.md` though it is dated today, so the journal then holds two blocks of that date, the earlier one kept as it was written, and the new `today.md` is the template's; one commit, `New day: <date> <day>` (`TestNewDayForce`). With `--quick` as well, `--quick` decides *(Step C9 of the plan implementing the 2026-11-19 swarm roadmap: a forced start archives the block of the same date rather than replacing the journal's earlier one, which would delete a day's log)* |
| Quick start | `--quick` starts the largest period not yet started: when the day has started it prints the line above, writes nothing and commits nothing (exit 0; it printed `✅ Day already started: <YYYY-MM-DD> <Day>`, the clock's date, before Step 30b). Otherwise it starts a new week when that date is in an earlier ISO week than today, else a new day, whichever of `new-day` and `new-week` was run; a `today.md` that is absent or has no such heading starts the command's own. Before the start's own lines it prints `⚡ Quick start: today.md was <YYYY-MM-DD>; retrospective not checked` (a day) or `…; retrospective not checked, daisy optimize not run` (a week), without `today.md was <date>; ` when there was no date: what the agent's New Day and New Week workflows do around the command and this start did not (`TestQuickStart`) |
| Exit codes | 0; 1 when `healthcheck` fails first (`Error: System health check failed` on stderr), when `tasks/todo.txt` is missing, when neither `<home>/templates/journal-day.md` nor `$DAISY_ROOT/templates/journal-day.md` is a file (`Error: Template missing: journal-day.md` [87] and `  Looked for: <home>/templates/journal-day.md, then <root>/templates/journal-day.md` [87]; the `--healthcheck` hook checks the same resolution), when the journal cannot be rotated (`rotate`'s refusal), or when the commit fails. A refusal writes nothing: every write is decided before the first is made (the bash had appended `today.md` to `journal.md` before a rotate refusal, so the next run archived it twice) |
| Stdout | `📦 Archived yesterday to journal.md` (when `today.md` existed) [81], `rotate`'s line [81], `🗑️  Deleted N cancelled task(s)` (when any) [81], `✅ New day started: <YYYY-MM-DD> <Day>` [81], `   High priority tasks: N` [81], `   Next priority tasks: N`, `   Inbox tasks: N`, `   GitHub tasks: N` [81], then `commit`'s `📝 Committed: …` line (git's own commit summary, which the bash let through, is not printed). Nothing follows the commit line: the pending-feedback nudge went with `optimize --list` at Step 23 — the New Week workflow runs `daisy optimize` itself |
| Ops | Real; the health check runs in-process first (`internal/health`, output discarded; a cached pass or `DAISY_HEALTHCHECK_PASSED=1` skips it), since Step 24, and then — never skipped — the home must be its own repository, before anything is written |
| Writes | the rotation window files and `journal/journal.md` (see `rotate`; `today.md` is appended after a `---` separator when the journal is non-empty, and rotated with it); `tasks/todo.txt` without its `z ` lines [81], every line newline-terminated; `journal/today.md` from the template, the home's `templates/journal-day.md` when it is a file, else the root's [87] (the order `prompt build` looks up prompts in): `{DATE}`, `{DAY}`, `{TIME}`, then `{HIGH_PRIORITY_TASKS}` = `(A)` [81], `{NEXT_PRIORITY_TASKS}` = `(B)` [81], `{INBOX_TASKS}` = dated without priority [81], `{GITHUB_TASKS}` = any open line containing `@git`/`@github`, whatever its priority, decided before the priority branches [81]; `(C)`/`(D)` lines without such a tag are not carried [81]; each carried line loses its priority and creation date when it has a date, and is carried whole otherwise — and a dated `@git` line whose priority is outside A–D (`(E) 2026-07-24 … @git`) is carried whole too, as the bash did (`TestCarry`). A task's text is written as it is (bash 5.2's pattern substitution replaced an `&` in it with the placeholder's name) |
| Commits | home, once: `New day: <date> <day>` [72, 81] |
| Cases | 81, 87; case 72 (`commit`'s) runs it |

### daisy new-week

| Field | Value |
|---|---|
| Status | as built (Step 19): Go; nudge removed as for `new-day` (D30, built in Step 23); the template resolved home-first as for `new-day` (D48, Step 36) |
| Legacy | `daisy new-week` → the bash `new-week` script, deleted in Step 19 |
| Usage | `daisy new-week [--quick] [--force]` (no argument; `--quick` and `--force` as `new-day`'s) |
| Period started | as `new-day`'s, by the ISO week (Monday first): when `today.md`'s first `### YYYY-MM-DD` heading is in today's ISO week or later, the week has started. When the day has started too (`today.md` dated today or later), `new-week` prints `✅ Week already started: today.md is dated <YYYY-MM-DD>; nothing rotated`, writes nothing and commits nothing (exit 0). When it has not (`today.md` dated earlier in this week), `new-week` starts the day: it prints `✅ Week already started: today.md is dated <YYYY-MM-DD>; starting the day`, then `new-day`'s start, its template, its output and its commit, `New day: <date> <day>`. So too when the week's start so far was a day's (a `new-day` earlier in the week): its completed tasks stay in `todo.txt` and `today.md` keeps the day template until the next week's start (`TestStartSkipsAPeriodStarted`). `--force` skips the check and starts a week all the same, completed tasks to `done.txt` and the week template, archiving `today.md` as `new-day --force` does (`TestNewWeekForce`) |
| Exit codes | as `new-day`; the template is `journal-week.md`, the home's `templates/` first, then the root's [87] |
| Stdout | as `new-day`, plus `📦 Archived N completed task(s) to done.txt` (when any) [82] after the cancelled line, and `✅ New week started: <date> <day>` [82] |
| Ops | Real |
| Writes | as `new-day`, plus `tasks/done.txt`: the `x ` lines of `todo.txt` appended [82] (after a newline when `done.txt` lacked a final one and something is archived — the bash glued the first onto its last line; with nothing archived the file is left as it was) and removed from `todo.txt` [82]; `today.md` from `templates/journal-week.md` [82], the home's before the root's |
| Commits | home, once: `New week: <date> <day>` [82] |
| Cases | 82; case 87 (`new-day`'s) runs its template refusal |

### daisy log

| Field | Value |
|---|---|
| Status | as built (Step 18): Go |
| Legacy | `daisy log <message...>` → the bash `log` script, deleted in Step 18 (the dispatcher joined the arguments with single spaces: `"$*"`; the binary does the same) |
| Usage | `daisy log <message...>` |
| Quick start | when `today.md`'s first `### YYYY-MM-DD` heading is before today, `log` runs `new-day --quick` first (its lines, its refusals, its own commit), then adds the entry to the new `today.md`; an absent or undated `today.md` is refused as below, not started (`TestLogQuickStartsTheDay`) |
| Exit codes | 0; 2 with no message — no argument, or only empty ones (usage on stderr; the bash exited 1 with `Error: Log message required`); 1 when `healthcheck` fails first (`Error: System health check failed`, `  Run: daisy healthcheck`), when `today.md` is missing (`Error: today.md not found`) or has no `#### Log` section (`Error: Log section not found in today.md`) |
| Stdout | a quick start's lines when one ran, then `✅ Logged: <HHMM> <message>` then `commit`'s line (`done` suppresses both: not `Logged:` [15]) |
| Ops | Real; the health check runs in-process first, as for `new-day` (the bash's `healthcheck.sh` through `Ops.Exec` until Step 24) |
| Writes | `journal/today.md`: appends `- HHMM message` to the `#### Log` section — the lines from the heading to the next heading of any level or the end of the file — after its last non-blank line, so blank lines before the next heading stay; a section with no entries is written in the template's shape (heading, blank line, entry, blank line before the next heading), which healthcheck Check 5 requires [80]. *(decided, D33, Step 18: the bash inserted at the first blank line after the section's first entry without stopping at the next heading, so a Log section abutting the next heading sent the entry into a later section)* |
| Commits | home, once: `Log: <HHMM> - <first 60 characters>`, after the quick start's own `New day:`/`New week:` commit when one ran (case 72; case 71 pins that a home inside the root repository is refused before anything is written) |
| Cases | 80; cases 71/72 (`commit`'s) and 76/78 (`init`'s) run it |

### daisy done

| Field | Value |
|---|---|
| Status | as built (Step 18): Go |
| Legacy | `daisy done <pattern>` → the bash `done` script, deleted in Step 18 |
| Usage | `daisy done <pattern>` (one argument) |
| Arguments | `<pattern>`: literal, case-insensitive substring (`.` is not a wildcard [08]; `/` is safe [07]; a leading `-` is a pattern, not a flag — except the sole-argument `--help`/`-h` and `--healthcheck`, which the cli takes first, and a word shaped like a long flag, `--dry-run`, an unknown flag (Common behaviour); `--dry-run support` is a pattern) matched against the active lines of `todo.txt` only (`x `/`z ` lines excluded before matching, cases 05, 06). Missing — no argument, or an empty one — or more than one: usage on stderr, exit 2 (the bash exited 1 with `Error: Task pattern required` and ignored extra arguments) |
| Exit codes | 0 exactly one match completed [01, 02, 05–08, 10–15]; 1 no match [04, 16], several matches [03], an Inbox-only item that is not in `todo.txt` [09], `healthcheck` failing first, or the journal entry failing after the edits [17] |
| Stdout | success: exactly one non-blank line [15], `✅ Done: <description> (commit <hash>)` — names the task (`Delta target task` [15]) and suppresses the sub-calls' chatter (not `Logged:` [15], `Committed:` [15], `No changes to commit` [15]); nothing with `✅` [16] when nothing completed; no `✅ Done` [17] when journaling failed |
| Stderr | `No task found matching:` [04] `<pattern>`; `Multiple tasks match "<pattern>":` and each candidate on its own `  N) <line>` (`Shared token Alpha` [03], `Shared token Beta` [03]); `⚠️  Task not found in today.md (may not be in active tasks)` when `todo.txt` is updated but today.md holds the task in neither form the Writes row ticks (`today.md` [11]; exit still 0); on a journal failure `Error: journal entry failed — …` naming whether the edits were committed, then the sub-calls' output (`Log` [17]). Cases 03, 04, 11, 17 assert on stdout and stderr combined, so the stream is not pinned |
| Ops | Real; the health check runs in-process first, as for `new-day` (the bash's `healthcheck.sh` through `Ops.Exec` until Step 24) |
| Writes | `tasks/todo.txt`: the one matched line removed and `x <today> <creation date> <description>` written after the other lines, before the first cancelled (`z `) one, else last (`docs/task-format.md`: completed before cancelled; `TestDoneCompletesAndCommitsOnce`), no priority [10] — the line read by `todo.Parse`, so an undated task completes as `x <today> <description>` with one space and its priority dropped (the bash left the priority in the description and wrote two spaces); `journal/today.md`: the first line equal to `- [ ] <the line as new-day carries it>` (an undated `(A)`/`(B)` line whole, `TestDoneWithoutTheTaskInToday`), else the first equal to `- [ ] <description>`, gets its `[ ]` made `[x]`, every other byte unchanged [01, 02, 12]; then the log entry `Done: <description>` in-process (no `log` sub-call) |
| Commits | home, exactly once for the whole completion, holding both files [13, 14], subject `Completed: <description>` *(decided, D33, Step 18: the bash's one commit was `log`'s, `Log: HHMM - Done: …`, and its own `Completed:` commit always found nothing)*; a failed journal entry still leaves the edits committed [17]; a failed commit after a successful completion is an error, exit 1 (the bash printed `(commit not committed)` and exited 0) |
| Cases | 01-18 |

### daisy add

| Field | Value |
|---|---|
| Status | as built (Step 31 of the plan remediating the 2026-11-15 design review, talk A14, P26): Go; before it the agent edited both files by hand |
| Usage | `daisy add <description> A\|B\|C\|D\|none` (two arguments) |
| Arguments | `<description>`: the task's text as todo.txt holds it, `@context`, `+project` and `due:YYYY-MM-DD` words included, verbatim; `<priority>`: `A`, `B`, `C`, `D`, or `none` for a task without one (the inbox), exactly so (`a`, `(A)`, `inbox` are not priorities). Asking the user for a priority they did not give is the agent's (`prompts/daisy.md`, Add Task), never the command's. Missing, empty or surplus words, or another priority (`Error: not a priority: <word>; give A, B, C, D or none`): usage on stderr, exit 2 |
| Exit codes | 0 the task added and committed; 1 a description that starts with a priority (`Error: the description starts with a priority, (A); give it as the priority argument: daisy add "<rest>" A`, for `(A)`–`(D)`; another letter, which the argument does not take, `Error: the description starts with a priority, (E), which add does not take; give A, B, C, D or none as the priority argument: daisy add "<rest>" A|B|C|D|none`, `TestAddTaskRefusals`), holds a line break (`Error: a task is one line; the description holds a line break`) or is blank (`Error: the description is blank`), `healthcheck` failing first, `todo.txt` unreadable, or the commit failing (`Error: todo.txt and today.md are written, but the commit failed`); a refusal writes nothing |
| Stdout | `✅ Added: <the todo.txt line>; today.md: <outcome> (commit <hash>)`, one line, the outcome `added to <section>`, `not added, the next new day carries it to <section>` (an inbox task: `Inbox`; any task with `@git`: `GitHub PRs`), `not added, the next new day does not carry it` (`(C)`, `(D)` without `@git`), or `unchanged` with the warning below (`TestAddTask`) |
| Stderr | `⚠️  today.md not found; only todo.txt changed`, `⚠️  today.md has no **<section>:** section; only todo.txt changed` (an (A) or (B) task; exit still 0, `TestAddTaskWithoutTodaysSection`) |
| Ops | Real; the health check runs in-process first, as for `done` |
| Writes | read, then decided, then written: `tasks/todo.txt`, the line `(<P>) <today> <description>` (`<today> <description>` for none) after the last open line that sorts at or before it, in todo.txt's order — `(A)`, `(B)`, no priority, `(C)`, `(D)`, then `(E)`–`(Z)` (docs/task-format.md, File Organization) — at the top when none does; a closed or blank line is never the anchor, so in a file out of that order the task still follows every line that should come first (`TestAddTaskPlacesByPriority`); for an `(A)` or `(B)` task, `journal/today.md`: `- [ ] <description>` at the end of the section a new day would carry it to (`(A)` Now, `(B)` Next, a task with `@git` GitHub PRs: `journal.Place`), after the section's last non-blank line, every other byte unchanged; a `(C)`, `(D)` or inbox task leaves today.md as it is |
| Commits | home, once, holding what it wrote: `Added: (<P>) <description>` (`Added: <description>` for none) |
| Cases | none; `TestAddTask`, `TestAddTaskPlacesByPriority`, `TestAddTaskRefusals`, `TestAddTaskWithoutTodaysSection` |

### daisy cancel

| Field | Value |
|---|---|
| Status | as built (Step 31 of the plan remediating the 2026-11-15 design review, talk A14, P26): Go; before it the agent edited both files by hand |
| Usage | `daisy cancel <pattern>` (one argument) |
| Arguments | `<pattern>`: as `done`'s, resolved by the same rule (one open line of `todo.txt`; `x `/`z ` lines never candidates) |
| Exit codes | 0 exactly one match cancelled and committed; 1 no match (`No task found matching: <pattern>`), several matches (`Multiple tasks match "<pattern>":` and each `  N) <line>`), `healthcheck` failing first, `todo.txt` unreadable, or the commit failing; 2 usage, as `done` |
| Stdout | `✅ Cancelled: <description> (commit <hash>)`, one line |
| Stderr | `⚠️  Task not found in today.md (may not be in active tasks)` when today.md holds the task in neither form `done` ticks (exit still 0) |
| Ops | Real; the health check runs in-process first, as for `done` |
| Writes | read, then decided, then written: `tasks/todo.txt`, the matched line removed and `z <today> <creation date> <description>` appended last, no priority, where docs/task-format.md keeps cancelled lines until the next `new-day` or `new-week` deletes them (docs/test-cases.md, Test Case 3); `journal/today.md`: the first line `- [ ] <the line as new-day carries it>`, else the first `- [ ] <description>`, gets its `[ ]` made `[z]`, every other byte unchanged. No log entry |
| Commits | home, once: `Cancelled: <description>` |
| Cases | none; `TestCancelTask`, `TestCancelAndPriorityRefusals` |

### daisy priority

| Field | Value |
|---|---|
| Status | as built (Step 31 of the plan remediating the 2026-11-15 design review, talk A14, P26): Go; before it the agent edited both files by hand |
| Usage | `daisy priority <pattern> A\|B\|C\|D\|none` (two arguments) |
| Arguments | `<pattern>`: as `done`'s, resolved by the same rule; `<priority>`: as `add`'s |
| Exit codes | 0 the priority changed and committed, or the task already has it (`✅ <description> is already <priority>; nothing changed`, nothing written or committed); 1 no match, several matches (as `cancel`), `healthcheck` failing first, `todo.txt` unreadable, or the commit failing; 2 usage, as `add` |
| Stdout | `✅ Priority: <description> <old> → <new>; today.md: <outcome> (commit <hash>)`, one line, each priority `(<P>)` or `none`, the outcome `moved to <section>`, `removed, the next new day does not carry it`, `added to <section>`, `not in it, the next new day carries it to <section>`, `not in it, the next new day does not carry it`, or `unchanged` with `add`'s warning (`TestPriorityTask`) |
| Stderr | `add`'s warnings, when the task's new section cannot be written |
| Ops | Real; the health check runs in-process first, as for `done` |
| Writes | read, then decided, then written: `tasks/todo.txt`, the matched line removed and written again with the new priority, its creation date and text kept, at `add`'s place for that priority; `journal/today.md`: the task's line, found as `cancel` finds it, removed and written at the end of the section a new day would carry the new line to (`journal.Place`: `(A)` Now, `(B)` Next, a dated task without a priority Inbox, `@git` GitHub PRs), with the text a new day would write (an undated line carried whole changes with its priority), or only removed when a new day would not carry it (`(C)`, `(D)` without `@git`); a task not in today.md is added there when it becomes `(A)` or `(B)`, and otherwise left out; when the new section is missing, today.md is left as it was |
| Commits | home, once: `Priority: <description> <old> → <new>` |
| Cases | none; `TestPriorityTask`, `TestCancelAndPriorityRefusals` |

### daisy tasks

| Field | Value |
|---|---|
| Status | as built (Step 18): Go; `--json` since Step 10 of the plan remediating the 2026-11-15 design review |
| Legacy | `daisy tasks [--all] [--todo\|--done] [pattern]` → the bash `tasks` script, deleted in Step 18 |
| Usage | `daisy tasks [--all] [--todo\|--done] [--json] [pattern]` |
| Arguments | scope: this workspace's projects by default — a line is in scope when one of its `+tag` words (whitespace-delimited, compared without case) is a workspace project — `--all` every project [19, 46, 47]; files: both by default [22], `--todo` = `todo.txt` [20], `--done` = `done.txt` [21], the last of the two given wins; `[pattern]`: literal, case-insensitive substring [24] (an empty pattern is none), a leading `-` is a pattern (`-1 point deduction task` [25]) once the three flags above have been consumed — except the sole-argument `--help`/`-h` and `--healthcheck`, and a word shaped like a long flag, an unknown flag (Common behaviour); flags before the pattern; a second positional is a usage error, exit 2 |
| Exit codes | 0 when at least one line is printed [19–25, 27, 29, 46–48], and with no pattern whatever the scope matched; 1 when a pattern is given and matches nothing, with empty stdout [26] and `No tasks match: <pattern>` on stderr; 1 when none of the selected files exists (`Error: no task file to search: …`); a missing `done.txt` alone is not an error [27] |
| Stdout | the matching lines verbatim and nothing else — no header (`Active projects` [29]), no decoration [29], blank lines skipped; every task line when no pattern is given [23]. With `--json`, `{"tasks": [<line>, …], "warnings": [...]}`, the same lines in the same order, `[]` for none (a pattern matching none is the failure above) |
| Stderr | when the workspace resolves to no project and `--all` was not given: `⚠️  This workspace resolves to no project; listing every project's tasks.` (`no project` [48]) and `  Bind it by creating .daisy/project, or pass --all to ask for this deliberately.`; the `.daisy/project` typo warnings, as `status` prints them; with `--json`, empty on success, these lines under `warnings` |
| Ops | ReadOnly (`todo.txt` and `done.txt` byte-identical and no commit after a query [28]) |
| Writes | none |
| Commits | none |
| Cases | 19-30, 46, 47, 48; `TestReadOnlyJSON` |

### daisy projects

| Field | Value |
|---|---|
| Status | as built (Step 18): Go; `--json` since Step 10 of the plan remediating the 2026-11-15 design review |
| Legacy | `daisy projects [--archived]` → the bash `projects` script, deleted in Step 18 |
| Usage | `daisy projects [--archived] [--json]` |
| Exit codes | 0; 1 outside a workspace; 2 with a positional argument |
| Stdout | `Active projects:` (or `Archived projects:` for `projects/_archive/`) then `  %-30s <real path>` per `*.md` file, in byte order; `Active projects: none` when there are none. With `--json`, `{"archived": <bool>, "projects": [{"name", "path"}, …], "warnings": []}`, the same files in the same order, `[]` for none |
| Stderr | the `.daisy/project` typo warnings, as `status` and `tasks` print them (`⚠️  .daisy/project names '<x>', but <home>/projects/<x>.md does not exist`); with `--json`, empty, these lines under `warnings` (`TestProjects`) |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | none (cases 42–44 pin project *resolution* through `files`); `TestReadOnlyJSON` |

### daisy rotate

| Field | Value |
|---|---|
| Status | as built (Step 19): Go; byte-identical to the bash on a copy of a real journal over 33 `new-day`/`new-week` runs across a month and a year boundary, and on 270 synthetic journals (D34) |
| Legacy | `daisy rotate` → the bash `rotate` script, deleted in Step 19 (also run by `new-day` and `new-week`, in-process in Go) |
| Usage | `daisy rotate` (no argument) |
| Exit codes | 0, also when `journal.md` is missing, empty or holds five day-blocks or fewer (nothing written, nothing printed) [83]; 1 when `journal.md` has non-blank content before its first `### YYYY-MM-DD` heading, but for a preamble ending in `---` with a day-block after it (`TestRotateKeepsALeadingRetrospective`) (`Error: rotate: journal.md has non-blank content before its first '### YYYY-MM-DD' heading — aborting, nothing written` on stderr [83]), when the month-to-date or year-to-date window holds no day-block (`Error: rotate: <file> has no '### YYYY-MM-DD' day-block …`; the bash died part-way there, with the last-14 window already written), when a block would move and a rolling window is not a regular file (`Error: rotate: <file> is not a regular file …`; the bash appended through a link, then replaced it with a file), or when a month or year window would close into a name the journal directory holds already, or that the run closed already — a day-block out of date order (`Error: rotate: closing the <YYYY-MM> window would replace journal-YYYYMM.md: …`; the bash's `mv` replaced it, losing its blocks; `TestRotateNeverReplacesAClosedWindow`, `TestRotateKeepsAClosedMonthFile`). Nothing is written on any refusal [83] |
| Stdout | `✅ Rotated N day-block(s); journal.md retains its last 5` when anything moved [83]; nothing otherwise [83] |
| Ops | Real |
| Writes | `journal/journal.md` keeps its last five `### YYYY-MM-DD` blocks [81, 83]; a block runs to the next such heading, less its trailing blank lines and one trailing `---`, so a preamble before a heading (new-week's Weekly Retrospective) stays with the block before it, and one before the first heading, ending in `---` (a home whose journal was empty when its first week began), stays at the top of `journal.md`, never rotated. Each older block, oldest first, is appended to all three rolling files — `journal-<YYYYMMDD>-last14.md` (which then drops its oldest block beyond 14 and takes the date of its new first block), `journal-<YYYYMMDD>-mtd.md` and `journal-<YYYYMMDD>-ytd.md`, each named by the full date of its first block [83] — and a month or year window whose first block is of another period than the incoming one closes first into `journal-YYYYMM.md` / `journal-YYYY.md` [83], never over one: a name taken is refused (Exit codes). So every rotated block is in exactly one month file and one year file, and in the last-14 window while it is among the fourteen most recently rotated (D34 restates Step 19's "exactly one window file" so) *(decided, D34, Step 19)*. Each window file carries `# <label>`, `## Table of Contents` (one date per block), `## Summary` (`N entries, <first> to <last>.` [83]), `---`, then its blocks separated by `---` |
| Commits | none of its own [83] (`new-day`/`new-week` commit) |
| Cases | 83 |

## Plans

### daisy plan new

| Field | Value |
|---|---|
| Status | as built (Step 22): Go (`internal/plan`); byte-identical to the bash's output and plan file but where the bash was wrong (D37) |
| Legacy | `plan` and `new` as one hyphenated word → its bash script, deleted in Step 22 |
| Usage | `daisy plan new [--spec] <description> [+project]` |
| Arguments | `<description>`: required — none, or only empty words, is usage on stderr, exit 2 (the bash exited 1 with `Error: Description required`); slugged (every run of bytes other than ASCII letters and digits → one `-`, trimmed, cut to at most 100 bytes and trimmed again, `fix-four-defects` [55] stays readable; the bash's `sed` kept the locale's letters, so `café` stayed `café` and 100 characters could be 200 bytes, and slugged each line of a multi-line description apart — Go's slug is ASCII whatever the locale, and a newline is one more hyphen); taken literally into the H1 (the bash's `awk -v` turned `\t` and `\\` into a tab and a backslash); `[+project]`: leading `+` optional, stripped; a third word is usage, exit 2 (the bash ignored it) |
| Exit codes | 0 [55]; 1 when `PLAN.md` already exists — as a dangling link too (`Error: PLAN.md already exists in this workspace`, then `Archive it first: daisy plan archive (or, to an agent, /daisy archive PLAN.md)`, the command and the skill's form, `TestPlanNewCreatesAndLinks`; the bash named the skill's form alone, wrote the plan file, then failed at `ln`) —, or with `--spec` when `<slug>_PLAN.md` does, when the slug is empty (`Error: description produced an empty slug`), when the project holds `/` or a NUL (`cannot be part of a file name`; the bash failed at the write) — with `--spec` too, where it names no file yet but `plan pickup` would refuse it, when the plan file already exists (two plans named in the same second; the bash overwrote it), when `templates/plan.md` is missing, when `.gitignore` cannot be read, or when the filename would reach 255 bytes: `the limit is 255` [55] and `Shorten the project tag` [55] on stderr, never the shell's `File name too long` [55]. Every refusal is decided before the first write; a failed write takes back the plan file and link already made (`Nothing was created.`) |
| Stdout | `  ✓ Added PLAN.md to .gitignore` / `  ✓ Created .gitignore with PLAN.md` when it was not ignored, then `✅ Plan created: <filename>`, `   Linked as: <ws>/PLAN.md`; `--spec`: the `*_PLAN.md` gitignore line, then `✅ Spec created: <slug>_PLAN.md`, `   Location: <path>`, `   Not registered with Daisy — promote later with: daisy plan pickup <path>` |
| Ops | Real |
| Writes | `$DAISY_HOME/plans/<yymmddHHMMSS>.<project>.<slug>.plan.md` (no project: `<ts>.<slug>.plan.md`) from `templates/plan.md`, line by line: the `{Description}{project_suffix}` line becomes `# <description>` (` — +<project>` added when there is one), the `**Project:** +{project}` line names the project or goes, the first `{YYYY-MM-DD HH:MM}` on a line becomes the time, and on any other line every `$DAISY_ROOT` the root; the relative symlink `<ws>/PLAN.md` → `.daisy/plans/<filename>`; `<ws>/.gitignore` gains a `PLAN.md` line (on a line of its own: the bash appended it to a last line without a newline), written through a link, unless a `.gitignore` of the repository already ignores the name, as for `init`'s entries (`TestPlanNewAddsNoLineGitAlreadyHas`, `TestBindingTheRootLeavesItAsGitHasIt`). `--spec`: `<ws>/<slug>_PLAN.md` only, `.gitignore` gains `*_PLAN.md` on the same terms, nothing under the home |
| Commits | none (decided, D37: the plan file is committed by the next home commit, `plan pickup`'s and `plan archive`'s included) |
| Cases | 55 |

### daisy plan pickup

| Field | Value |
|---|---|
| Status | as built (Step 22): Go (`internal/plan`) |
| Legacy | `plan` and `pickup` as one hyphenated word → its bash script, deleted in Step 22 |
| Usage | `daisy plan pickup <path>` (a relative path is the working directory's; the bash's was the workspace root's) |
| Exit codes | 0; 2 with no path or a second one; 1 for a path that is not a regular file (`Error: File not found: <path>`), an existing `PLAN.md` (a dangling link included; `plan new`'s two lines), no `# <Description>` H1 in the file (`Could not find a '# {Description}' header`), an empty slug, a project holding `/` or a NUL, a filename of 255 bytes or more (`the limit is 255` — the bash did not bound the slug here, so a long H1 died on the filesystem), an existing plan file of that name, or a home not in its own git repository (checked before the first write; the bash copied and linked, then failed at the commit); a failed link takes back the copy; a failed commit leaves the plan picked up and says so |
| Stdout | `✅ Picked up: <basename> → <filename>`, `   Linked as: <ws>/PLAN.md`, `   Original left in place at: <path>`, then `commit`'s line (`📝 Committed: <message> (<hash>)`; git's own commit summary, which the bash let through, is not printed) |
| Ops | Real |
| Writes | copies the spec (its mode under the umask, as `cp`) to `$DAISY_HOME/plans/<ts>.<project>.<slug>.plan.md` (description and project read from the first `# ` line, split at its last ` — +`; the slug as `plan new`'s, capped), links `PLAN.md`; never touches the original |
| Commits | home, once: `Pick up plan: <filename>` |
| Cases | none (`TestPlanPickup`) |

### daisy plan archive

| Field | Value |
|---|---|
| Status | as built (Step 22): Go (`internal/plan`) |
| Legacy | `plan` and `archive` as one hyphenated word → its bash script, deleted in Step 22 |
| Usage | `daisy plan archive` (any positional argument: usage, exit 2; the bash ignored them) |
| Exit codes | 0 [54]; 1 when `PLAN.md` does not exist, is not a symlink (`not a Daisy plan`), its target is not in `plans/` (`The symlink may be broken…`), `plans/archive/<filename>` already exists (the bash's `mv` replaced it), a project file cannot be read, or the home is not in its own git repository — each decided before the first write; 1 when a write fails, with the move, the link and every project file put back (`Nothing was archived.`, or which undo failed); a failed commit leaves the plan archived and says so |
| Stdout | `✅ Archived: <filename>`; `  ✓ Repointed links in N project file(s)` when any (`project file` [54]); then `commit`'s line |
| Ops | Real |
| Writes | moves `plans/<filename>` to `plans/archive/<filename>`, removes `<ws>/PLAN.md`; in every regular `projects/*.md` and `projects/_archive/*.md` (not dotfiles, as the glob) rewrites each literal `../plans/<filename>` to `../plans/archive/` [54] `<filename>`, written through a link (the bash's `sed -i` replaced a linked project file with a copy), leaving unrelated project files byte-identical [54] |
| Commits | home, once: `Archive plan: <filename>` |
| Cases | 54 |

### daisy metrics

| Field | Value |
|---|---|
| Status | as built (Step 16a of the plan that clears the Deferred list, 2026-11-12): Go (`internal/metrics`, `internal/commands/metrics.go`); `--json` since Step 10 of the plan remediating the 2026-11-15 design review, `tokens_cached` and the per-plan lines since its Step 28, the reverted steps and pull requests since its Step 29, the reviews' effect since its Step 30 |
| Legacy | none |
| Usage | `daisy metrics [--swarm <owner>/<repo>] [--since <time>] [--json]`: `--swarm` takes only that swarm's runs (kind `swarm-run`, `plan_or_swarm` the value), `--since` (an RFC 3339 time) only the rows whose `when` is not before it, both together the swarm's runs since then (Step 15 of the plan for the inbox requests F27-F35, for `/daisy swarm connect`'s metrics since a model or prompt change); a positional argument is usage, exit 2 |
| Exit codes | 0, the summary or none recorded yet; 1 outside a workspace, `--since` not RFC 3339 (`Error: --since must be an RFC 3339 time, such as 2026-11-12T14:05:00+01:00 (got: <value>)`), or when the CSV cannot be read, its header is not the columns `metrics add` writes nor an older header of theirs (below), or a row has the wrong number of cells (`row <i> has <k> cells, not <n>`, or `not 27 to <n>` under a header that has grown) or a cell `metrics add` would refuse (the row named; any row the flags take, a superseded one too), or, with `--since`, a row's `when` is not RFC 3339 (`row <i>: when is not an RFC 3339 time (got: <value>)`), or git cannot read the reverts in a repository that holds a step's range (`Error: cannot read the reverts in <dir>: …`) or in a swarm's mirrors (`Error: cannot read the reverts in <state>'s mirrors: …`), or `$DAISY_LOCAL/swarm` cannot be read |
| Stdout | `No metrics recorded yet (<path> does not exist)` and the line that records one; else `Metrics: <path>`, with a flag `  Rows taken: --swarm <owner>/<repo> --since <time>` (the flags given, in that order), `  N rows, K superseded by a later record of the same step or run` (N the rows the flags take, K those of them a later row of the same key supersedes, whether or not the flags take that row) (a later row with the same kind, plan or swarm, step or run and role supersedes an earlier one: the CSV is only appended to, so a correction is a new row), then one line per kind and model, in that order: rows, tokens (`tokens_total`, else `tokens_in` and `tokens_out` added), cached (`tokens_cached`), cost (`cost_reported`), estimated (`cost_estimated`), measured (rows with lines), code, tests and prose (`+added/-removed` over the measured rows); then, when the rows taken hold a `plan-step` row, one line per plan (`plan_or_swarm`), in name order: its rows, every role's (`step`, `review`, `orchestrator`), tokens as above, code added (`+<code_added>` over the measured rows), tokens per code line (the tokens over the code added, rounded, then ` (k/n)` when k of the plan's n rows report tokens; `—` with no tokens or no code added), the figure the 2026-11-15 design review compared plans by, and reverted (`<k> of <n> (<p>%)`, the rate a whole percentage, over the steps found, then `, <m> not found`; `<m> not found` alone with none found; `—` for a plan with no range), then `  A step is reverted when a commit on HEAD says "This reverts commit <sha>" of a commit in its range,` and `  each range looked up in <dir>, then <dir>.` (`no repository (neither the working directory nor $DAISY_ROOT is in one)` with none): a plan's steps are the distinct `range` cells its rows record, every role's, each looked up in the first repository whose git reads it, the working directory's and then `$DAISY_ROOT`'s (`git rev-parse --show-toplevel`, one when both are the same), and reverted when a commit in that repository's `HEAD` holds git's own line `This reverts commit <sha>` (`git revert`'s; `<sha>` in full or abbreviated) naming one of the range's commits, a revert that a later revert in effect names (a reapply, git's `Reapply "…"`) taking nothing back, the chain followed (reverted, reapplied and reverted again is reverted; decision [141](swarm/decisions/0141-reverts-counted-from-the-mirrors.md)), so a step whose range is in neither (another project's repository: run it from that repository's workspace) is not found; then, when the rows taken hold a `swarm-run` row, one line per swarm (`plan_or_swarm`) and `<state>` of it on this machine (`$DAISY_LOCAL/swarm/<forge-host>/<owner>/<repo>`), in name order: swarm, merged pulls, reverted (`<k> of <n> (<p>%)`, `0 of 0` with none merged) and as of (`the mirrors' last fetch, <time>`; `fetch time unknown` when no mirror read records a fetch, a `FETCH_HEAD`; or `no mirror fetched` when none holding `base` was read), counted as `swarm status`'s `reverted` line counts them (docs/swarm.md section 11, Throughput) on the `base` that `swarm.json` names on the swarm repository's default branch in its mirror, with `--since` only the merges whose commit time is not before it; or `—`, `—` and why there is no count (`no state on this machine`; `no swarm.json in the mirror <path>: …`; `the swarm.json in the mirror <path> names no base`); then `  A swarm's pull request is merged by a merge commit on its base whose second parent a worker run pushed,` and `  and reverted when a commit on base says "This reverts commit <sha>" of it; no fetch is made.`; then, when the rows taken hold a review row (its role `review`, `adversary` or `sympathetic`, or another role's row recording a `review_effect`), one line per role and model of those rows, in that order: review role, model, rows, effect (`review_effect`: a plan review's findings fixed as `metrics add --review-effect` recorded them, a swarm reviewer's as its collection derives them, docs/swarm.md 7.6, The run's record), tokens (as above), and tokens per effect (the tokens over the effect of the rows that have both, rounded; `—` when those fixed none), then `  A review's effect is the in-gate findings it raised that were fixed: a plan review's as its row records it,` and `  a swarm review's its blocking findings when its role's next review of the pull request approves a new head.`; a sum over only some of the rows reads `<sum> (k/n)`, over none `—`; a model left empty is `(none)`. With `--json`: one document (below) |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | none; `TestMetricsAddRecordsAStep`, `TestSummarize`, `TestSummarizeFilter`, `TestSummarizePlans`, `TestSummarizeReviews`, `TestFirstHeaderReadAndExtended`, `TestStep28HeaderReadAndExtended`, `TestMetricsTokensPerCodeLine`, `TestMetricsReviewEffect`, `TestMetricsStepReverts`, `TestMetricsSwarmReverts`, `TestMetricsJSONRoundsCosts`, `TestPullReverts`, `TestRevertedReapplyCancelsTheRevert`, `TestSwarmConnectBriefing`, `TestReadOnlyJSON` |

`metrics --json` (Common behaviour, `--json`) has `file`, the CSV's
path; `exists`, `false` for none recorded yet (the rest then zero, `null`
and `[]`, but `rows_taken`, which keeps the flags given); `rows_taken`, `{"swarm", "since"}`, each flag's value or
`null`; `rows` and `superseded`; `groups`, one per kind and model in the
text's order, each `kind`, `model` (`null` for `(none)`), `rows`,
`tokens`, `tokens_cached`, `cost` and `estimated` (each the sum, the
two costs rounded to six decimals, `null` for the text's `—`, beside `rows_with_tokens`,
`rows_with_tokens_cached`, `rows_with_cost` and `rows_with_estimated`,
the text's `k`), `measured`, and `code`, `tests` and `prose`, each
`{"added", "removed"}`, `null` when no row is measured; `plans`, one per
plan in the text's order (`[]` for none), each `plan`, `rows`, `tokens`
(`null` for `—`) beside `rows_with_tokens`, `measured`, `code_added`
(`null` when no row is measured), `tokens_per_code_line` (`null` for
`—`), `steps` (its distinct ranges), `steps_found`, `steps_reverted` and
`revert_rate` (the reverted over the found, unrounded, `null` with none
found); `repositories`, the repositories a range is looked up in, in
order; `swarms`, one per line of the swarms' (`[]` for none), each
`swarm`, `state`, `base`, `merged_pulls`, `reverted_pulls`,
`revert_rate` (`null` with none merged), `fetched` (RFC 3339, `null`
for `no mirror fetched` and for `fetch time unknown`) and `problem` (`null`, or the why, the counts
and `state` or `base` then `null`); `reviews`, one per line of the
reviews' (`[]` for none), each `role`, `model` (`null` for `(none)`),
`rows`, `effect` and `tokens` (`null` for `—`) beside
`rows_with_effect` and `rows_with_tokens`, and `tokens_per_effect`
(`null` for `—`); and `warnings`. A cost is in dollars, unrounded.
Abridged:

```json
{
  "file": "/srv/homes/work/metrics/metrics.csv",
  "exists": true,
  "rows_taken": {"swarm": null, "since": "2026-11-01T00:00:00Z"},
  "rows": 3,
  "superseded": 0,
  "groups": [
    {"kind": "plan-step", "model": "claude-opus-5-5", "rows": 2, "tokens": 1200, "rows_with_tokens": 1,
     "tokens_cached": null, "rows_with_tokens_cached": 0,
     "cost": 1.5, "rows_with_cost": 1, "estimated": null, "rows_with_estimated": 0, "measured": 1,
     "code": {"added": 10, "removed": 2}, "tests": {"added": 5, "removed": 0}, "prose": {"added": 3, "removed": 1}}
  ],
  "plans": [
    {"plan": "261111115545.daisy.Clear-the-daisy-Deferred-list", "rows": 2, "tokens": 1200, "rows_with_tokens": 1,
     "measured": 1, "code_added": 10, "tokens_per_code_line": 120,
     "steps": 1, "steps_found": 1, "steps_reverted": 0, "revert_rate": 0}
  ],
  "repositories": ["/srv/src/daisy"],
  "swarms": [
    {"swarm": "acme/swarm-app", "state": "/srv/local/swarm/forge.example.net/acme/swarm-app", "base": "main",
     "merged_pulls": 3, "reverted_pulls": 1, "revert_rate": 0.3333333333333333, "fetched": "2026-11-16T08:00:00Z", "problem": null}
  ],
  "reviews": [
    {"role": "review", "model": "claude-opus-5-5", "rows": 1, "effect": null, "rows_with_effect": 0,
     "tokens": null, "rows_with_tokens": 0, "tokens_per_effect": null}
  ],
  "warnings": []
}
```

### daisy metrics add

| Field | Value |
|---|---|
| Status | as built (Step 16a of the plan that clears the Deferred list, 2026-11-12): Go (`internal/metrics`, `internal/commands/metrics.go`) |
| Legacy | none |
| Usage | `daisy metrics add --step <s> [--plan <p>] [--range <r> [--repo <dir>]] [<flag> <value>...]`: every word a value flag, in any order, no positional argument. `--step` the step as the plan numbers it (`16a`); `--plan` a plan file's path (the working directory's when relative), else one plan in the home's `plans/` or `plans/archive/` named in full, without `.plan.md`, by its timestamp or by its slug, and without `--plan` the plan `PLAN.md` links to; `--range` the step's commits, `<a>..<b>` or `<commit>^!` (a lone commit would be diffed against the working tree, a word starting with `-` is refused, and so is `<a>...<b>`, git's symmetric difference), measured in `--repo` (default the working directory) and stored resolved to its commits' shas there (`git rev-parse`: `<sha>..<sha>`, an empty side `HEAD`'s, or `<sha>^!`), so `HEAD^!` names the same commit once HEAD moves; `--when` an RFC 3339 time (default now); `--role` (default `step`; a review pass's row is `review`, the orchestrator's own row of the step `orchestrator`, its tokens given when it can count them and else left empty), `--provider`, `--model`, `--effort`, `--outcome`, `--prompt-hash` and `--notes` free text; `--review-rounds`, `--tokens-in` (every input token, those read from a cache included), `--tokens-out`, `--tokens` (the total, when only that is known), `--tokens-cached` (the part of the input read from a cache, so not more than `--tokens-in` when both are given) and `--review-effect` (a `review` row's in-gate findings that were fixed, over the review passes the row records, from the step agent's hand-back) whole numbers; `--cost` (reported) and `--cost-estimated` dollars (`1.25`); `--root-sha` hex, default `git rev-parse HEAD` in `$DAISY_ROOT`, the root the step ran under; `--home-sha` hex, default `git rev-parse HEAD` in the bound home, so the two name every prompt and skill in use (the root's by the root's sha, the home's skills and overrides by the home's). A flag not given is an empty cell, unknown, never zero |
| Lines | `git -C <repo> diff --numstat -z -M <range> --`, never a count the caller gives: each file classed by the path it ends at (a rename by its new name): a `_test.go` file, or one under a directory named `tests` or `test` at any depth, is **tests**; else a `.md` file, or one under a directory named `docs`, is **prose**; anything else is **code**. A binary file (git's `-` counts) is counted as no lines and in `binary_files`. No `--range`: the line cells are empty |
| Exit codes | 0 recorded; 1 outside a workspace, no `--plan` and no `PLAN.md` (or a broken link), a `--plan` naming no plan or more than one (each listed), a cell refused (`Error: --tokens must be a whole number (got: 12k)`), a range refused or `git diff` failing (its first line), the CSV's header not these columns nor an older header of theirs, a file unreadable, the metrics lock held past 30 seconds, a write failing (the CSV put back: `Nothing was recorded.`); 2 for `--step` missing, `--repo` without `--range`, `--tokens-cached` more than `--tokens-in` (`Error: --tokens-cached <k> is more than --tokens-in <n>: the cached tokens are a part of the input`), a value flag without a value, a positional word |
| Stdout | `  ✓ Recorded step <s> of <plan>: code +a/-r, tests +a/-r, prose +a/-r` (then ` (N binary file(s), counted as no lines)` when any; ` (no --range: no lines measured)` without one), `  CSV:  <path>`, `  Plan: <real path>` |
| Stderr | errors; `⚠️  root_sha left empty: …` when `$DAISY_ROOT` is no clone and no `--root-sha` is given, `⚠️  home_sha left empty: …` when the home has no commit and no `--home-sha` is given |
| Ops | Real |
| Lock | `$DAISY_LOCAL/metrics.lock` (`env.MetricsLock`), taken before the CSV is read and held until the plan is written (or the CSV put back), so that a row `swarm apply` appends meanwhile (a swarm's run, `docs/swarm.md` 7.6, The run's record) is not written over; held by another writer, it waits, and past 30 seconds refuses, naming the holder (`Error: cannot take <lock>: <lock> is held: <holder> pid <pid>`, `Nothing was recorded.`). One lock a machine, outside the home, whose next commit would sweep a lock file up |
| Writes | `<home>/metrics/metrics.csv` (`env.MetricsFile`; directory created; written atomically, through a link): the header (the columns, in order: `when`, `kind` — `plan-step` here, `swarm-run` for a swarm's run, which `swarm apply` appends to the home `<state>/home` names (`docs/swarm.md` 7.6, The run's record), `intervention` for a swarm maintainer's intervention (`docs/swarm.md` section 11, The interventions' record) —, `plan_or_swarm` the plan's file name without `.plan.md`, `step_or_run`, `role`, `provider`, `model`, `effort`, `root_sha`, `home_sha` (empty for a row with no home), `prompt_hash`, `outcome`, `review_rounds`, `tokens_in`, `tokens_out`, `tokens_total`, `cost_reported`, `cost_estimated`, `code_added`, `code_removed`, `tests_added`, `tests_removed`, `prose_added`, `prose_removed`, `binary_files`, `range`, `notes`, `tokens_cached`, `review_effect`, `cause` — a swarm run's stop's cause, `docs/swarm.md` 7.5, or an intervention's type, empty here —, `config_hash` — a swarm run's configuration, `docs/swarm.md` 7.6, empty here) when the file is new, then one row, quoted as CSV quotes (Go's CSV writer); the bytes before it are the file's, which must start with that header or an older one, the columns up to `notes` (the header before Step 28 of the plan remediating the 2026-11-15 design review) or up to `tokens_cached` (before its Step 30) or up to `review_effect` (before Step 4 of the plan implementing the 2026-11-19 swarm roadmap) or up to `cause` (before its Step R3-12b; else refused, nothing written): an older header's line is rewritten to this one and the rows under it kept as they are, each as short as the header it was written under, read with the columns it lacks empty (a column is only ever added at the end). The plan, at its real path (never through `PLAN.md`): the step's row of the `## Metrics` table (`\| Step \| Role \| Model \| Effort \| Root \| Home \| Outcome \| Reviews \| Tokens \| Cost \| Code \| Tests \| Prose \| Range \| Notes \|`, the two shas cut to 7 digits, a cell's `\|` escaped and its line ends made spaces): the row of the same step and role replaced when the table has one, else this one appended; no table, it is added at the end of the section; no section, the section goes after `## Decisions`, else at the end of the plan; every other byte of the plan as it was, but the blank line before an added section |
| Commits | none: the next home commit sweeps both files up, as for `feedback` |
| Cases | none; `TestMetricsAddRecordsAStep`, `TestMetricsAddRefusals`, `TestMetricsTokensPerCodeLine`, `TestMetricsReviewEffect`, `TestFirstHeaderReadAndExtended`, `TestStep28HeaderReadAndExtended`, `TestClassify`, `TestCount`, `TestAppendCSV`, `TestPlanRowInsertsAfterDecisions`, `TestPlanRowPlacement`, `TestAppendUnderTheLockLosesNoRow`, `TestLockGivesUpNamingTheHolder` |

## Homes and identities

### daisy home

| Field | Value |
|---|---|
| Status | as built (Step 21): Go, four rows of the `cli` table (`home` is a noun, D35) |
| Legacy | `daisy home [list\|add\|remove\|new]` → the bash `home` script, deleted in Step 21 |
| Usage | `daisy home list`, `daisy home add <name> <path>`, `daisy home remove <name>`, `daisy home new <name> [path]`; bare `daisy home` or another verb: exit 2 listing the four usages (bare `daisy home` was `home list` in bash) |
| Exit codes | `list`: 0 [68] (69 and 70 run it and assert stdout only). `add`: 0 registered [68]; 1 when the name is `already registered` [68], invalid (`bad=name`, case 68), the path is not a directory [68], has a path problem (`Error: <path> <problem>`: the root itself — `cannot be a home` [68] — or a directory containing it, `contains the Daisy root; …`, which case 68 pins through a registry line and `status`, not through `home add`; the path is already resolved here, so the other two problems cannot arise), has no `include.txt` [68], is `not in its own git repository` [70] (the root's is not one, since Step 24), or nothing under it can be staged. `remove`: 0 [68]; 1 for an unregistered name [68]. `new`: 0 created [69]; 1 when the name is registered (nothing written, case 69) or the location `already exists` [69]; 1 when any step after the first write fails, up to the registration — the copy is removed: `was removed and nothing is registered` [70] on stderr (the build of `<home>/AGENTS.md` after it is a warning, exit 0, the home kept: its `Error: …` and `⚠ AGENTS.md not built; run 'daisy prompt build <name>'` on stderr); 1 when a prompt gets EOF |
| Stdout | `list`: `%-14s %s` name and path per home (`testhome` [68] and its path [68, 69]; ` (missing)` when the directory is gone); no homes: `No homes registered in <registry>` on stderr, exit 0. `add`: `✓ Registered home '<name>' at <path>`, a blank line, `Next: daisy init <name> (in the folder you work in)`. `remove`: `✓ Unregistered home '<name>' (its files at <path> are untouched)`. `new`: the prompts `  Location [<default>]: ` (only without `[path]`), `Git identity for home '<name>':`, `  Name: `, `  Email: ` through `Ops.Prompt` (the questions on stderr); `  ✓ Saved git identity to <path>/gitconfig` or `  ⚠ Skipped (both name and email required)…`; `  ✓ Initialized git repository with first commit`; `✅ Created home: <name>`, `   Location: <path>`, `   Prompts: <path>/include.txt chooses which load; your own go in <path>/prompts/`, a blank line, `Next: daisy init <name> (in the folder you work in)` (Common behaviour, Next) |
| Stderr | the errors above, each with its fix on a second line (`include.txt` [68] names what a home must contain); `add` refusing a non-repository says `not in its own git repository` [70] |
| Ops | `list` ReadOnly; `add`, `remove`, `new` Real (decided, D30) |
| Writes | the registry `$DAISY_CONFIG/homes` (`name=/absolute/path` lines; `add` stores the resolved real path). `new`: copies `templates/home/` to the location (default `$DAISY_LOCAL/homes/<name>`) under the umask, as the bash's `cp -r` did (a template file 644 is 600 under umask 077, `TestCopiesTakeTheUmask`), fills in `gitconfig`: the `name=` [69] and `email=` [69] lines take the place of the template's commented `# name=` and `# email=`, every other line kept, so the commented `# sshkey=` hint stays [69] (`identity.Fill`), `git init` (on `main` as `init`'s is [69], the user's `init.defaultBranch` winning) with `user.name`/`user.email` set and a first commit `New home: <name>` as that identity [69], registers, builds `<home>/AGENTS.md`; the template `.gitignore` ignores `.healthcheck.ttl` and `AGENTS.md` [69]; any failure up to the registration removes the copy and registers nothing [70] (a failed build is a warning) |
| Commits | `new`: the new home's first commit; otherwise none |
| Cases | 68, 69, 70; `TestNewcomersNextLines` |

Case 68 also pins `init` against the registered homes (`init second` binds
the links into that home) and `status` after `home remove` (`not registered`
[68], exit 1); case 69 that `init fresh` binds `.daisy/today.md` into the
new home.

### daisy identity list

| Field | Value |
|---|---|
| Status | as built (Step 21) |
| Legacy | none |
| Usage | `daisy identity list` |
| Exit codes | 0; 1 when a registered home cannot be read: the rest are listed, and the unreadable ones named on stderr (`home '<name>' is registered at <path>, which <problem>`) |
| Stdout | one line per identity across the registered homes, `%-14s %s`, identity then home: each home's own name first, then its `identities/*` in name order — the list `init`'s unknown-identity error prints; a file named like a registered home, usable or not, which `init` never resolves since the home takes the name, ends ` (shadowed: the home <name> takes this name)` (`TestHomeOf`) |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | none in bash; `TestIdentityList` and `TestHomeOf` |

## Prompts and skills

### daisy prompt build

| Field | Value |
|---|---|
| Status | as built (Step 20): Go, renamed from `build` (no alias) |
| Legacy | `daisy build [home]` → the bash `build-prompt` script, deleted in Step 20 |
| Usage | `daisy prompt build [<home>]`: with `<home>` builds `<home>/AGENTS.md` (no workspace needed); without, builds the current workspace's `.daisy/AGENTS.md` for its bound home (after a root update, a bare `daisy init` rebuilds it and refreshes the rest of what init writes). (`--output`, which the bash `init` passed, went with it in Step 21; `home new` builds in-process and `optimize` passes the home's name.) `daisy prompt` alone or with another verb is a usage error listing the `prompt` commands |
| Exit codes | 0; 1 for an unregistered or unusable home (the Home resolution messages), outside a workspace without `<home>`, a missing `include.txt` (`Error: <home>/include.txt not found` and `Create include.txt in your home directory listing prompts to include`), an `include.txt` line with an unmatched quote, a prompt that cannot be read, or an output that cannot be written; nothing is written then; 2 for a surplus argument |
| Stdout | `✅ Built AGENTS.md for home: <home>`, `   Full: <names>`, `   Lazy: <names>` (each only when not empty), `   Output: <file>`, `   Size: N lines` |
| Stderr | `Warning: Prompt not found: <path>` (the root's path, the last looked in; an error block is written into the output and the build continues); `Warning: No ## Trigger section found in <path>` and `  Include as full prompt instead, or add a ## Trigger section` (included in full, counted as Full) |
| Ops | Real |
| Writes | the output file (mode 0644, replaced atomically) and, for the workspace's build, `.daisy/prompts/<name>.md` for each layered override in the home's `prompts/`, included or not (D49) — the merged prompt, byte for byte `prompt.Merge`: the root's file with the home's rules appended to its `## Rules` section after one blank line, renumbered after the root's rules (a root without the section gets one where `optimize apply --rules` puts it) — any other `*.md` there removed [89]; a layered file that is not exactly the marker and one `## Rules` section fails the build, naming the file and line. When the home overrides any root prompt, a `# Home overrides: wherever a prompt names the root's file, read the home's` block follows the header, one line per override (`` - `<root file>`: read `<file>` instead (a layered\|full override) ``; a home's own build, which materialises nothing, says of a layered one `` - `<root file>`: read it, then the rules in `<home file>`, which follow its own (a layered override) ``), and every `$DAISY_ROOT/prompts/<name>.md` of an overridden prompt in the text written — a lazy stub or a whole prompt — names that file instead: the home's for a full override, the materialised one for a layered [89]; a home without overrides builds as before. A layered file with no root file under it fails the build. A home's own build (`<home>`) materialises nothing and takes a lazy layered prompt in whole. The output: a header — `# Daisy Prompt - Auto-generated`, `# Home: <home>`, `# Built: <YYYY-MM-DD HH:MM TZ>`, `# Source: <home>/include.txt`, `#`, `# This file is generated by daisy prompt build`, `# Output: <file>`, `# Do not edit directly - edit source prompts in prompts/ directory`, `# To rebuild: <command>` (`daisy prompt build <home>` for a home's file; `daisy init`, run in the workspace, for a workspace's, whether `init`, `prompt build` or Check 8b's comparison builds it, `TestInitStampsTheWorkspace`, case 84), a blank line — then per `include.txt` entry a banner and the prompt: whole for `name`, the `## Trigger` section (the lines up to the next line starting `#`, trailing blank lines dropped) for `~name`, resolved in `<home>/prompts/` then `$DAISY_ROOT/prompts/`, every literal `$DAISY_ROOT` replaced by the root. `include.txt`: blank lines and lines whose first non-space is `#` skipped; each other line's words read as `xargs` reads them (blanks collapsed, quotes and backslashes honoured) and a leading `~` making it lazy. For a home without overrides, byte for byte the bash's output except the `Built:`, `This file is generated by` and `To rebuild:` lines (Step 20, D35) |
| Commits | none |
| Cases | 84; every `init` case (32, 33, 76–79) and `home new` (69) build through it |

### daisy install

| Field | Value |
|---|---|
| Status | as built (Step 21): Go; non-interactive in both forms (the bash asked for a default home and wrote a variable naming it, which nothing reads since Step 24); writes nothing under `~/.claude` since Step 13 of the plan that clears the Deferred list (2026-11-12): skills and permissions are `init`'s, per workspace |
| Legacy | `daisy install [--update]` → built in to `daisy.sh`, deleted in Step 21 |
| Usage | `daisy install [--update]`: `--update` re-applies the symlink and takes back what earlier installs put under `~/.claude` (Writes); plain `install` also writes the shell rc line and refuses when already installed, the link and the rc line both naming this root (`Error: Daisy is already installed.`, exit 1; a link already right with the rc line naming another root is installed, D47). The root is the one whose `bin/daisy` is running (`Ops.Executable()`, `<exe>/../..` with links resolved; a `$DAISY_ROOT` that resolves to it is used as set), so `<root>/bin/daisy install` bootstraps a shell with nothing set; a `$DAISY_ROOT` that resolves to another root is refused before any write, both forms (`Error: DAISY_ROOT is <set>, but this binary (<exe>) is in <root>` [86], then the fix, plain `install` from either form since it also writes the rc line the export came from: `DAISY_ROOT=<root> <root>/bin/daisy install` or unset it, and `<set>/bin/daisy install` for the other root, and a `grep` for an export in another rc file, for a new shell refused again; D47 — a stale export had sent an install, and `~/bin/daisy`, to the old root). It does not build: `make build` writes the binary, and `install` warns when it is not built yet. `make install-dev` (`make -C <root> install-dev` from anywhere) is the one command that installs a root as its remote has it: `git pull --ff-only`, `make build` with a writable `GOCACHE` (Go's own cache, else `$TMPDIR/gocache`, said on stdout), `install --update` with `DAISY_ROOT` the root, then a failure (make exits non-zero) unless `~/bin/daisy version` is the root's `git rev-parse HEAD` exactly: another commit (`Error: ~/bin/daisy is <sha>, but the root's HEAD is <sha>`), or HEAD marked ` (modified)`, a tree with changes, an untracked file included (`Error: ~/bin/daisy is <sha> built from a tree with changes HEAD does not hold`, then `git status --porcelain` and the fix; a binary marked modified over a tree with none is the stale build, the first error) (`TestInstallDevChecksTheInstalledCommit`) |
| Exit codes | 0 [31]; 1 already installed, `~/bin/daisy` exists and is not a symlink, `~/.claude/settings.json` cannot be read or is not a JSON object (decided before any write), `HOME` unset, no `DAISY_ROOT` and the binary not under a root's `bin/`, a `DAISY_ROOT` other than the binary's root [86] or set with the binary's own path unknown |
| Stdout | `Installing Daisy...` / `Updating Daisy...`, `  DAISY_ROOT: <root>`, ✓ lines per step, `  ✓ Removed N copies of root skills from ~/.claude/skills (<names>); Daisy installs skills in each workspace now: run 'daisy init' there` (`Removed 1 copy of a root skill from ~/.claude/skills (testing)` [31]) and `  · Kept ~/.claude/skills/<name>: named as a root skill but not a copy of any version of it (an edit, a file of your own, or no SKILL.md); remove it by hand if it is Daisy's` (`Kept ~/.claude/skills/git` [31]) per directory kept, and, when `~/.claude/skills` leads into the root (a link to its `skills/`, whose skills are each a version of themselves), `  · Left ~/.claude/skills as it is: it leads into the root (<real path>), so it holds the root's own skills, not copies an install left` in their place, nothing removed (`TestInstallUpdateKeepsTheRootsSkillsThroughALink`), `  ✓ Removed Daisy's permissions from ~/.claude/settings.json (each workspace's 'daisy init' grants them now)` or `  ✓ Removed ~/.claude/settings.json (it held only Daisy's permissions, …)`, each only when it did so, `  Shell config: <rc>` and `  ✓ Added DAISY_ROOT to <rc>` / `  ✓ DAISY_ROOT already set correctly` / `  ✓ Updated DAISY_ROOT (was: <old>)`, `--update`: `Done.`, no `Next:` line (Common behaviour, Next); plain: `Done. To activate, run:`, `  source <rc>`, a blank line and `Next: daisy home new <name> (or 'daisy home add <name> <path>' for a home you already have)` while no home is registered, else `Next: daisy init <home> (in the folder you work in)`, `<home>` the first registered home whose directory exists (`env.UsableHomes`), the `home new` line when none does (`TestNewcomersNextLines`, `TestInstallNextSkipsAMissingHome`) |
| Ops | Real |
| Writes | `$HOME/bin/daisy` symlink; under `$HOME/.claude` it creates nothing and rewrites a file only to take one of Daisy's rules out [31]: it takes back what earlier installs put there, and only that. Of `$HOME/.claude/skills`, it removes each copy of a root skill (`skill.RootCopies`): a directory named as a skill the root has or has had (a directory holding a `SKILL.md` under `skills/`, in the working tree or any commit of the root's history, `git log --relative --raw -- skills`), holding a `SKILL.md`, every file in it (a link by its target) byte for byte a version of the file at that path under such a directory, in the working tree or the history (compared by git blob id, in the root's object format), so a copy an older root installed is removed [31] and a copy edited, or holding a file of the user's, is kept and named [31]; a skill of another name, `synced` (where Claude Code's own skill sync writes) [31], a hidden name and a link are never touched; a root that is not a repository has its working tree alone (`TestRootCopies`). Of `$HOME/.claude/settings.json`, it removes Daisy's allow rules in every form an install wrote (`Read(/$DAISY_ROOT/**)`, `Edit(/$DAISY_ROOT/**)` — the leading slash plus the absolute root gives the `//`-anchored form — `Bash(daisy:*)`, the legacy `Bash(…/daisy/scripts/*)`, single-slash and triple-slash rules), the rest kept in place, the file re-indented when a rule came out and removed when nothing else is left, as `clean` does; a file holding none of them is not written [31] (`TestInstallRemovesOnlyRootSkillCopies`); without `--update`: the rc file by `$SHELL` (`~/.zshenv`, `~/.bashrc`, `~/.config/fish/config.fish`, else `~/.profile`): every line starting (after any indent, which is kept) with `export DAISY_ROOT=` (fish: `set -gx DAISY_ROOT `) replaced in place unless every one already names the root, quoted or not (a later stale one is what the shell keeps; the lines checked are the lines rewritten, so a comment or a line using `$DAISY_ROOT` is neither, D47), else, with no such line, `# daisy` and the line appended; a file kept as a symlink is written through. Applying twice equals once |
| Commits | none |
| Cases | 31, 86; `TestNewcomersNextLines`; `TestInstall`, `TestInstallBootstrapsFromTheBinary`, `TestInstallRefusesAnotherRoot`, `TestInstallRewritesAStaleRcLine`, `TestInstallRemovesOnlyRootSkillCopies`, `TestInstallRoot`, `TestRootCopies` |

### daisy feedback

| Field | Value |
|---|---|
| Status | as built (Step 23): Go; the bash's bytes but where D38 says |
| Legacy | `daisy feedback [--workflow <n>] <text>` → `feedback.sh`, deleted in Step 23 |
| Usage | `daisy feedback [--workflow <n>] <text...>`; `echo <text> \| daisy feedback [--workflow <n>]`; `-w` is `--workflow` (the last given wins); the words are joined by one space; piped text loses its trailing newlines |
| Arguments | `<n>`: `prompts/<name>`, `skills/<name>`, or a bare name resolved as a prompt (the home's, then the root's), a skill source (the home's over the root's), or an alias for `prompts/daisy` — the home's over the root's, as for `daisy` — (`new-day`, `new-week`, `done`, `log`, `add`, `priority`, `cancel`, `status`, `sync`, `project`, `projects`, `home`); a name holding `/` names nothing; a tag that names no file is refused before anything is written [67] |
| Exit codes | 0 recorded [67]; 1 for an unresolvable workflow [67], no message given — no word, and stdin a terminal or a pipe that brings nothing but newlines (usage and the workflow list on stderr, `TestFeedback`) —, an empty message (`feedback ""`: an empty word is a message given, so stdin is not read), a missing `feedback/feedback.md`; 2 for `--workflow` without a value |
| Stdout | `  ✓ Feedback recorded (workflow: <n>, <YYYY-MM-DD HH:MM>)` (no `workflow: …, ` without one), `  File: <path>` |
| Stderr | `Error: workflow '<n>' names no prompt or skill` (`names no prompt or skill` [67]), `  Qualify the target, e.g. --workflow prompts/daisy or --workflow skills/git.`, `  Available:` and every target, byte order (`prompts/daisy` [67], `skills/git` [67]) |
| Ops | Real |
| Writes | appends to `feedback/feedback.md` (through a link, atomically): blank, `## <YYYY-MM-DD HHMM>`, `**Workflow:** <n>` (`**Workflow:** skills/git` [67]) when given, blank, the text; nothing on refusal (byte-identical, case 67) |
| Commits | none (decided, D38: the entry is swept up by the next home commit, as for `plan new`) |
| Cases | 67; `TestFeedback` |

### daisy optimize

| Field | Value |
|---|---|
| Status | as built (Step 23): Go: the redesign agreed in D12 — no API key; the briefing and `apply`; the rewrite threshold 3, fixed, where it was 10 (Step 24 of the plan that clears the Deferred list, 2026-11-13) |
| Legacy | `daisy optimize [--workflow <n>] [--dry-run] [--force] [--list]` → `optimize.sh`, deleted in Step 23 with the 2-entry floor, `--force`, `--dry-run`, `--list`, `EDITOR` and the `y/N/e` prompt |
| Usage | `daisy optimize [--workflow <n>]` prints the briefing; `daisy optimize apply --workflow <n> (--rules\|--rewrite) <file> [--yes]` applies text the agent proposed (`-w` is `--workflow`; `<file>` is the working directory's when relative) |
| Targets | entries group by the file their `**Workflow:**` tag resolves to (as `feedback` resolves it), in order of first appearance; two tags naming one file are one target (`[prompts/plan] 2 entries` [38]); an unheadered entry is `prompts/daisy`'s (`[prompts/daisy] 1 entry` [41]); `--workflow <n>` scopes to the target `<n>` names (`[prompts/plan] 1 entry` [40]; one naming nothing is refused, exit 1 [40]); a skill resolves to its source `SKILL.md` (`Target: <home>/skills/widgetcraft/SKILL.md` [49]); a tag naming no file is a group of its own, briefed with the reason (`[daisy-write] 1 entry` [67], `names no prompt or skill` [67]) and never defaulted to daisy.md |
| Rules | the `## Rules` section runs from its heading (`## Rules`, or `## Rules` and more after a space — a heading such as `## Rules of the loop`, which no shipped file has today but `TestRulesSectionBoundary` pins — as the bash's `/^## Rules/` read it, bounded at the word; closing `#`s allowed) to the next `#` or `##` heading; an `###` is inside it [53]; Daisy reads ATX headings, LF or CRLF line ends (up to three spaces, one to six `#`, then a space, a tab or the line's end) and fenced code (three or more backticks or tildes, a backtick fence's info string holding no backtick — "```x``` …" is inline code — closed only by a run of the same character at least as long, then only spaces or tabs); a line in a fence is no heading. Other markdown (setext headings, containers, HTML blocks) is not read: where a section, the target's or a proposal's, holds a line of only `=` or `-` straight after a non-blank line other than a heading (to another reader, perhaps a setext underline; a list item above is no exception), `apply --rules` refuses, naming the line and the file, rather than guess where the section ends; a blank line above it makes it a thematic break. So does a fence opened in the target's section and never closed (the section would run to the end of the file), or, without a section, a fence left open at the end of the file when no `## ` heading other than the Trigger precedes it (the rules would be appended inside it); the briefing prints the refusal under the target (`--rules refused: …`) (D38). A rule is a list item at the left margin (`N. `, `- `, `* `) outside a fence (`Rules: 2` [53]). `daisy help <skill>` prints the same section |
| Briefing | under `ReadOnly` [57]. `Pending feedback: N entries for N targets (<feedback.md>)` [38], or `Nothing pending in <feedback.md>` [56]; per target `[<label>] N entries` [38], `Target: <path>` [38] — then ` (root — apply writes a home override)` for a target in the root, ` (home override, layered)` or ` (home override, full)` for one the home overrides (`, behind the root: …` added when `feedback/.overrides` says the root changed since the copy) [62, 88]; an override's text, its rules and their count are the merged ones (D49) —, `Rules: N` [39, 56] (`Rules: N — more than 3: rewrite flagged` [56]; `Rules: none …`), `--- entries ---` and each entry as written, `--- current ## Rules ---` and the section [39, 53], `--- end ---`; a vendored skill (carries `LICENSE`/`NOTICE.md`) is listed with the reason (`vendored` [50]) and its entries stay pending; then `Rewrite flagged (more than 3 rules, nothing pending):` and `[<label>] rewrite flagged: N rules` [56] for every active target over 3 rules (the prompts `include.txt` names, in its order, then every skill source by name; 3 is not flagged [56]); then how to apply (`daisy optimize apply --workflow <label> --rules` [39], `--rewrite` [56]). No floor: one entry is briefed (`Skipped` [39] and `floor` [39] absent) |
| Apply | `--rules <file>`: the file must be one `## Rules` section — its first non-blank line the heading (`must begin with` [61]), no other `#`/`##` heading outside a fence (named with its line, `## Another Section` [61]), no fence left open; and the result must read back as the section written, so nothing after it is swallowed; it replaces the target's section, keeping the blank lines between the section and the next heading as they were, so replacing the rules with themselves changes nothing, byte for byte [53]; `# Document Title` [53, 58], `## Next Section` [53] and `Body text` [53, 58] survive; a target without a section gets one before its first `## ` heading other than the Trigger, or at the end. `--rewrite <file>`: the file, byte for byte [60]; an empty one is refused. Both print the unified diff (`-1. Measure before cutting.` [58], `+2. Cut once.` [58]) and ask `Apply these changes to <label>? [y/N] ` [58] on stderr through `Ops.Prompt`, unless `--yes` (never asks [60]). `y`/`yes` (any case) applies; anything else is `Cancelled` [59]; EOF is `Cancelled — no answer on stdin (EOF); nothing written.` (`EOF` [64]); any other failure to read the answer is `Error: cannot read the answer: …`, exit 1, nothing written. The writes are computed from what was read before the question: if the target, `feedback.md` or `archive.md` changed while it waited (an entry recorded meanwhile), the apply is refused, exit 1, nothing written. Applied: the target (`  ✓ Updated ` [58] `<path>`), the target's pending entries — every tag naming it [58] — moved from `feedback.md` to `archive.md`, each followed one blank line down by `**Addressed by:** `` `daisy optimize` `` <YYYY-MM-DD> — `` `<target>` `` (rules\|rewrite\|layered rules\|full rewrite)` [63, 62, 88], `<target>` relative to the home — the override's path for a root target [62] — or, when nothing is written (rules adding nothing to a root target), the file as it stands (`$DAISY_ROOT/…`) with the plain mode; `feedback.md` keeps its header and the other entries, one blank line apart, byte for byte [66]; `feedback/.last-optimized` gets `<YYYY-MM-DD HHMM> <label>` [63]; for a prompt, the home's `AGENTS.md` and the workspace's `.daisy/AGENTS.md` and `.daisy/prompts/` (see `prompt build`) are rebuilt from the prompts as written (for a skill, the run says `daisy init` in each workspace propagates it [49]; for the home's override of a root skill, `daisy init` in this one); the home is committed once [58, 60, 62]. **A target in the root is never written (D49).** For one with no home file, or a layered override, apply writes the home's file at the same path (`<home>/prompts/<name>.md`, `<home>/skills/<name>/SKILL.md`), says `Writes the home's <form> override <path>; the root's <path> is not written.`, and the root's tree is byte-identical [62, 88]: `--rules` a **layered** override — the line `<!-- daisy: upstream body -->` then `## Rules` holding the home's rules: the proposal is the merged section as briefed, the root's section must stand first in it unchanged (a root rule changed or dropped is refused with the proposal's line, `use --rewrite`) and the rules after it are the home's, renumbered from 1; applying again rewrites the override only [62]; adding nothing to a root target writes no override. `--rewrite` a **full** override, the proposal whole, the diff from the root's (or the merged) text, and `<rel> <sha256 of the root file>` in `feedback/.overrides` [88]; over a layered override it says `The layered override is replaced by a full override`. A home prompt, and a full override, are written in place as before; a `--rewrite` of a full override records the root's hash as it is now. Applying text already there with nothing pending: `Nothing to apply: …`, exit 0 |
| Exit codes | 0 (nothing pending, a briefing, an applied, declined or EOF apply, nothing to apply); 1 when `feedback/feedback.md` does not exist (`Error: No feedback file found at <path>` and the `mkdir -p … && touch …` that makes it), a target the briefing cannot read (printed in its place, the errors after the briefing, `The briefing above is incomplete.`), a file changed during the question, `archive.md` absent with no template to start it from when entries are to move, a workflow naming no file [40, 61] (`names no prompt or skill`), a vendored target on `apply` [50], a proposal that cannot be read (`cannot read` [61]) or is not one rules section [61], the home not a repository, a write or the AGENTS.md build failing (every write taken back: `Nothing was applied.`), the commit failing (the files written, the `daisy commit <message>` to run printed), a target `--rules` cannot write (the open-fence and setext refusals of the Rules row, naming the target); 2 for a usage error [57, 61]: `--dry-run`, `--force`, `--list` [57] or any positional argument to the briefing, bare `optimize apply` [61], no `--workflow`, neither or both of `--rules`/`--rewrite`, a value flag without a value (an empty word, or one of the command's own flags, is none) [61], a stray word [61] |
| Ops | ReadOnly, the briefing [57]; `optimize apply` Real |
| Writes | `apply` only, every refusal before the first write and a failed write taking back the ones before it: the target prompt or `SKILL.md` in the home (never the root's [62, 88]), `feedback/feedback.md`, `feedback/.overrides` (a full override over the root), `feedback/archive.md` (started from `templates/home/feedback/archive.md` when absent; no template is an error when entries are to move), `feedback/.last-optimized` (`healthcheck` Check 7 reads it), the home's `AGENTS.md` and the workspace's `.daisy/AGENTS.md` and `.daisy/prompts/`. Daisy writes by path and never at a root path; as a best-effort check, not a guarantee, a write whose path, its links resolved, lies in the root (a user-made link: a home file or directory linked to the root's, a `.daisy/prompts` linked into it) is refused before any write, `Error: <path> leads into the root (<real path>), which Daisy never writes; …`, `Nothing was applied.` (`TestNoWriteReachesTheRootThroughALink`; `prompt build` the same); a registered home inside the root's directory is not the root, as for a root commit (`TestHomeInsideTheRootIsWritten`); a link at a materialised path is replaced, never written through. The suite's fixture copies `prompts/` and `skills/`, so not even a fault could write the repository's (case 65) |
| Commits | `apply`: the home, once, `Optimize <label> (<mode>)` [58], holding the override and the feedback files, never the materialised prompts (the workspace's) [62]; never the root [58, 62, 88] |
| Cases | 38, 39, 40, 41, 49, 50, 53, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 88; `TestApplyRulesOnARootTargetLayers`, `TestApplyRewriteOnARootTargetIsFull`, `TestOptimizeBriefsOverrides`, `TestSkillOverrides`, `TestOptimizeApplyApproval`, `TestOptimizeApplyArchivesAndCommitsOnce`, `TestOptimizeApplyRollsBack`, `TestOptimizeUsage`, `TestOptimizeBriefing` |

### daisy eval

| Field | Value |
|---|---|
| Status | as built (Step 23): Go; the bash's bytes but where D38 says |
| Legacy | `daisy eval [<case>] [--record]` → `eval.sh`, deleted in Step 23 |
| Usage | `daisy eval` lists the cases in `$DAISY_ROOT/prompts/evals/*.md`; `daisy eval <case>` prints one; `daisy eval --record <case> pass\|fail ["notes"]` records a result (the help line `eval [--record] [<case> [<result>]]` abbreviates this form, the notes left out; `--record` leads). With no notes and a terminal, asks `Notes (optional): ` through `Ops.Prompt` |
| Exit codes | 0; 1 for a missing evals directory, an unknown case (`Error: Eval case not found: <case>`; a name holding `/` is none), a result other than `pass`/`fail`, `--record` without a case (its usage line); 2 for a second word without `--record` or a fourth with it |
| Stdout | list: `No eval cases found in <dir>` (exit 0) or `Available eval cases:`, `  %-20s  <H1 title>` per regular `*.md` file in byte order, the two usage lines; one case: the file, then `  Record result: daisy eval --record <case> pass\|fail "notes"`; record: `  ✓ Recorded <result> for eval '<case>' (<YYYY-MM-DD HH:MM>)`, `  File: <path>` |
| Ops | Real (decided, D30: one wrapper per command; `--record` writes) |
| Writes | `--record`: `$DAISY_HOME/feedback/eval/results.md` (created with a header), appending `## <YYYY-MM-DD HHMM>`, `**Case:**`, `**Result:**`, the notes |
| Commits | none (decided, D38, as `feedback`) |
| Cases | none; `TestEval` |

## Swarm

### daisy swarm init

| Field | Value |
|---|---|
| Status | as built (Step 1 of the swarm build; finding the swarm, `<state>/env` and the unit's root at Step 2 of revision 6's plan, the product repositories by forge name and the mirrors' remotes at Step 3, the swarm repository's mirror and question 32's swarm-repository paths at Step 4, `repos/<name>.md`'s seed at Step 5 (the declared repositories' mirrors fetched before the files, the swarm repository's after them), the rest of revision 6 at its Step 6 (the swarm repository created or adopted with the maintainer's token, cloned, seeded, committed and pushed, `<state>/env` written; decision [109](swarm/decisions/0109-init-creates-and-seeds-the-swarm-repository.md)), a variable unset at `init` written commented out at Step 6a (decision [110](swarm/decisions/0110-an-unset-secret-is-written-commented-out.md)), question 3's default from `--api` or `$GITEA_API_URL`/`$GITHUB_API_URL` at Step 6b (decision [111](swarm/decisions/0111-init-learns-a-self-hosted-forges-api-address.md)), the forge defaults following question 2's kind, the unusable-prompt warning, a blank variable taken as unset and the quoted unit at Step 20 of the plan that clears the Deferred list (decision [120](swarm/decisions/0120-declare-forge-and-runner-loose-ends.md)), docs/swarm.md section 11, Finding the swarm's and init's As built): Go (`internal/swarm/declare`, `internal/swarm/forge`, `internal/swarm/cmd/swarm.go`, `internal/swarm/cmd/swarm_init.go`); the contract is [docs/swarm.md](swarm.md) section 11, with `swarm.json` in section 3 and the forge calls in section 10 |
| Legacy | none |
| Usage | `daisy swarm init [--defaults] [--api <url>]`: in the swarm repository's clone, or in any folder holding none (the clones directly under it giving the defaults, the clone made there as `<repo>`; docs/swarm.md section 11), asks the wizard's questions (below) through `Ops.Prompt` on stderr, an empty answer taking the default in brackets (the existing `swarm.json`'s value, else detection, else section 3's example); `--defaults` asks nothing; `--api <url>` is question 3's (the forge API's) default, over the file's, with or without `--defaults` |
| Exit codes | 0 written or unchanged, or cancelled at EOF (`Cancelled — no answer on stdin (EOF); nothing written.`); 1 an invalid file or default (`Error: swarm.json: <field>: <problem>`), a token unset (`Error: swarm: $<VAR> is not set (in <state>/env or the environment)`), `DAISY_MAINTAINER_TOKEN` the swarm's or its account, kind `github` (no adapter yet), the maintainer's token unable to read, create or open the swarm repository (`Error: swarm: $DAISY_MAINTAINER_TOKEN cannot …`), the clone refused (`<dir>/<repo>` not a clone of the swarm repository, or one holding `swarm.json`, which `init` is run in instead), the swarm's token unable to read the swarm repository, a declared repository or its own account (`Error: swarm: $<VAR> cannot read …`), a write failing, the push refused (the commit stays in the clone), a mirror's fetch failing, `EnsureLabels` failing (a refused rename or create names every label still to rename and to create; the files stay written and a re-run completes it); 2 usage. A readiness warning never changes it |
| Stdout | each write named with its token: `✓ Maintainer <login>: <owner>/<repo> created` (or `  … exists`) and `✓ Maintainer <login>: <swarm-login> may write to <owner>/<repo>`; `✓ Cloned <owner>/<repo> into <swarm>` when it cloned; `✓ Token $<VAR> reads <owner>/<repo> as <login>`; `✓ Wrote swarm.json` or `  Unchanged swarm.json`; per role `✓ Wrote prompts/<role>.md` or `  Kept prompts/<role>.md (exists)`, and so for each seeded file; `✓ Committed and pushed <short sha> to <owner>/<repo>'s <branch> as <git author>` or `  Nothing to commit`; `✓ Wrote <state>/env (mode 0600)` or `  Kept <state>/env (exists)`; `✓ Mirrors: <repo>, <name>, … fetched`; `✓ Token $<VAR>: labels 17 present (<c> created, <r> renamed)`; the systemd user service that runs `apply`'s loop (restarted one `interval` after a failure; a stop waiting one `timeout`, `TimeoutStopSec`, for the drain: Step 35 of the swarm build; its `Environment=` and `ExecStart=` values quoted and escaped as systemd reads a word), and its install line, which names where to save the unit |
| Stderr | the questions; a refused answer's message, then the question again; `⚠️  <state>/env <problem>` for a line the swarm's secrets file cannot be read past; `⚠ <state>/env: $<VAR> is not set; its line is commented out: fill it in, or set $<VAR> in apply's environment` for each variable unset (or empty, or blanks alone) in its environment, written commented out; `⚠ prompts/<role>.md <is empty|is a link to nothing|cannot be read (<error>)>; apply refuses it: …` for a kept prompt validation refuses; the four readiness warnings (`⚠ Readiness: …`), and each line of the comparison with CI at a repository's `base` as `⚠ Readiness: <name>: CI differs from the declared check environment: <difference>`, after that repository's own (docs/swarm.md section 7.9), never a refusal |
| Ops | Real |
| Writes | on the forge with `DAISY_MAINTAINER_TOKEN`, the swarm repository when absent and the swarm's account's write access to it; the swarm repository's clone when `init` runs outside one; in the clone, `swarm.json` when the answers differ from it and each other file of docs/swarm.md section 2's table (`prompts/<role>.md`, `repos/<name>.md`, the READMEs, `.gitignore`) that is absent, then one commit of those files and of each file of that table the clone holds and its `HEAD` lacks, and its push; `<state>/env` when absent, mode `0600`, a line `export <VAR>='<value>'` per variable and `# <VAR>=` for one unset in its environment, which assigns nothing, so the environment supplies it; the mirrors under `<state>/git/`; on the forge with the swarm's token, the organization's labels of the seventeen (the nine stages, the flags `swarm:gated`, `swarm:blocked`, `swarm:paused`, `swarm:blocking`, `swarm:class-architecture` and `swarm:question`, the terminals `swarm:done` and `swarm:rejected`: docs/swarm.md section 5), each of the seven stage labels found under its name before Step 15 of the swarm build renamed in place (decision [74](swarm/decisions/0074-the-label-vocabulary.md)) and every other missing one created |
| Commits | in the swarm repository's clone, the files it wrote and each file of docs/swarm.md section 2's table the clone holds and its `HEAD` lacks (an earlier run's whose commit failed), by the clone's own git identity (the maintainer's), its message `swarm: written by init`, pushed to the repository's default branch |
| Cases | none; `TestSwarmInitLabelsThroughGitea`, `TestSwarmInitCreatesThroughGitea`, `TestSwarmInitRefusals`, `TestSwarmInitCreatesTheSwarmRepository`, `TestSwarmInitAdoptsTheSwarmRepository`, `TestSwarmInitRefusesInAFolder`, `TestSwarmInitSeedsRepositoryNotes`, `TestSwarmInitReadinessCIDiffers`, and the `internal/swarm` tests |

The wizard's questions, in order (defaults in docs/swarm.md section 11):
1 the product repositories (forge names, `<owner>/<name>`, the product
first; each one's `ssh_command` taken from its clone when one lies
directly under the working folder); 2 the forge kind;
3 the forge API (`--api`'s value, else the file's while question 2
answered the file's kind, else `$GITEA_API_URL` or
`$GITHUB_API_URL` by kind when set and not empty, else detected from the
origin for that kind); 4 the swarm repository, which holds the issues; 5 the token variable
(the file's while the kind is the file's, else by kind); 6 the base
branch; 7–26 per role (owner, architect, worker, adversary, sympathetic)
its provider (`anthropic`, `openai` or `xai`; a role moved to another
provider takes that provider's model as its default, and a provider no
`keys` entry names gets its default credential: `openai` a login slot,
`xai` its API key; `keys` itself is not asked, and a file's form, an
API key or a login slot for either, is kept: docs/swarm.md sections 8
and 11), model (every role names
one: no `-`), effort and co-author (the `Co-Authored-By:` value,
`<model> <noreply@anthropic.com>`, `<noreply@openai.com>` or
`<noreply@x.ai>` by default, `-` for none: no trailer); 27 each repository's image; 28 max
members; 29 max rounds (5 since docs/swarm.md revision 5, built at
Step 4 of the swarm build); 30 the timeout; 31
the interval; 32 the resources (paths in the swarm repository, or
`<name>:<path>` in a declared one; `briefs/` and `docs/` by default; a
file's note on a path the answer keeps is kept);
then, once both tokens have read the forge and their accounts, 33 the
maintainer's logins (the swarm's refused), 34 the commit email (the
account's by default) and 35 whether the swarm repository's `notes/`
keeps the swarm's research notes (`yes` or `no`, the file's `notes` by
default, else `no`; Step 23 of the swarm build). Question
1's default leaves out a clone whose `origin`'s repository name begins
`swarm-`, the swarm repository by convention (Step 30 of the swarm
build), and question 4's default is the clone `init` runs in, else the
first such clone's repository on the forge's host (Step 35), else
`<owner>/swarm-<name>` after the first product. A new
repository entry's `protected` default names `.claude/` and `.grok/`
beside the forges' workflows, `AGENTS.md` and `CLAUDE.md` (Step 35).

### daisy swarm prompt build

| Field | Value |
|---|---|
| Status | as built (Step 7 of the plan that clears the Deferred list, 2026-11-12): Go (`internal/swarm/declare/prompts.go`, the table of earlier defaults and `ComparePrompt`; `internal/swarm/cmd/swarm.go`); the contract is [docs/swarm.md](swarm.md) section 11, prompt build, and decision [117](swarm/decisions/0117-swarm-prompt-build-reseeds-the-prompts-from-the-defaults.md) |
| Legacy | none |
| Usage | `daisy swarm prompt build [--force] [--dry-run]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); each `prompts/<role>.md` compared with this Daisy's default for its role: the current default left as it is, an earlier default (its sha256 in the binary's table of every default the role has had) or an absent prompt written as the current, an edited one kept with its diff from the default printed; `--force` overwrites an edited one too; `--dry-run` runs read-only, writes nothing and wins over `--force`, which it previews; any positional argument is usage on stderr, exit 2 |
| Exit codes | 0 every prompt current or written; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, the root's commit, a prompt or the origin's default branch unreadable, a write, the commit, the push or the mirror's fetch failing (the push refused: `Error: swarm: push <swarm>: <git's error>; the commit stays in the clone: pull, then push it`), and an edited prompt kept, reported after the others are written, committed and pushed (`Error: swarm: edited, so kept: prompts/<role>.md, … (each diff above); merge the default's changes in by hand, or run 'daisy swarm prompt build --force' to overwrite them`), under `--dry-run` too; 2 usage |
| Stdout | per role, in order: `  Unchanged prompts/<role>.md`, `✓ Rebuilt prompts/<role>.md (an earlier default)`, `✓ Wrote prompts/<role>.md` (absent), `  Kept prompts/<role>.md (edited), its diff from the default:` or with `--force` `✓ Rebuilt prompts/<role>.md (edited, overwritten by --force), its diff from the default:`, each followed by the unified diff (`--- prompts/<role>.md (default)`, `+++ prompts/<role>.md`); under `--dry-run`, `  Would write` and `  Would rebuild` for the writes, then `  Dry run: nothing written; would commit "<message>" and push it` or `  Dry run: nothing to write`; else `✓ Committed and pushed <short sha> to <owner>/<repo>'s <branch> as <git author>` and `✓ Mirrors: <repo> fetched`, or `  Nothing to commit` |
| Stderr | errors |
| Ops | Real; `ReadOnly` with `--dry-run` |
| Writes | in the clone, the prompts above; the swarm repository's mirror, `<state>/git/<repo>`, fetched after the push; no forge call, no secret read |
| Commits | in the swarm repository's clone, the prompts it wrote and nothing else, by the clone's own git identity, its message `swarm: prompts rebuilt from the root at <short sha> (<role>, …)`, `<short sha>` the Daisy root's `HEAD`, pushed to the repository's default branch as its origin's `HEAD` names it (`git ls-remote --symref origin HEAD`); nothing written, no commit |
| Cases | none of its own; case 92 (`swarm pause`'s) runs its `--dry-run`; `TestSwarmPromptBuild`, `TestComparePrompt`, `TestDefaultHistoryHoldsEachCurrentDefault` |

### daisy swarm status

| Field | Value |
|---|---|
| Status | as built (Steps 4, 8, 9, 14, 15, 20, 27 and 29 of the swarm build; finding the swarm and `<state>/env` at Step 2 of revision 6's plan, the briefs read in the mirrors at Step 4, Drift at Step 5): Go (`internal/swarm/forge`, `internal/swarm/reconcile`, `internal/swarm/brief`, `internal/swarm/cmd/swarm.go`); the contract is [docs/swarm.md](swarm.md) section 11; `--waiting`, and Waiting for you and Held moved last, since Step 14 of the plan remediating the 2026-11-15 design review; the `reverted` line since its Step 29 |
| Legacy | none |
| Usage | `daisy swarm status [--json] [--waiting]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); any positional argument is usage on stderr, exit 2 |
| Exit codes | 0, also when Docker cannot be reached; 1 no swarm found, `swarm.json` absent or invalid (`Error: swarm.json: <field>: <problem>`), the token unset (`Error: swarm: $<VAR> is not set (in <state>/env or the environment)`), a forge call failing (`Error: swarm: forge: …`), a brief, `<state>/auto`, a run's `result` or a batch run's `out.md` unreadable, or a mirror's history git cannot read for the `reverted` line (`Error: swarm: reading the reverts in the mirrors: …`); 2 usage; with exit 1, nothing on stdout: the report is written whole once every read is made. With `--json` the same: a forge it cannot reach (`Error: swarm: forge: …`) is exit 1 with no document, partial or empty, since a report without the forge's facts would read as a swarm with nothing waiting |
| Stdout | `Swarm: <owner>/<repo> on <forge-host> (<kind>), repositories <name>, …`; `State: <state>` with the paused and held notes (` — owner batch held: no brief is active` while no brief under `resources` is `active`: the owner's batch waits for one, docs/swarm.md section 6; a hold that waits on issues followed by ` (released once #<n> … are closed)`); `Gates: …`; a line per stage, `  %-26s %d` and its issue numbers; `Running: …`, `none` or `unknown (docker: <message>)`; `Personas:` and a line per role; `Throughput:` and its seven lines (`reverted` the swarm pull requests merged on `base` and those a revert there names, read from the mirrors as their last fetch left them, docs/swarm.md section 11; `auto` the gates Daisy passed for routine jobs, per issue the larger of its lines in `<state>/auto` and its auto notes on the open and recently closed issues; `stage time` the median run time per stage, a run recorded before Step 15 of the swarm build counted under its stage's new name), then a `#<n>` line of correction counts per open or recently closed issue with a plan, a pull request or a block; `Briefs:` and a line per `active` brief under `resources` (its milestones' open and closed issues as the forge counts them in the milestones that mirror them, those with none or not yet on the forge, and `met (owner run <k>)` when the newest batch run reports it; one whose file name an earlier `active` brief has, `<path>: shares the name <name> with <other path>; mirrored to no milestone`), or `Briefs: none active`; last, nearest the prompt, `Waiting for you: …`, the open issues the predicate `apply` sets `swarm:gated` by holds for (`triage`, `plan`, a `review-2-approved` issue under `merge`, `next slice`, `close` or, its current pull request closed unmerged, `pull closed` as that pull request calls for, `question` every issue carrying `swarm:blocked` whose newest blocking note passed a member's question to the maintainer, `blocked` every other issue carrying `swarm:blocked`, at whatever stage, and `notes` each `swarm/notes-<n>-<role>-<k>` branch the swarm repository's mirror holds: a run's notes that changed the same lines as the head, to merge by hand, docs/swarm.md section 7.6, and `no check` each declared repository whose `check` is empty while `gates.merge` is false, whose pull requests merge with no check of Daisy's run, docs/swarm.md section 3, and `ci differs` each declared repository whose CI differs from its declared check environment at `base`, read from its mirror as the last fetch left it, docs/swarm.md section 7.9), each `#<n>` under `triage` and `plan`, where only an architecture job waits, and `#<n> (<class>)`, its job class (`routine` or `architecture`, docs/swarm.md section 5) beside it, under the others, or `nothing`; `Held: …`, `paused #…` for the issues carrying `swarm:paused` and `#<n> blocked by #<m>` for each whose newest maintainer comment names an open blocker, separated by `; `, or `nothing`. With `--waiting`, the `Waiting for you: …` line alone. With `--json`: the report as one document (below) |
| Stderr | with `--json`, empty on success, each line below then under `warnings`; `⚠️  <state>/env <problem>` for a line the swarm's secrets file cannot be read past; `⚠ <swarm>/.env.sh: …`; in a swarm repository's clone, each time, `⚠ Drift: <path> differs from <owner>/<repo>'s <branch> at <short sha>; commit and push it` (`is not in`, `is only in`) per path of `swarm.json` and `prompts/` that differs from the swarm repository's default branch as its mirror was last fetched (docs/swarm.md section 11, Drift; never an exit code); `⚠ #<n>: Blocked by #<m> names no issue; not held` (no hold listed for it); errors |
| Ops | ReadOnly, with a forge `GET` or `HEAD`, `docker ps` and (revision 6) git's reading commands on a mirror (`for-each-ref` too, for the notes' branches) let through |
| Writes | nothing, anywhere |
| Commits | none |
| Cases | none; `TestSwarmStatusThroughGitea`, `TestSwarmStatusStages`, `TestSwarmStatusStep7`, `TestSwarmStatusCorrections`, `TestSwarmApplyGatedFlag`, `TestSwarmApplyRoutineJob`, `TestSwarmStatusWritesNothingOnAFailure`, `TestSwarmStatusPullClosedUnmerged`, `TestSwarmStatusBriefsShareAName`, `TestSwarmStatusAutoKeepsOldJobs`, `TestSwarmApplyNoteOnAMovedIssueBlocksNothing`, `TestSwarmDrift`, `TestSwarmApplyResearchNotesRebased`, `TestSwarmStatusJSON`, `TestSwarmStatusJSONThroughGitea`, `TestSwarmStatusWaiting`, `TestSwarmStatusWaitingThroughGitea`, `TestSwarmStatusReverts`, `TestSwarmMergeWithoutACheck`, `TestSwarmCIDiffersTold`, `TestSwarmCIDiffersServicesFile`, `TestSwarmCIDiffersLaterDirectory`, and the other `internal/swarm` tests |

`swarm status --json` (Common behaviour, `--json`) carries the text's
facts by line: `repository`, `forge` (`host`, `kind`) and
`repositories`, the Swarm line; `state`, the State line: `path`,
`paused` (`since`, `reason`, `""` when it gives none), `restart_requested_at`,
`owner_held` (`since`, `reason`, `waits`, the issues whose closing
releases it), each `null` without its file, `no_active_brief`, and
`qa_held` (`since`, `reason`, `waits` always `[]`), `null` without
its file;
`gates` (`triage`, `plan`, `merge`, booleans); `stages`, each of the nine
stage labels and its open issues; `waiting`, `issues` (each `item`, one
of the line's words, `triage` to `blocked`, `issue` and `class`, given
under `triage` and `plan` too) in the line's order, `notes`, the
notes' branches, `no_check`, the repositories with no check, and
`ci_differs`, those whose CI differs from their declared check
environment (docs/swarm.md section 7.9); `held`, `paused` and `blocked_by` (each `issue`, `by`);
`running`, `members` (each `issue`, `role`) and `error`, docker's
message when it cannot be reached, else `null`; `personas`, each role's
`provider`, `model` and `effort`; `throughput`: `runs` (`total`,
`posted`, `blocked`, `questions`, `failed`, `faults`, `other`), `cost` (`today`,
`all`, in dollars, `runs_without_cost`, `estimated`, in dollars at list
price, `runs_without_estimate`; the three sums rounded to six decimals), `budget` (`per_day`, `total`, or
`null`), `approved` (`pull_requests`, `at_round_limit`,
`blocked_at_round_limit`), `reverted` (`merged_pulls`,
`reverted_pulls`, `fetched`, RFC 3339 or `null` for `no mirror
fetched` and `fetch time unknown`), `auto` (`triage`, `plan`) and `stage_time`,
by `batch`, a stage's label or `check`, only those with a run, each
`median_seconds` and `runs`; `corrections`, the `#<n>` lines, each
`issue`, `plan_revisions`, `blocks` and `pulls` (each `repo`, `number`,
`rounds`, `maintainer_comments`); `briefs`, the active ones in path
order, each `path`, `name` (the milestones' prefix), `shares_name_with`
(the earlier active brief with its name, else `null`), `milestones` in
heading order (each `heading`, `on_forge`, `open`, `closed`; `[]` for a
brief that shares a name) and `met_run` (the batch run reporting it met,
else `null`); and `warnings`. With `--waiting`, the document is
`{"waiting", "warnings"}`, `waiting` as the whole document gives it: the
keys the text prints and no other, since a key left empty would read as
a fact (no stage holds an issue, nothing runs), so a caller reads
`waiting` alike from either. Abridged:

```json
{
  "repository": "acme/swarm-app",
  "forge": {"host": "forge.example.net", "kind": "gitea"},
  "repositories": ["app", "lib"],
  "state": {"path": "<state>", "paused": null, "restart_requested_at": null, "owner_held": null, "no_active_brief": false, "qa_held": null},
  "gates": {"triage": true, "plan": true, "merge": true},
  "stages": {"swarm:proposal-posted": [1, 2], "swarm:plan-0-posted": [3], "swarm:review-2-approved": [6]},
  "waiting": {
    "issues": [
      {"item": "triage", "issue": 1, "class": "architecture"},
      {"item": "merge", "issue": 6, "class": "routine"}
    ],
    "notes": [],
    "no_check": [],
    "ci_differs": []
  },
  "held": {"paused": [11], "blocked_by": [{"issue": 12, "by": 3}]},
  "running": {"members": [{"issue": 8, "role": "adversary"}], "error": null},
  "personas": {"owner": {"provider": "anthropic", "model": "claude-opus-5-5", "effort": "high"}},
  "throughput": {
    "runs": {"total": 1, "posted": 1, "blocked": 0, "questions": 0, "failed": 0, "faults": 0, "other": 0},
    "cost": {"today": 0.5, "all": 0.5, "runs_without_cost": 0, "estimated": 0, "runs_without_estimate": 1},
    "budget": null,
    "approved": {"pull_requests": 0, "at_round_limit": 0, "blocked_at_round_limit": 0},
    "reverted": {"merged_pulls": 0, "reverted_pulls": 0, "fetched": "2026-11-16T08:00:00Z"},
    "auto": {"triage": 0, "plan": 0},
    "stage_time": {"swarm:proposal-accepted": {"median_seconds": 600, "runs": 1}}
  },
  "corrections": [{"issue": 3, "plan_revisions": 0, "blocks": 0, "pulls": []}],
  "briefs": [
    {"path": "docs/briefs/roadmap.md", "name": "roadmap", "shares_name_with": null,
     "milestones": [{"heading": "M0", "on_forge": true, "open": 1, "closed": 0}], "met_run": null}
  ],
  "warnings": []
}
```

### daisy swarm list

| Field | Value |
|---|---|
| Status | as built (Step 15 of the plan for the inbox requests F27-F35): Go (`internal/swarm/cmd/swarm_connect.go`); the contract is [docs/swarm.md](swarm.md) section 11; `--json` since Step 10 of the plan remediating the 2026-11-15 design review |
| Legacy | none |
| Usage | `daisy swarm list [--json]`: anywhere, in a clone or not, with no Daisy workspace needed; any positional argument is usage on stderr, exit 2 |
| Exit codes | 0, also with no swarm; 1 a directory under `$DAISY_LOCAL/swarm` unreadable; 2 usage |
| Stdout | `Swarms on this machine ($DAISY_LOCAL/swarm):`, then per `<state>` in path order `  <owner>/<repo> on <forge-host>: clone <path>; home <name>; not paused` (`clone not recorded`, `clone <path> (not a clone of it now)`; `no home`; `paused since <time>[: <reason>]`), and `; here <swarm>` when the working folder is in a clone of that swarm; with none, `No swarm on this machine ($DAISY_LOCAL/swarm holds no swarm's state)`. With `--json`: one document (below) |
| Ops | ReadOnly |
| Writes | nothing |
| Commits | none |
| Cases | none of its own; case 92 (`swarm pause`'s) runs it; `TestSwarmConnectBriefing`, `TestSwarmListStates`, `TestReadOnlyJSON` |

`swarm list --json` (Common behaviour, `--json`) has `dir`,
`$DAISY_LOCAL/swarm`; `swarms`, in the text's order, `[]` with none,
each `repository`, `host`, `clone` (`{"path", "current"}`, `current`
`false` for the text's `(not a clone of it now)`, `null` for `clone not
recorded`), `home` (`null` for `no home`), `paused` (`{"since",
"reason"}` as `swarm status --json` gives it, `null` for `not paused`)
and `here` (the clone the working folder is in, else `null`); and
`warnings`:

```json
{
  "dir": "/srv/local/daisy/swarm",
  "swarms": [
    {"repository": "acme/swarm-app", "host": "forge.example.net", "clone": {"path": "/srv/swarm-app", "current": true},
     "home": "work", "paused": {"since": "2026-11-06T15:00:00Z", "reason": "swarm fault: docker gone"}, "here": null}
  ],
  "warnings": []
}
```

### daisy swarm journal

| Field | Value |
|---|---|
| Status | as built (Step 15 of the plan for the inbox requests F27-F35): Go (`internal/swarm/cmd/swarm_connect.go`, `internal/swarm/reconcile/record.go`); the contract is [docs/swarm.md](swarm.md) section 11; `--json` since Step 10 of the plan remediating the 2026-11-15 design review, `--issue` since its Step 14 |
| Legacy | none |
| Usage | `daisy swarm journal [--json] [--issue <n>] \| --mark <run>`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); a positional argument is usage on stderr, exit 2 |
| Exit codes | 0; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, the journal unreadable, with `--mark` a run the journal holds no line of (`Error: swarm journal --mark: the journal holds no line of <run>; mark a run its lines name`) or the write failing; 2 usage, `--json` with `--mark` among it (`Error: --json prints the journal and moves no mark; give --mark alone`, then the usage), `--issue` with `--mark` (`Error: --issue filters the journal it prints and moves no mark; give --mark alone`), `--issue` with a word that is no issue number, 0 among them (`Error: not an issue number: <word>`) |
| Stdout | the lines after the one `<state>/connect-seen` marks: `Journal of <owner>/<repo> since <run> (marked seen <time>): <k> lines`, each line as `<YYYY-MM-DD> - <HHMM> <run>: …`, then `Mark them seen: daisy swarm journal --mark <run>` naming the last; none after it, `Journal of <owner>/<repo>: nothing since <run> (marked seen <time>)`; no mark, or one the journal does not hold, the newest day's lines under `Journal of <owner>/<repo>, <YYYY-MM-DD> (no line marked seen): <k> lines`; an empty journal, `Journal of <owner>/<repo>: no line yet`. With `--issue <n>`, of the lines it would show only those whose run names issue `<n>` (`<n>-<role>-<k>`, `<n>-command-<id>`; a line of no issue, `0-owner-<k>`, `0-restart-<time>`, `0-release-<k>`, never), the count followed by ` on #<n>` (`nothing on #<n>` for none: `Journal of <owner>/<repo>: nothing on #<n> since <run> (marked seen <time>)`, `Journal of <owner>/<repo>, <YYYY-MM-DD> (no line marked seen): nothing on #<n>`), and no `Mark them seen` line: the mark is the whole journal's, and one issue's last line would mark the other issues' lines before it seen. With `--mark`, `✓ Marked the journal seen through <run> (<state>/connect-seen)`. With `--json`: one document (below) |
| Stderr | `⚠ <state>/connect-seen names <run>, which the journal does not hold: showing its newest day`; with `--json`, empty, the line under `warnings` |
| Ops | Real; `ReadOnly` without `--mark` |
| Writes | with `--mark`, `<state>/connect-seen`; else nothing |
| Commits | none |
| Cases | none; `TestSwarmConnectBriefing`, `TestReadOnlyJSON`, `TestSwarmJournalIssue` |

`swarm journal --json` (Common behaviour, `--json`) has `repository`;
`marked`, `{"run", "at"}`, the mark when the journal holds its line,
else `null`; `day`, the newest day when no line is marked and its lines
are shown, else `null` (both `null`: the journal has no line yet);
`lines`, the lines shown, each `day`, `run` and `line` (as the journal
holds it, `- <HHMM> <run>: …`), `[]` for none (`nothing since`), with `--issue` only that issue's; and
`warnings`. To mark them seen, `--mark` the last line's `run` (of the
document without `--issue`):

```json
{
  "repository": "acme/swarm-app",
  "marked": {"run": "1-architect-1", "at": "2026-11-05T10:00:00Z"},
  "day": null,
  "lines": [{"day": "2026-11-06", "run": "2-worker-1", "line": "- 1501 2-worker-1: posted; pull request #3"}],
  "warnings": []
}
```

### daisy swarm connect

| Field | Value |
|---|---|
| Status | as built (Step 33 of the plan remediating the 2026-11-15 design review, P21): Go (`internal/swarm/cmd/swarm_connect.go`, `internal/swarm/reconcile/briefing.go`); the contract is [docs/swarm.md](swarm.md) section 11 |
| Legacy | none |
| Usage | `daisy swarm connect [--json]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); any positional argument is usage on stderr, exit 2 |
| Exit codes | 0, also when the metrics cannot be read (the report says why); 1 as `swarm status` (no swarm found, `swarm.json` invalid, the token unset, a forge call failing, a run's file or a mirror unreadable), the journal unreadable, or `AUTHORITIES.md` present and unreadable (`Error: swarm connect: <error>`); 2 usage; with exit 1, nothing on stdout: the report is written whole once every read is made |
| Stdout | the briefing of `prompts/swarm-connect.md` rule 2 (docs/swarm.md section 11, `daisy swarm connect`): `Swarm: <owner>/<repo> on <forge-host>, clone <swarm>`, the `State:` and `Running:` lines as `swarm status` prints them; `Since the last connect: ` and `swarm journal`'s first line, its lines under `  #<n>` per issue in issue order and `  no issue` last, indented four spaces, and its `Mark them seen: daisy swarm journal --mark <run>`; `Waiting for you: …` as `swarm status` prints it, each issue in it followed by `  #<n>, its gate note:` and that note's text after its marker line, indented four spaces (`  #<n>: no gate note of this gating on the issue` when the issue carries none of its gating now); `Held: …`; `Runs: <k> (<p> posted, <b> blocked, <f> failed, <x> faults, <o> other)`; `Questions the swarm answers: #<n> for the <architect\|owner>; …` (or `none`), each with `  #<n>, <role> run <k> at <time>:` and the question, indented; `Authorities: <swarm>/AUTHORITIES.md, quoted verbatim; Daisy acts on none of it`, the file's bytes, `End of AUTHORITIES.md`, or `Authorities: none granted (no AUTHORITIES.md in <swarm>)`; `Metrics of the swarm's runs: <csv> (home <name>)`, `  since the last model or prompt change, <time> <sha> <subject> (the clone's newest commit to swarm.json or prompts/):` and a line per model, `  in all:` and a line per model (`    <model>  <k> rows, tokens <t>, cost $<c>, estimated $<e>`, sums as `daisy metrics` gives them), or `Metrics of the swarm's runs: none (<why>)`; each part after the first opens with a blank line. With `--json`: one document (below) |
| Stderr | as `swarm status` and `swarm journal`; with `--json`, empty on success, each line under `warnings` |
| Ops | ReadOnly, widened as for `swarm status` |
| Writes | nothing |
| Commits | none |
| Cases | none; `TestSwarmConnect`, `TestSwarmConnectThroughGitea`, `TestCurrentGateNote`, `TestSwarmMergeWithoutACheck`, `TestSwarmCIDiffersTold` |

It reads `AUTHORITIES.md` only to quote it: Daisy acts on none of it
(docs/swarm.md section 2, The authorities). `swarm connect --json`
(Common behaviour, `--json`) has `repository`, `host` and `clone`, the
Swarm line; `state` and `running` as `swarm status --json` gives them;
`journal`: `marked` and `day` as `swarm journal --json` gives them,
`issues`, each `issue` (`null` for the lines of no issue) and its
`lines` (each `day`, `run`, `line`), and `mark`, the run `--mark`
takes (`null` with no line); `waiting`, `swarm status --json`'s with
each issue's `gate_note`, the note's text after its marker line, or
`null`; `held` and `runs` (`total`, `posted`, `blocked`, `questions`,
`failed`, `faults`, `other`) as `swarm status --json` gives them; `questions`,
each `issue`, `answerer` and `asked` (`role`, `run`, `at`, `text`, or
`null`); `authorities`, `{"path", "text"}`, `text` the file's bytes,
or `null` with no file; `metrics`: `home` and `file` (`null` when not
read), `change` (`at`, `commit`, `subject`, or `null`),
`since_change` and `all`, each model's `model` (`null` when left
empty), `rows`, `tokens`, `rows_with_tokens`, `cost`, `rows_with_cost`,
`estimated` and `rows_with_estimated` (a sum `null` when no row has
it), and `problem`, why there are none, else `null`; and `warnings`.
Abridged:

```json
{
  "repository": "acme/swarm-app", "host": "forge.example.net", "clone": "/srv/swarm-app",
  "state": {"path": "<state>", "paused": null, "restart_requested_at": null, "owner_held": null, "no_active_brief": false, "qa_held": null},
  "running": {"members": [], "error": null},
  "journal": {"marked": null, "day": "2026-11-06", "mark": "0-owner-2", "issues": [
    {"issue": 2, "lines": [{"day": "2026-11-06", "run": "2-architect-1", "line": "- 1502 2-architect-1: posted; the plan; #2"}]},
    {"issue": null, "lines": [{"day": "2026-11-06", "run": "0-owner-2", "line": "- 1503 0-owner-2: posted; batch"}]}]},
  "waiting": {"issues": [{"item": "plan", "issue": 2, "class": "architecture",
    "gate_note": "**reconciler:** gated at plan-0-posted: the plan gate holds the plan until the maintainer accepts it.\n\n- Class: architecture.\n…\n\nDecide: `daisy swarm accept 2`, …"}], "notes": [], "no_check": [], "ci_differs": []},
  "held": {"paused": [3], "blocked_by": []},
  "runs": {"total": 3, "posted": 2, "blocked": 0, "questions": 1, "failed": 0, "faults": 0, "other": 0},
  "questions": [{"issue": 1, "answerer": "architect",
    "asked": {"role": "worker", "run": 1, "at": "2026-11-06T15:02:00Z", "text": "QUESTION: Should main.go say v1 or v1.0?\n"}}],
  "authorities": {"path": "/srv/swarm-app/AUTHORITIES.md", "text": "# Authorities\n\nA1. …\n"},
  "metrics": {"home": "work", "file": "/srv/work/metrics/metrics.csv",
    "change": {"at": "2026-11-06T15:01:30Z", "commit": "e9c2508", "subject": "Worker: keep main.go short"},
    "since_change": [{"model": "claude-fable-5-1", "rows": 1, "tokens": 30, "rows_with_tokens": 1, "cost": 0.25, "rows_with_cost": 1, "estimated": 0.001, "rows_with_estimated": 1}],
    "all": [{"model": "claude-fable-5-1", "rows": 2, "tokens": 60, "rows_with_tokens": 2, "cost": 0.5, "rows_with_cost": 2, "estimated": 0.002, "rows_with_estimated": 2}],
    "problem": null},
  "warnings": []
}
```

### daisy swarm apply

| Field | Value |
|---|---|
| Status | as built (Steps 5, 6, 9, 14, 15, 18, 19, 20, 23, 24, 25, 26, 27, 29 and 32 of the swarm build; finding the swarm, `<state>/env` and `docker` and `id` in the swarm's root at Step 2 of revision 6's plan, the resources, references and briefs read from the mirrors at the pass's commits at Step 4, `repos/<name>.md` in a member's `system.md` and Drift at Step 5): Go (`internal/swarm/provider`, `internal/swarm/runner`, `internal/swarm/reconcile`, `internal/swarm/brief`, `internal/swarm/cmd/swarm.go`); the contract is [docs/swarm.md](swarm.md) section 11, whose As built paragraph says what a pass leaves out (the board and the budget) |
| Legacy | none |
| Usage | `daisy swarm apply [--once]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); with `--once`, one pass, its runs waited for and collected (the maintainer's smokes); without, the loop the unit `init` prints runs: a pass every `interval` whatever runs, each run collected by the first pass after it ends, until SIGINT or SIGTERM; any other argument is usage on stderr, exit 2 |
| Exit codes | 0 the pass or passes ran (a member's failure is on the forge, not in the exit code; a pause on stdout); 1 no swarm found, `swarm.json` absent or invalid at the start, the token or an `env` credential unset (a slot without its login is not checked at the start: it is the swarm fault *slot refused* of the run that takes the slot, `fault: slot <provider>/<j>: no login; run '<login>'`, the login line `CODEX_HOME=<slot> codex login -c cli_auth_credentials_store=file` for `openai` and `GROK_HOME=<slot> grok login --device-auth` for `xai`, docs/swarm.md section 8), `docker ps` failing at the start (`Error: swarm: docker: <message>`), the lock held (`Error: swarm apply is already running for <owner>/<repo> (<holder line>); lock: <state>/lock`), `EnsureLabels` failing (a refused rename or create among the causes), and with `--once` a forge, git or `docker ps` error during the pass (`Error: swarm: …`; without `--once` it is printed and the next pass runs; a worker run's fetch from its clone failing is one the first time, and the second time in a row that run's failure *fetch failed*, its issue blocked (an abandoned run's: its commits not kept, the run collected); a run's clone failing, or a resource no longer passing its checks, at a launch is not one but that run's swarm fault, *clone failed* or *resource refused*, docs/swarm.md section 7.7); 130 a second signal, during the pass the first ended or while the loop collects its running members; 2 usage |
| Stdout | `✓ Labels: 17 present (<c> created, <r> renamed)` at the start when `EnsureLabels` made a label; `#<n> <role> run <k>: launched` (the owner's batch: `owner run <k>: launched`), then for a worker, and a reviewer launched at `swarm:review-0-ready`, `#<n> <role> run <k>: <stage> → <in-progress stage>`; at collection, `#<n> <role> run <k>: ` then `posted comment`, `posted review (<verdict>)`, `check started`, `check exited <code>` or `check timeout`, `pushed swarm/<n>-<s> <short sha>`, `created pull <name>#<m>`, `<from> → <to>`, `blocked`, `question` (`swarm:question` set), `failed (<reason>)`, `fault (<reason>)`, `pushed notes <short sha>` (with `notes`, docs/swarm.md section 7.6, The notes); the batch's `owner run <k>: proposed #<n> <title>` or `owner run <k>: <reason>; batch held (<state>/owner-held)` (`; batch held until #<n> … are closed (…)` when its `BLOCKED: waiting on #<n>, …` named open issues), and `Owner batch released: #<n> … closed` at the first pass at which they all are (docs/swarm.md section 6, G0); with `qa` declared, the QA's `qa run <k>: launched`, `qa run <k>: proposed #<n> <title>`, `qa run <k>: no failure` or `qa run <k>: <reason>; qa held (<state>/qa-held)` (docs/swarm.md section 6, The QA trigger); `#<n> pull <name>#<m>: <label>` (in step (5), and after the flag for an issue the pass moved since, docs/swarm.md section 5); the merge step's `#<n> merged pull <name>#<m> (slice <s> of <S>)`, `#<n> swarm:review-2-approved → swarm:plan-1-accepted`, `#<n> closed (last slice merged)` then `#<n> swarm:review-2-approved → swarm:done`, `#<n> merge of pull <name>#<m> refused (<message>)` then `#<n> blocked` (`no head both reviews approve` its message when none is), `#<n> not mergeable: back to the worker` then `#<n> swarm:review-2-approved → swarm:review-3-changes-requested` (T25, the direction naming the files that conflict on the mirror), `#<n> not mergeable a second time at <short sha>: blocked` then `#<n> blocked` (T25's second time on one head), `#<n> merge of pull <name>#<m> refused (<message>); its mergeability is not settled, so the next pass tries again`, `#<n> merge of pull <name>#<m> refused (<message>); its head merges into <base> on the mirror, so the forge is taken to be recomputing and the next pass tries again`; a routine job's gate passed, `#<n> <stage> → <stage> (auto)` and `#<n> auto note posted` (the auto note, docs/swarm.md section 5); the maintainer's flag's `#<n> gated`, `#<n> gate note posted` (the gate note, at each gating event: the flag added, or, the flag on, a new stage or a new plan, docs/swarm.md section 5) and `#<n> gated cleared`; `#<n> question cleared` (an answer, or the question blocked for the maintainer, docs/swarm.md section 6, The question); a comment command's `#<n> <its first command line> by <login> (comment <id>): <outcome>` (`done`, `refused (<error>)`, `not run (more than one command)`, `not run (edited)`; docs/swarm.md section 6, The comment commands) and `#<n> comment <id>: answer posted again`; `Paused since <time>[: <reason>] (<state>/paused): launching nothing`; the milestones' `milestone created: <title>` and `#<n> milestone: <title>`; with `notify`, `Notified: <line>` for each new item waiting for the maintainer, the command having run with that line on stdin (docs/swarm.md section 5, Notify); in a swarm repository's clone, once its drift is gone, `✓ Drift cleared: swarm.json and prompts/ are <owner>/<repo>'s <branch> at <short sha>` (docs/swarm.md section 11, Drift: the drift lines on stderr at the first pass and then only when the set of drifting paths changes); `Pass <HH:MM:SS>: launched <a>, collected <c>, running <r>` |
| Stderr | `⚠️  <state>/env <problem>`; `⚠ <swarm>/.env.sh: …` and the drift warnings (docs/swarm.md sections 3 and 11); with `gates.merge` false, `⚠ swarm.json: repos[<i>] (<name>) has no check and gates.merge is false: no check of Daisy's runs on its pull requests before the merge step merges them` for each repository whose `check` is empty, at the first pass and again only when the set of them changes (docs/swarm.md section 3, A merge with no check); `⚠ <name>: CI differs from the declared check environment: <difference>` for each difference the comparison with CI finds at a repository's `base` in its mirror, likewise, and `⚠ comparing <name>'s CI: <error>` for a read git refuses (docs/swarm.md section 7.9); `⚠ #<n>: Blocked by #<m> names no issue; not held` (the forge holds no `#<m>`: it holds nothing, each pass while the comment stands), `⚠ #<n>: carries <label> and <label>; skipped`, `⚠ #<n>: <label> but its current pull request is not open; skipped`, `⚠ #<n> closed during the run; nothing posted`, `⚠ #<n>: relabelled to <label> during the run; not relabelled` (`during the merge step` for the merge step's), `⚠ #<n>: pull <name>#<m>'s head <short sha> not merged on the mirror (<error>); the forge's report stands`, `⚠ #<n> worker run <k>: commit <short sha> has no Co-Authored-By: <co_author>`, `⚠ #<n> worker run <k>: abandoned; commits not kept (<reason>)`; with `notes`, `⚠ #<n> <role> run <k>: notes conflict with <owner>/<repo>'s head; pushed to swarm/notes-<n>-<role>-<k>` and `⚠ #<n> <role> run <k>: notes not pushed (<reason>); kept in <run>/notes`; the flag's `⚠ #<n>: gated: <error>`, `⚠ #<n>: gate note: <error>`, `⚠ #<n>: class: <error>`, `⚠ #<n>: auto: <error>` and `⚠ gated: <error>`, the copy's `⚠ #<n>: pull request label: <error>`, the milestones' warnings (docs/swarm.md section 11) and notify's `⚠ notify: <line>: <error>[: <its output>]` and `⚠ notify: <error>; nothing told this pass` (docs/swarm.md section 5, Notify), never an exit code; errors |
| Ops | Real |
| Writes | `<state>/lock`, `<state>/paused` (two swarm faults in a row), `<state>/owner-held`, `<state>/qa-held`, `<state>/commands/<id>` (each comment command run, its answer: docs/swarm.md section 6), `<state>/auto` (a line per gate passed for a routine job), with `notify` `<state>/notified` (the items waiting for the maintainer it has told), `<state>/git/`, `<state>/references/` (revision 6), with `notes` `<state>/notes/` (the host's clone of the swarm repository), `<state>/runs/`, a slot's lock and the clearing of the slot; on the forge, issues, comments, reviews, pull requests, label changes, pushed `swarm/<n>-<s>` branches, with `notes` the research notes' commits pushed to the swarm repository's `notes/` directory (its default branch; on a conflict to `swarm/notes-<n>-<role>-<k>`), the seventeen labels when one is missing (an old stage name renamed in place), `swarm:class-architecture` on each of the batch's proposals but those it classes routine, a routine job's `swarm:proposal-posted` and `swarm:plan-0-posted` relabelled with an auto note, `swarm:gated` set and cleared on open issues, a gate note on each issue the flag is added to, `swarm:blocked` set on a failed or blocked run's issue and on a refused merge's, `swarm:question` set on the issue of a member's question and cleared at its answer, or with `swarm:blocked` set when it reaches the maintainer, a milestone per `active` brief's milestone and an issue's milestone where it has none, and with `gates.merge` false merges and closed issues, each closed issue then `swarm:done` |
| Commits | none |
| Cases | none; `TestSwarmApplyLifecycle`, `TestSwarmApplyLoopCollectsALaterPass`, `TestSwarmApplyCollectsARunAnotherApplyStarted`, `TestSwarmApplyGatesOff`, `TestSwarmApplyMilestones`, `TestSwarmApplyGatedFlag`, `TestSwarmApplyHolds`, `TestSwarmInProgressTakenBack`, `TestSwarmApplyResearchNotes`, `TestSwarmApplyResearchNotesMerges`, `TestSwarmApplyNotesInTheSwarmRepository`, `TestSwarmApplyOwnNotesNotADirectory`, `TestSwarmApplyOwnNotesNotADirectoryNothingWritten`, `TestSwarmApplyOwnNotesBecomeALink`, `TestSwarmSecondReviewerGoesHome`, `TestSwarmApplyContinuedPullRepositoryRemoved`, `TestSwarmApplyRoutineJob`, `TestSwarmBatchCountsRoutineProposals`, the comment commands' (`TestSwarmApplyCommand` and the others of `swarm_commands_test.go`), the pass's and the gates' (`TestSwarmApplyGateNoteOnEachGating`, `TestSwarmApplyBlockedGateNote`, `TestSwarmApplyMergeRefusedNoHead`, `TestSwarmFaultPauseOrder`, `TestSwarmApplyLoopTwoSignalsInAPass`, `TestSwarmApplyLoopSecondSignalAfterAPass`, `TestSwarmApplyLoopLeavesNoWatcher`, `TestSwarmApplyLoopSignalSkipsTheMergeStep`), the rounds' and the milestones' reads (`TestSwarmApplyDescriptionIsNoAnswer`, `TestSwarmApplyLoopMilestonesReadClosedOnce`), the collection's (`TestSwarmAbandonedWorkerKeepsItsCommits`, `TestSwarmOrphanEndedByTheExitMark`, `TestSwarmCheckRestartedAfterItsApplyDied` and the others of `swarm_collection_test.go`), the merge step's `TestSwarmMergeStepUnsettled`, `TestSwarmMergeStepRecomputing`, `TestSwarmMergeStepMirrorCannotTell` and `TestSwarmMergeStepSentBackTwice`, the warning of a merge with no check's `TestSwarmMergeWithoutACheck`, the comparison with CI's `TestSwarmCIDiffersTold` and `TestSwarmCIDiffersServicesFile`, and the other `TestSwarmApply` tests, and the `internal/swarm` tests |

### daisy swarm metrics

| Field | Value |
|---|---|
| Status | as built (Step 5 of the plan for the inbox requests F27-F35): Go (`internal/swarm/reconcile/record.go`, `internal/metrics`, `internal/swarm/provider`, `internal/swarm/cmd/swarm.go`); `--reprice` since Step 28 of the plan remediating the 2026-11-15 design review, a reviewer's `review_effect` since its Step 30, a member's stop's `cause` since Step 4 of the plan implementing the 2026-11-19 swarm roadmap and Daisy's own stops' since its Step 5, `--scorecard` (`internal/swarm/reconcile/scorecard.go`) since its Step 7, its lines by configuration since its Step R3-12, grouped by `config_hash` since its Step R3-12b; the contract is [docs/swarm.md](swarm.md) section 11 |
| Legacy | none |
| Usage | `daisy swarm metrics --backfill \| --reprice [--dry-run] \| --scorecard [--since <time>] [--until <time>]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); none of the three actions or more than one, `--dry-run` without `--reprice`, `--since` or `--until` without `--scorecard`, `--until` before `--since`, or an argument: usage on stderr, exit 2. `--scorecard` prints a row per merged slice and a summary (docs/swarm.md section 11, The scorecard), `--since <time>` (RFC 3339, as `daisy metrics` takes it) narrowing both to the slices merged and the records dated at or after it, `--until <time>` (the same form) leaving out the slices merged and the records dated after it, from the slices' rows as from the summary |
| Exit codes | 0 the rows written (none among them); 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, `<state>/home` naming no home (`Error: swarm metrics --<action>: <state>/home names no Daisy home, whose metrics CSV would take the rows; write a registered home's name in it`) or one that cannot be used (`Error: swarm metrics --<action>: <state>/home names <name>: <reason>`), the CSV's header not this Daisy's nor an older one, the metrics lock held past 30 seconds, a read or the write failing; `--scorecard`: `--since` or `--until` not an RFC 3339 time (`Error: --since must be an RFC 3339 time, such as 2026-11-12T14:05:00+01:00 (got: <value>)`, `--until` likewise), the forge token unset, or a mirror's, the CSV's or the forge's read failing (`Error: swarm metrics --scorecard: <reason>`); 2 usage |
| Stdout | `--backfill`: `✓ Backfilled <w> rows into <home>/metrics/metrics.csv (<c> runs collected, <h> with its row already)`, then `; <e> earlier reviews' rows superseded with their review_effect` when it superseded any. `--reprice`: `✓ Repriced <w> rows into <home>/metrics/metrics.csv (<c> runs collected, <h> with a row: <k> current, <u> with no usage to read)`; with `--dry-run`, a line per row it would write, `  <run> <provider> <model>: tokens_in <a> → <b>, tokens_cached <a> → <b>, tokens_total <a> → <b>, cost_estimated <a> → <b>` (the held row's cell, then the new row's; `—` for empty), a reviewer's (`adversary`, `sympathetic`) ending `, review_effect <a> → <b>`, and a row whose `config_hash` changes then `, config_hash <a> → <b>`, then `Would reprice <w> rows in <file> (…); nothing written`. `--scorecard`: `Scorecard of <owner>/<repo>: runs and interventions from <csv>, merges from the mirrors as last fetched, <time>` (`no mirror fetched`), a line per merged slice, `  #<n> slice <s> in <name>, merged <time>, <wall> after the issue's opening` (`the previous slice's merge`) `: runs …; tokens …; estimated $<e> at list price …; review rounds <d>; stops …; interventions …` (`  no slice merged` for none), and `Summary, since <time> until <time>` (the bounds given, `every record` for neither) ` (<d> days): <m> slices merged; <r> runs, <r/m> per merge; input <i/m> tokens per merge; worker stop rate <p>% (<s> of <w> worker runs); <v> interventions, <v/d> a day`, then `By configuration, ` and the same bounds `:`, a line per role and configuration of the window's runs, `  <role> <provider> <model> <effort>, config <hash>: runs <r>; tokens …; estimated $<e> at list price …; stops <s>`, a reviewer's ending `; review effect <f> (<k> of <r> runs known)` (`  no run` for none; since Step R3-12 of the plan implementing the 2026-11-19 swarm roadmap) |
| Stderr | `⚠ #<n> worker run <k>: lines not counted: <reason>` for a worker whose range its mirror cannot diff (its row written, the line cells empty); `⚠ #<n> <role> run <k>: cause left empty: "<word>" is not one of plan, prerequisite, main-red, environment, forge-fact, kept-work, ruling, scope, capacity` for a stopped run whose `Cause:` line names another word (docs/swarm.md 7.6, its row written, the cell empty); `⚠ #<n> <role> run <k>: no row: its result has no end line or started line to date it` |
| Ops | Real; `--scorecard` `ops.ReadOnly` |
| Lock | `$DAISY_LOCAL/metrics.lock`, as `metrics add` and `swarm apply` take it, held for one read and one rewrite of the CSV: every row is made, each worker's lines measured, before it is taken; `--dry-run` and `--scorecard` take none. Not `<state>/lock`: it runs beside a live `apply` |
| Writes | `<home>/metrics/metrics.csv` of the home `<state>/home` names, in one rewrite, nothing when there is no row to write: `--backfill`, one `swarm-run` row (`metrics add`'s columns, above) per collected run whose kind, swarm, run and role the CSV holds no row of, in the order of their times, then, in a second rewrite under the same lock, a row superseding each earlier review's whose known `review_effect` a backfilled review decides; `--reprice`, per collected run the CSV holds a row of, a row superseding the last one of its key when the usage `<run>/stdout` reports now gives other usage cells, or for a reviewer's run its role's runs on the issue give another `review_effect`, or its kept `<run>/io/system.md` gives another `config_hash` (docs/swarm.md section 11), in the order of the runs' names; `--dry-run` and `--scorecard`, nothing. No journal line; no forge call and no secret read but `--scorecard`'s token and `Issue(n)` for each issue whose first merged slice it prints |
| Commits | none |
| Cases | none; `TestSwarmMetricsBackfill`, `TestSwarmMetricsBackfillGoneShasAndNoHome`, `TestSwarmMetricsReprice`, `TestSwarmMetricsRepriceReviewEffect`, `TestSwarmConfigHash`, `TestScorecardRowPerMergedSlice`, `TestScorecardRefusals`, `TestReviewEffects`, `TestSwarmApplyRecordsReviewEffect`, `TestAppendSkipsHeldKeys`, `TestSupersede` |

### daisy swarm pause

| Field | Value |
|---|---|
| Status | as built (Step 6 of the swarm build; `<n>` at Step 15; finding the swarm at Step 2 of revision 6's plan): Go (`internal/swarm/cmd/swarm.go`, `internal/swarm/cmd/swarm_maintainer.go`); the contract is [docs/swarm.md](swarm.md) section 11 |
| Legacy | none |
| Usage | `daisy swarm pause [<n>] [--type <type>]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); without `<n>`, the whole swarm; with it, as the maintainer, issue `<n>` alone (the maintainer's verbs' start, below); `--type` records the intervention (The maintainer's verbs, below); a second argument but `--type <type>` is usage on stderr, exit 2 |
| Exit codes | 0 paused or already paused; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, the write failing, and with `<n>` the maintainer's verbs' refusals and a forge failure; 2 usage |
| Stdout | `Paused: no new member will launch (<state>/paused); running members finish and are posted` [92], or `Already paused since <time>` [92], then `: <reason>` when the file carries one; with `<n>`, `Maintainer <login> on <owner>/<repo>`, then `#<n> paused` or `#<n> already paused` |
| Ops | Real |
| Writes | without `<n>`, `<state>/paused` (and `<state>` when absent), no forge call, no secret read; with it, on the forge, `Relabel(n, "", "swarm:paused")` unless it carries it |
| Commits | none |
| Cases | 92; `TestSwarmPauseAndResume`, `TestSwarmMaintainerHolds`, `TestSwarmApplyHolds` |

### daisy swarm resume

| Field | Value |
|---|---|
| Status | as built (Step 6 of the swarm build; `<n>` at Step 15; finding the swarm at Step 2 of revision 6's plan): Go (`internal/swarm/cmd/swarm.go`, `internal/swarm/cmd/swarm_maintainer.go`); the contract is [docs/swarm.md](swarm.md) section 11 |
| Legacy | none |
| Usage | `daisy swarm resume [<n>] [--type <type>]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); without `<n>`, the whole swarm; with it, as the maintainer, issue `<n>` alone; `--type` records the intervention (The maintainer's verbs, below); a second argument but `--type <type>` is usage on stderr, exit 2 |
| Exit codes | 0; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, a removal failing, and with `<n>` the maintainer's verbs' refusals and a forge failure; 2 usage |
| Stdout | `Resumed: members launch at the next pass` [92] when `<state>/paused` was present, `Owner batch released` when `<state>/owner-held` was, `QA released` when `<state>/qa-held` was; `Not paused` [92] when none was; with `<n>`, `Maintainer <login> on <owner>/<repo>`, then `#<n> paused cleared` or `#<n> not paused` |
| Ops | Real |
| Writes | without `<n>`, removes `<state>/paused`, whoever wrote it, `<state>/owner-held` and `<state>/qa-held`, no forge call, no secret read; with it, on the forge, `Relabel(n, "swarm:paused", "")` when it carries it |
| Commits | none |
| Cases | none of its own; case 92 (`swarm pause`'s) runs it; `TestSwarmPauseAndResume`, `TestSwarmApplyBatchTrigger`, `TestSwarmMaintainerHolds` |

### daisy swarm restart

| Field | Value |
|---|---|
| Status | as built (Step 19 of the plan for the inbox requests F27–F35): Go (`internal/swarm/cmd/swarm_restart.go`); the contract is [docs/swarm.md](swarm.md) section 11, daisy swarm restart and The restart |
| Legacy | none |
| Usage | `daisy swarm restart [--type <type>]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); `--type`, a leading value flag, records the intervention on no issue (The maintainer's verbs, below); an argument is usage on stderr, exit 2 |
| Exit codes | 0 requested or already requested; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, the write failing; 2 usage |
| Stdout | `Restart requested (<state>/restart): the running apply re-executes itself at its first pass with no member running`, or `Restart already requested at <time>` |
| Ops | Real |
| Writes | `<state>/restart` (and `<state>` when absent), no forge call, no secret read; the running `apply` loop removes it when it re-executes itself, at its first pass after which no member runs |
| Commits | none |
| Cases | none; `TestSwarmRestartRequested`, `TestSwarmApplyRestartsOntoANewBuild`, `TestSwarmApplyRestartExecFails` |

### The maintainer's verbs

`swarm triage`, `swarm accept`, `swarm class`, `swarm approve`, `swarm
merge`, `swarm unblock`, `swarm block`, `swarm replan` and `swarm close`, and `swarm pause
<n>` and `swarm resume <n>`, are the maintainer's writes on the forge
(docs/swarm.md section 11, built at Step 10 of the swarm build, `block`
and the issue's `pause` and `resume` at Step 15, `class` at Step 20,
`replan` at Step 20 of the plan for the inbox requests F27–F35), each
under the maintainer's own token, `DAISY_MAINTAINER_TOKEN`, resolved as
`check-secrets` resolves a variable (`<state>/env`, read never run, else
the environment); `status` never reads it, `apply` only to run a comment
command through these verbs (docs/swarm.md section 6, The comment
commands: `/swarm <verb> [<words>]` in a trusted maintainer's comment on
a swarm issue or its pull request, `<n>` the issue, answered in the
thread), and `init` only for its writes on the swarm repository
(docs/swarm.md section 8, rule 5). Each verb
starts the same way, before any write: `<n>` a positive issue number
(else usage, exit 2); `swarm.json` validated as `status` validates it;
the token set (`Error: swarm: $DAISY_MAINTAINER_TOKEN is not set (in
<state>/env or the environment)`) and not equal to the value of the
variable `forge.token_env` names (`Error: swarm:
$DAISY_MAINTAINER_TOKEN holds the swarm's token ($<VAR>); the
maintainer's verbs act with your own (docs/swarm.md section 8)`, before
the forge is reached); `Login()` under it, refused when the login is the
swarm's, `trusted`'s last (`Error: swarm: $DAISY_MAINTAINER_TOKEN is
<login>'s token, the swarm's account (trusted's last login); …`); then
`Maintainer <login> on <owner>/<repo>` on stdout and issue `<n>` read,
which must be open and carry exactly one stage label, the flags
(`swarm:gated`, `swarm:blocked`, `swarm:paused`, `swarm:blocking`,
`swarm:class-architecture`) and the terminals being none (`Error: swarm:
#<n> is closed; nothing changed`, `… carries no stage label; nothing
changed`, `… carries <label> and <label>; remove all but one on the
forge first`, `… is <label>, not <label>; nothing changed`). None writes
a file (but `unblock --cause`, its row in the home's metrics CSV, and
`--type`, below),
launches a member or runs a process (git aside, for the validation), or
commits; a refusal writes nothing. A relabel, or
`unblock`'s clearing of `swarm:blocked`, of an issue carrying
`swarm:gated` is followed by `Relabel(n, "swarm:gated", "")` and `#<n>
gated cleared` unless the labels it leaves are gated (the issue still
blocked, or unblocked at a gate that is on and holds its class: the flag
stays, as `apply` would set it); that removal refused is `⚠ #<n>:
swarm:gated not cleared (<error>); the next pass of apply clears it` on
stderr, exit 0. A close is followed by the terminal in the stage's
place, `Relabel(n, <stage>, "swarm:done")` after `merge`'s and
`Relabel(n, <stage>, "swarm:rejected")` after `close`'s, then the removal
of each flag it carries (`#<n> <flag> cleared`); a terminal refused after the close is `Error: swarm: #<n> is
closed, but not relabelled <terminal>: <error>; relabel it on the forge`,
and a flag's removal refused after the terminal `Error: swarm: #<n> is
closed and relabelled <terminal>, but <flag> is not cleared: <error>;
remove it on the forge` (exit 1). A comment a verb posts is not posted
when its thread's newest comment is already the maintainer's with that
text (`#<n> comment already posted`, `… on pull <name>#<m>`), so a verb
run again after a later write was refused posts it once; blank words
after `<n>` are no comment. Their writes carry no
marker: a comment opening with it is refused (`Error: swarm: the comment
opens with the swarm's marker, which only the swarm's entries carry;
nothing posted`).

**`--type <type>`** (Step 6 of the plan implementing the 2026-11-19
swarm roadmap; docs/swarm.md section 11, The interventions' record):
each of these verbs, and `swarm restart`, also takes `--type <type>`,
right after `<n>` (in either order with `unblock`'s `--cause` and
`--start` and `block`'s `--by`) or leading as a value flag, and from a comment
command right after the verb's name: the maintainer's intervention,
`<type>` one of the catalogue's `I1` to `I14`. A type outside them,
`--type` with no type or given twice is usage (exit 2) before anything
is read; `<state>/home` naming no usable home, or its metrics CSV not
this Daisy's, is `Error: swarm: --type: <reason>; nothing changed`
(exit 1) before the forge is read. Once the verb's writes are made
(exit 0), the intervention is recorded: a row of kind `intervention`
appended to `<home>/metrics/metrics.csv` under
`$DAISY_LOCAL/metrics.lock` (`step_or_run` `<n>-intervention-<k>`,
`<n>` `0` for the whole swarm's `pause`, `resume` and `restart`;
`role` `maintainer`; `outcome` the verb's name; `cause` the type;
`notes` `#<n>`), its line `- <HHMM> <n>-intervention-<k>: <type>;
<verb>; #<n>` queued in `<state>/journal` under
`<state>/journal.lock`, and `Intervention <n>-intervention-<k>
recorded: <type>` the last line of stdout; the record refused after the
writes is `Error: swarm: intervention not recorded: <reason>` (exit 1),
recorded then by `swarm intervene`. A verb refused records nothing.

### daisy swarm triage

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build; finding the swarm and `<state>/env` at Step 2 of revision 6's plan): Go (`internal/swarm/cmd/swarm_maintainer.go`); the contract is [docs/swarm.md](swarm.md) section 11 |
| Usage | `daisy swarm triage <n> [--type <type>]`: G1, T3, for an issue at `swarm:proposal-posted` |
| Exit codes | 0 relabelled; 1 the maintainer's verbs' refusals (above), a forge call failing (`Error: swarm: forge: …`); 2 usage |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<n> swarm:proposal-posted → swarm:proposal-accepted`, and `#<n> gated cleared` when it carried the flag and is not blocked |
| Ops | Real |
| Writes | on the forge, `Relabel(n, "swarm:proposal-posted", "swarm:proposal-accepted")`, then `Relabel(n, "swarm:gated", "")` when it carried the flag and is not blocked; `swarm:blocked` and `swarm:paused` are left as they are |
| Commits | none |
| Cases | none; `TestSwarmMaintainerVerbs`, `TestSwarmTriageThroughGitea`, `TestSwarmMaintainerRefusesTheSwarmsToken`, `TestSwarmMaintainerRefusals`, `TestSwarmMaintainerKeepsTheFlagWhereGated`, `TestSwarmMaintainerFlagRemovalRefused`, `TestSwarmMaintainerStep29Refusals` |

### daisy swarm accept

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build), as `swarm triage` |
| Usage | `daisy swarm accept <n> [--type <type>]`: G2, T7, for an issue at `swarm:plan-0-posted` |
| Exit codes | as `swarm triage` |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<n> swarm:plan-0-posted → swarm:plan-1-accepted`, and `#<n> gated cleared` when it carried the flag and is not blocked |
| Ops | Real |
| Writes | on the forge, `Relabel(n, "swarm:plan-0-posted", "swarm:plan-1-accepted")`, then the flag's removal when it carried it and is not blocked |
| Commits | none |
| Cases | none; `TestSwarmMaintainerVerbs`, `TestSwarmMaintainerRefusals`, `TestSwarmFlagClearedOnEveryTransition` |

### daisy swarm class

| Field | Value |
|---|---|
| Status | as built (Step 20 of the swarm build), as `swarm triage` |
| Usage | `daisy swarm class <n> [--type <type>] routine\|architecture`: the job class of issue `<n>`, at any stage (docs/swarm.md section 5): `architecture` sets `swarm:class-architecture`, `routine` clears it, and either then posts the class statement `Class: <class>`, which Daisy reads and no member answers; an issue of that class already is left as it is, but for an architecture job without `swarm:class-architecture` (no class statement, or the maintainer's own issue), which is given the flag alone |
| Exit codes | 0 classed, or already of the class; 1 the maintainer's verbs' refusals, `routine` by a login not in `trusted`, whose statement Daisy would not read (`Error: swarm: <login> is not in trusted, so Daisy would not read your class statement; add the login to trusted in swarm.json (docs/swarm.md section 3); nothing changed`), a forge failure; 2 usage, a word other than `routine` and `architecture` among it (`not a class: <word>; swarm class takes routine or architecture`) |
| Stdout | `Maintainer <login> on <owner>/<repo>`; `#<n> class-architecture` (architecture) or `#<n> class-architecture cleared` (routine, when it carried the flag), then `#<n> commented`; `#<n> class-architecture` alone (an architecture job given the flag); or `#<n> already <class>` |
| Ops | Real |
| Writes | on the forge, `Relabel(n, "", "swarm:class-architecture")` (architecture) or `Relabel(n, "swarm:class-architecture", "")` when it carries it (routine), then `Comment(n, "Class: <class>")`; for an architecture job without the flag, `Relabel(n, "", "swarm:class-architecture")` alone; nothing else when it is of the class already |
| Commits | none |
| Cases | none; `TestSwarmMaintainerClass`, `TestSwarmApplyMaintainerFiledClass` |

### daisy swarm approve

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build), as `swarm triage` |
| Usage | `daisy swarm approve <n> [--type <type>]`: a formal approving review of issue `<n>`'s current pull request, open, at its head, at any stage |
| Exit codes | 0 approved; 1 the maintainer's verbs' refusals, no open swarm pull request (`Error: swarm: #<n> has no open swarm pull request; nothing approved`), the forge refusing the approval, a `422` for one's own pull request among the causes (`Error: swarm: approving pull <name>#<m>: forge: POST …: 422 <message>`), another forge failure; 2 usage |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<n> approved pull <name>#<m> at <short sha>` |
| Ops | Real |
| Writes | on the forge, `Approve(m, <head>)` (no comment-type fallback) |
| Commits | none |
| Cases | none; `TestSwarmMaintainerVerbs`, `TestSwarmMaintainerRefusals` |

### daisy swarm merge

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build), as `swarm triage` |
| Usage | `daisy swarm merge <n> [--type <type>]`: G3 for an issue at `swarm:review-2-approved`: its current pull request merged at the head both the swarm's newest reviews approve, as the merge step merges (`do: merge`, `head_commit_id`), or found merged; then T13 when the accepted plan declares a slice after it, else the issue closed and labelled `swarm:done`; a merge the forge refuses of a pull request it reports not mergeable (`base` moved since the branch took it in) is T25 instead: the direction to merge `base` posted on the pull request (docs/swarm.md section 12) and the issue relabelled `swarm:review-3-changes-requested`, nothing merged; the verb reads the forge alone, so its direction names no file; a head sent back once already, the thread holding a direction on it before its last comment, is refused |
| Exit codes | 0 merged (or found merged) and relabelled or closed, or sent back to the worker (T25); 1 the maintainer's verbs' refusals, no swarm pull request, one closed unmerged, no head both reviews approve, the forge refusing the merge of a pull request it reports mergeable (`Error: swarm: #<n>: merge of pull <name>#<m> at <short sha> refused: <message>; nothing changed`: no note and no block, unlike the merge step) or whose mergeability it has not computed (`Error: swarm: #<n>: merge of pull <name>#<m> at <short sha> refused: <message>; the forge has not yet computed whether it merges, so run merge again shortly; nothing changed`), a head sent back once already that the forge still reports not mergeable (`Error: swarm: #<n>: pull <name>#<m> was sent back to merge <base> at <short sha> once already, was approved again at that head, and the forge still reports it not mergeable; merge <base> into swarm/<n>-<s>, or relabel the issue; nothing changed`), a job without its built workspace whose plan the replay would take cut from its post, its run's `out.md` gone, before any write (`Error: swarm: #<n>: the plan of <run> is cut on the forge and runs/<run>/io/out.md is gone, so its slices cannot be counted (docs/swarm.md section 7.8, Rollout); nothing changed`), another forge failure; 2 usage |
| Stdout | `Maintainer <login> on <owner>/<repo>`; `#<n> merged pull <name>#<m> (slice <s> of <S>)` or `#<n> pull <name>#<m> already merged (slice <s> of <S>)`; then `#<n> swarm:review-2-approved → swarm:plan-1-accepted` or `#<n> closed (last slice merged)` and `#<n> swarm:review-2-approved → swarm:done`; `#<n> <flag> cleared` for each flag the close cleared, or `#<n> gated cleared` after the relabel (not for an issue still blocked); sent back, `#<n> not mergeable: back to the worker`, `#<n> commented on pull <name>#<m>` (`#<n> comment already posted on pull <name>#<m>` when the direction is the thread's newest already, a re-run), `#<n> swarm:review-2-approved → swarm:review-3-changes-requested` and `#<n> gated cleared` when it carried the flag |
| Ops | Real |
| Writes | on the forge, `Merge(m, <approved head>)`; refused for a pull request not mergeable, `Comment(m, <the direction>)`, `Relabel(n, "swarm:review-2-approved", "swarm:review-3-changes-requested")` and the gated flag's removal; else `Relabel(n, "swarm:review-2-approved", "swarm:plan-1-accepted")` and the gated flag's removal when it carried it, or `CloseIssue(n)`, `Relabel(n, "swarm:review-2-approved", "swarm:done")` and the removal of each flag it carried |
| Commits | none |
| Cases | none; `TestSwarmMaintainerVerbs`, `TestSwarmMaintainerCloseUnblockMerged`, `TestSwarmMaintainerFlagAndTerminals`, `TestSwarmMaintainerRefusals`, `TestSwarmMaintainerMergeNotMergeable`, `TestSwarmMaintainerMergeBackRunAgain`, `TestSwarmMaintainerMergeSentBackTwice`, `TestSwarmFlagClearedOnEveryTransition`, `TestSwarmMergeRefusesACutPlanWithoutWorkspace` |

### daisy swarm unblock

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build; `swarm:blocked` a flag at Step 15; `--cause` at Step 5 and `--start base` at Step R1-13 of the plan implementing the 2026-11-19 swarm roadmap), as `swarm triage` |
| Usage | `daisy swarm unblock <n> [--cause <word>] [--start base] [--type <type>] <comment...>`: G4, T18, for an issue carrying `swarm:blocked` beside its stage, which the block kept: the comment, then the flag cleared; refused at the round limit's `swarm:review-1-in-progress` (or `swarm:review-0-ready`, T21's write refused), where no role runs; the words after `<n>` are one comment, saying what changed. `--cause <word>`, right after `<n>` or leading as the cli reads a value flag, retags the stop the unblock releases (docs/swarm.md 7.5, The stop's cause): the metrics row of the run whose collection set the block, in the home `<state>/home` names, is superseded first by one with that cause, its notes ending `retagged`, as `swarm metrics --reprice` supersedes a row; the word one of `plan`, `prerequisite`, `main-red`, `environment`, `forge-fact`, `kept-work`, `ruling`, `scope`, `capacity`. `--start base`, right after `<n>` in either order with `--cause` or leading as the cli reads a value flag, posts the comment with the line `Start: base` and a blank line before it, so the slice's next worker run starts where it would with no kept run: from `<base>` for a slice not yet pushed, from the open pull request's head otherwise (docs/swarm.md 7.8, The start); the kept commits stay in the job's workspace and the mirror. Also `/swarm unblock [--cause <word>] [--start base] <comment...>` in a comment |
| Exit codes | 0 commented and unblocked; 1 the maintainer's verbs' refusals, the issue not blocked (`Error: swarm: #<n> is not blocked; nothing changed`), blocked at the round limit (`Error: swarm: #<n>: blocked at the review round limit, …; nothing changed`, naming the relabels `swarm:review-3-changes-requested` and `swarm:plan-0-posted` to make first), with `--cause` a block no run's stop set (`Error: swarm: #<n>: --cause: no run's stop set this block (<why>); unblock it without --cause; nothing changed`: none flagged it, or a run of the issue started after the one that did ended, the block set since by the merge step or by hand), no usable home or no row of the run (`Error: swarm: #<n>: --cause: <reason>; nothing changed`), a forge failure; 2 usage (no comment, or blank words only, `--cause` without a word, or a word outside the nine, `--start` without `base` or with any other word, and either flag given twice, among them), before anything is read or written |
| Stdout | `Maintainer <login> on <owner>/<repo>`, with `--cause` `#<n> <run> cause <old> → <word>` (`—` for an empty cell; `#<n> <run> cause already <word>` when it is, nothing written), `#<n> commented` (or `#<n> commented on pull <name>#<m>`, at `swarm:review-3-changes-requested` with the pull request open; `#<n> comment already posted`, `… on pull <name>#<m>`, when it is the thread's newest already, a re-run), `#<n> blocked cleared`, then `#<n> gated cleared` (a blocked issue carries the flag after a pass) unless the stage the block kept is a gate that is on and holds its class, where the flag stays |
| Stderr | `⚠ <login> is not in trusted: no member reads your comment (docs/swarm.md section 6)`, never an exit code |
| Ops | Real |
| Lock | with `--cause`, `$DAISY_LOCAL/metrics.lock`, as `swarm metrics` takes it, for one read and one rewrite of the CSV |
| Writes | with `--cause`, first, one row appended to `<home>/metrics/metrics.csv` (none when the run's last row has the cause already); on the forge, `Comment(n, <comment>)` (on the current pull request instead, when it is open and the stage is `swarm:review-3-changes-requested`; none when it is that thread's newest already), then `Relabel(n, "swarm:blocked", "")` and the gated flag's removal when it carried it and the stage is not gated |
| Commits | none |
| Cases | none; `TestSwarmUnblockRetagsTheCause`, `TestSwarmMaintainerCloseUnblockMerged`, `TestSwarmUnblockReviewedRunsTheWorker`, `TestSwarmMaintainerRefusals`, `TestSwarmMaintainerBlankComment`, `TestSwarmMaintainerRunAgainPostsOnce`, `TestSwarmMaintainerKeepsTheFlagWhereGated`, `TestSwarmApplyGateNoteAfterABlockLifted`, `TestSwarmMaintainerStep29Refusals` |

### daisy swarm block

| Field | Value |
|---|---|
| Status | as built (Step 15 of the swarm build), as `swarm triage` |
| Usage | `daisy swarm block <n> --by <m> [--type <type>]`: hold issue `<n>`'s job until issue `<m>` of `forge.repo`, open, is closed: `#<m>` given `swarm:blocking`, then the maintainer's comment `Blocked by #<m>` on `#<n>`, which the role rule reads while it is `#<n>`'s newest maintainer comment (docs/swarm.md section 6, rule 1); `--by <m>` may also lead, as the cli reads a value flag |
| Exit codes | 0 flagged and commented; 1 the maintainer's verbs' refusals for `#<n>`, `#<m>` the same issue (`Error: swarm: #<n> cannot block itself; nothing changed`), closed or a pull request (`Error: swarm: #<m> is not an open issue; nothing changed`), a forge failure; 2 usage, `--by` missing or `<m>` not an issue number among it |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<m> blocking` (none when it carries the flag already), `#<n> commented` (`#<n> comment already posted` when it is `#<n>`'s newest comment already) |
| Stderr | `⚠ <login> is not in trusted: the role rule does not read your Blocked by #<m>, so it holds nothing (docs/swarm.md section 6)`, never an exit code |
| Ops | Real |
| Writes | on the forge, `Relabel(m, "", "swarm:blocking")` unless it carries it, then `Comment(n, "Blocked by #<m>")` |
| Commits | none |
| Cases | none; `TestSwarmMaintainerHolds`, `TestSwarmMaintainerRefusals`, `TestSwarmApplyHolds`, `TestSwarmMaintainerStep29Refusals` |

### daisy swarm replan

| Field | Value |
|---|---|
| Status | as built (Step 20 of the plan for the inbox requests F27–F35), as `swarm triage`; the contract is [docs/swarm.md](swarm.md) section 11, daisy swarm replan, and section 6, The re-plan |
| Usage | `daisy swarm replan <n> [--type <type>] [direction...]`: the architect re-plans the job from the maintainer's direction at `apply`'s next pass, no block or unblock needed; the words after `<n>`, when given, are the direction, posted as `Re-plan: <direction>` (`Re-plan.` with none), a comment the architect answers. Between slices (`swarm:plan-1-accepted` with a plan) the architect revises the remaining slices, merged ones kept, or confirms they stand; at `swarm:proposal-accepted` and `swarm:plan-0-posted` it plans or revises from the comment; a job with no plan goes to `swarm:proposal-accepted`, and one in review whose pull request is closed or merged to `swarm:plan-1-accepted` (T31). Also `/swarm replan [direction...]` in a comment |
| Exit codes | 0 commented, and relabelled where the stage moves; 1 the maintainer's verbs' refusals, the maintainer not in `trusted`, `swarm:proposal-posted` (triage first), `swarm:plan-2-in-progress`, a run on the issue not yet collected, an open pull request at `swarm:plan-1-accepted` or in review (`Error: swarm: #<n>: pull request <owner>/<name>#<m> (<url>) is open (<stage>); close or merge it first, then replan; nothing changed`), `swarm:question` on the issue, a direction holding a command line, a forge failure; 2 usage |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<n> commented` (`#<n> comment already posted` when it is the issue's newest already, a re-run), then `#<n> <stage> → <to>` and `#<n> gated cleared` for a relabel |
| Ops | Real; reads `<state>/runs` for runs not yet collected |
| Writes | on the forge, `Comment(n, "Re-plan: <direction>")`, then, where the stage moves, `Relabel(n, <stage>, <to>)` and the gated flag's removal when it carried it |
| Commits | none |
| Cases | none; `TestSwarmReplan`, `TestSwarmApplyCommandReplan`, `TestSwarmReplanStages`, `TestSwarmReplanRefused` |

### daisy swarm close

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build), as `swarm triage` |
| Usage | `daisy swarm close <n> [--type <type>] [comment...]`: T19, at any stage, the rejection (a job done is closed by `swarm merge`, as `swarm:done`); the words after `<n>`, when given, are one comment posted first |
| Exit codes | 0 closed and relabelled; 1 the maintainer's verbs' refusals, the terminal refused after the close, a flag's removal refused after the terminal, a forge failure; 2 usage |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<n> commented` when a comment was given (`#<n> comment already posted` when it is the issue's newest already, a re-run after a refused close), `#<n> closed`, `#<n> <stage> → swarm:rejected`, and `#<n> <flag> cleared` for each flag it carried |
| Ops | Real |
| Writes | on the forge, `Comment(n, <comment>)` when given, then `CloseIssue(n)`, `Relabel(n, <stage>, "swarm:rejected")` and the removal of each flag it carried; the pull requests are left as they are |
| Commits | none |
| Cases | none; `TestSwarmMaintainerCloseUnblockMerged`, `TestSwarmMaintainerFlagAndTerminals`, `TestSwarmMaintainerRefusals`, `TestSwarmMaintainerBlankComment`, `TestSwarmMaintainerRunAgainPostsOnce`, `TestSwarmMaintainerFlagRemovalRefused` |

### daisy swarm intervene

| Field | Value |
|---|---|
| Status | as built (Steps 6 and R0-7, `--issue`, of the plan implementing the 2026-11-19 swarm roadmap): Go (`internal/swarm/cmd/swarm_intervene.go`, `internal/swarm/reconcile/intervention.go`); the contract is [docs/swarm.md](swarm.md) section 11, The interventions' record |
| Legacy | none |
| Usage | `daisy swarm intervene --type <type> [--issue <n>] <note...>`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); the record of an intervention no verb makes (a hand edit of `swarm.json`, a hand-published patch), `<type>` one of `I1` to `I14`; `--issue <n>`, the issue the act was on, a positive issue number (not looked up on the forge); both leading value flags, in either order; the words after them are the note, their whitespace made single spaces; without `--type`, a type outside them, `--issue` not an issue number or given twice, or no note: usage on stderr, exit 2, before anything is read |
| Exit codes | 0 recorded; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, `<state>/home` naming no usable home or its CSV not this Daisy's (`Error: swarm: --type: <reason>; nothing changed`), the metrics lock held past 30 seconds or a write failing (`Error: swarm: intervention not recorded: <reason>`); 2 usage |
| Stdout | `Intervention <n>-intervention-<k> recorded: <type>`, `<n>` the `--issue` given, else `0` |
| Ops | Real |
| Lock | `$DAISY_LOCAL/metrics.lock` for the row, `<state>/journal.lock` for the journal line, each for one read and one rewrite |
| Writes | one row of kind `intervention` appended to `<home>/metrics/metrics.csv` of the home `<state>/home` names (`step_or_run` `<n>-intervention-<k>`, `role` `maintainer`, `outcome` `intervene`, `cause` the type, `notes` the note, then `, #<n>` with `--issue`), then the line `- <HHMM> <n>-intervention-<k>: <type>; intervene; <note>` (then `; #<n>` with `--issue`) queued in `<state>/journal`, which `apply`'s next pass pushes; no forge call, no secret read |
| Commits | none |
| Cases | none; `TestSwarmIntervene`, `TestSwarmInterveneIssue`, `TestSwarmInterventionRefused`, `TestSwarmVerbRecordsItsIntervention`, `TestSwarmApplyCommandRecordsItsIntervention`, `TestSwarmJournalKeepsALineQueuedDuringItsPush`, `TestIsInterventionType` |

## System

### daisy commit

| Field | Value |
|---|---|
| Status | as built (Step 24): Go (`internal/gitx`, `env.RootExclusions`); renamed from `--home`/`--not-home`, no aliases |
| Legacy | `daisy commit --home\|--not-home <message>` → `commit.sh`, deleted in Step 24 |
| Usage | `daisy commit <message>` (the home); `daisy commit --root <message>` (the root). One message word: none, a second, or one starting with `-` (so the removed `--home`/`--not-home`, alone or before a message [73], is never a message) is usage on stderr, exit 2 — the bash exited 1 and ignored a surplus word. Case 73 passes `--root` itself (decided, D30) |
| Exit codes | 0 committed [73] or nothing to commit [73]; 1 outside a workspace or when the home does not resolve (both forms, as the bash dispatcher required), when the home is not its own repository — the root's included [71] (`Error: Home '<name>' at <path> is not in its own git repository` and the same `  Run: …` fix as `healthcheck`'s), the root is not a repository (`Error: DAISY_ROOT is not a git repository`), or `git add`/`git commit` fails; 2 for a usage error [73] |
| Stdout | `📝 Committed: <message> (<short hash>)` — only that: git's own `[branch hash] message` summary, which `commit.sh` let through, is not printed (as `log` and the other committing commands) |
| Stderr | `No changes to commit` [73] ` in <scope>` (scope `home '<name>'`, `root`, or `root (excluding <paths>)` [73]), exit 0 |
| Ops | Real |
| Writes | nothing but the repository |
| Commits | home: `gitx.HomeRepo` and `Repo.Commit` (the one commit `log`, `done` and the rest make): `add -- .` and one bare commit in the home's own repository — a home inside the root's repository is refused [71] (until Step 24 its commit was path-scoped into the root's history, which a push publishes); root: the root repository minus `env.RootExclusions` (every usable registered home whose resolved directory lies inside the root), as `:(exclude,literal)` pathspecs on `status`, `add` and, while anything is excluded, `commit` (case 73: `code.txt` [73] committed, `zebranest` [73] not, the home's change left staged); a top-level `home/` is root content like any other (`home/stray/note.txt` [73] committed; the bash excluded it as the pre-split layout until Step 24); with nothing excluded a bare `git commit` that also takes pre-staged changes (`more.txt` [73]). The root commit is the publication gate's subject: `daisy healthcheck --force` before it |
| Cases | 71, 72, 73 |

### daisy healthcheck

| Field | Value |
|---|---|
| Status | as built (Step 24): Go (`internal/health`) |
| Legacy | `daisy healthcheck [--force]` → `healthcheck.sh`, deleted in Step 24 (it was also run silently first by `done`, `log`, `new-day`, `new-week`, which now call the same check in-process) |
| Usage | `daisy healthcheck [--force]`; any other word is usage, exit 2 (the bash ignored it) |
| Caching | a passing run writes the unix expiry time to `$DAISY_HOME/.healthcheck.ttl`; a run within `DAISY_HEALTHCHECK_TTL` seconds (default 300; `0` disables; anything but decimal digits is the default) exits 0 after Checks 1–2 without re-checking (no `Component:` [37] lines) and honours the stamp even over a new leak [37]; `--force` ignores it [37]; a failure removes the stamp [37] — wherever it stops the run, Check 3 included (the bash's early exits left it); a corrupt or too-far-ahead stamp is no cache [37]; `DAISY_HEALTHCHECK_PASSED=1` in the environment short-circuits unless `--force` [37] |
| Exit codes | 0 healthy [00, 34, 35, 36, 37, 74, 75] — warn-only checks never fail it [34, 74]; 1 on any error: `DAISY_ROOT` unset or not a repository, the home unresolvable, the home `not in its own git repository` [74] — the root's [71] (its tree, or its git directory through a `gitdir:` file, a `.git` link or a linked worktree: the error then ends `(its .git points into the root's)`) or a foreign one [74] (the bash accepted the root's); the fix `git -C ` [74] `<path> init && … commit` printed — preceded, for a home whose `.git` points into the root's, by `rm <home>/.git && git -C <root> worktree prune`, since `git init` there would reinitialise the root's and commit the home into it, and for a `.git` file pointing at no repository (a pruned worktree; `(its .git points at no repository)`) by `rm <home>/.git` — the paths quoted for the shell (as is when every character is safe, which is `printf %q`'s output too; else in single quotes, or `$'…'` for a non-printable byte), a missing required directory (`prompts`, `templates`, `skills`), `today.md` format faults or a `today.md` that exists and cannot be read (the bash failed its format checks there; a missing or empty one passes), a component hook failing, a publication-hygiene hit [35, 36, 37, 75] or a publishable file it cannot read; 2 for a usage error |
| Stdout | one `✓ <check>` line per pass: `DAISY_ROOT:`, `DAISY_HOME:`, `Git repository: <hash>`, `today.md format valid`, `Component: <command>` (`Component:` [37]) per hook in table order, `Publication hygiene: no identifying terms outside the homes`, `Feedback: N entries (last optimized: …\|never optimized)`, blank, `System healthy`; cached: `Checks cached — valid for Ns (--force to re-run)`. On a stream that takes colour (Colour, Common behaviour) the marks carry the bash's colours, `✓` green on stdout, `⚠` yellow and `✗` red on stderr; elsewhere the mark alone, no escape byte (`TestHealthyOutput`, `TestMarksColourOnlyOnATerminal`, `TestHealthcheckPlainOffATerminalAndUnderNoColor`) |
| Stderr | `✗ <error>` and `⚠ <warning>` lines. Hygiene (Check 6c): `Publication hygiene: 'zebrahome'` [35, 37] / `Publication hygiene: 'Zebra Quillfeather'` [75] ` appears in publishable file(s) — …` then each file indented (`leak.md` [35, 75], `public-leak.md` [36]); terms are every registered home's name (placeholders `work`, `personal`, `example`, `testhome` exempt), the git identity of each usable home's repository and of the root's, `$USER` and `$HOME/`, each once without case, less any declared in `$DAISY_CONFIG/hygiene-skip` (below); the surface is every path the root tracks (`git ls-files --cached`, wherever it lies) and every untracked, unignored one (`--others --exclude-standard`) but those under a registered home inside the root (`env.RootExclusions`) — an ignored file is never reported (`ignored-leak.md` [36] absent) and an untracked, unignored one is [36]; a regular file is read for its content, a symlink for its link text (what git publishes) and, pointing at a file, that file's content (the bash read no link text: Step 13's Deferred item); every path's name; and every file or link entry of the index (`ls-files -s`, any stage; gitlinks skipped) whose object the worktree no longer matches is read by object id (`cat-file blob`), so a staged version — assume-unchanged, skip-worktree, conflicted or not — is scanned as the next commit would publish it (the bash read neither names nor the index); a term matches without case, whole-word at each edge that is a word character, so `$HOME/` is found inside a longer path (`grep -w` required a non-word character after the `/` and missed `$HOME/src`). A failing hook: `Component: <command> failed health check` and its message indented. Warnings: `Publication hygiene: skipping declared term(s) 'notes' (<file>) — the scan does not look for them`, `Publication hygiene: unused declared term(s) 'retired' in <file> (no term the scan derives) — remove them` and `Publication hygiene: cannot read <file>, so no term is skipped: …` (6c, on every scan `healthcheck` prints, never on a cached pass, while the in-process check the commands that write the day's tasks or log (`log`, `done`, `add`, `cancel`, `priority`, `new-day`, `new-week`) run first prints nothing: the declared list `$DAISY_CONFIG/hygiene-skip`, one term per line, blank lines and lines starting with `#` aside, skips a derived term a line equals without case, never a part of one or a longer term holding it; no file prints nothing, an unreadable one skips nothing; `TestHygieneDeclaredTerms`), `<HOME>/.claude/skills/testing is a copy of the root's skill 'testing' that an earlier 'daisy install' left; skills are installed in each workspace by 'daisy init' now — run 'daisy install --update' to remove it` [34] (Check 4c: what `install` would remove, `skill.RootCopies`; never a skill of the user's own [34], an edited copy or `synced`, where Claude Code's own skill sync writes [31], nor anything when `~/.claude/skills` leads into the root, `TestSkillDriftNamesRootCopies`), `Installed skill '<name>' has no matching source in skills/` (4b, in `.claude/skills` and `.agents/skills` alike, naming the tree; never for `.agents/skills/daisy`, which init generates, `TestSkillDriftCoversBothTrees`), `journal.md has N day-blocks (expected ≤ 5)` and `<window file> is missing TOC/Summary front matter` (5b), AGENTS.md's Command Reference missing or differing from `daisy help --markdown` (6b, the drift check; the bash checked a script table), `Feedback has N entries — consider running 'daisy optimize'` (7, at 20 or more), `Override of prompts/<name>.md is behind the root (root changed since the copy)` and how to fold it in (8, per `feedback/.overrides` line whose full override's root file hash differs; silent when unchanged, never for a layered override) [88], `<ws>/.daisy/AGENTS.md is not what a build makes now (the root's prompts or the home's overrides changed since) — run 'daisy init'` or `<ws>/.daisy/prompts is not the merge … — run 'daisy init'` (8b: wherever an override is involved — in the home now, or in the workspace's build — the workspace's build compared with one made now, the `Built:` line aside — the root moved on, an override added, changed in form or removed elsewhere), `<ws>/.claude/skills/<name> is not the home's override of <root skill dir> as it is now — run 'daisy init <identity>'`, and the same for `<ws>/.agents/skills/<name>` (each tree's copy compared, `TestHealthCheckComparesBothSkillCopies`; every file init would install compared: the root's directory, the home's files, the SKILL.md as merged), `The workspace's AGENTS.md cannot be rebuilt: …` (a malformed layer, said the moment it appears; `TestHealthcheckSaysAStaleMaterialisedPrompt`, `TestHealthcheckSaysAMalformedLayer`); `The workspace was initialized by Daisy <stamp>, not this one (<commit>): run 'daisy init' here to refresh it`, or the no-stamp form, as `status` words them (8c, a warning: `.daisy/version` against the commit the binary carries, none when it carries none; `TestHealthcheckNoticesAStaleStamp`); `✗ N issue(s) found` last, each issue counted once (the bash counted a failing component and a missing directory twice). The pre-split warning (3b) is gone: the bound home inside the root repository is now Check 3's error [71], and another registered home placed under the root is no warning (`zebranest` [74] absent) |
| Ops | Real (the stamp) |
| Writes | `$DAISY_HOME/.healthcheck.ttl` only |
| Commits | none |
| Cases | 00, 34, 35, 36, 37, 74, 75 |

### daisy check-secrets

| Field | Value |
|---|---|
| Status | as built (Step 24): Go; `.env.sh` is parsed (`env.ParseShellEnv`), never run (decided, D30); the swarm's variables at Step 2 of revision 6's plan (decision [105](swarm/decisions/0105-finding-the-swarm-and-its-secrets-as-built.md)) |
| Legacy | `daisy check-secrets` → `check-secrets.sh`, deleted in Step 24 |
| Usage | `daisy check-secrets` (no argument; any is usage, exit 2): in a workspace, a swarm (docs/swarm.md section 11, Finding the swarm: a swarm repository's clone; the old layout's `<ws>/swarm/swarm.json` is no swarm since Step 7 of revision 6's plan), or both |
| Exit codes | 1 outside both a workspace and a swarm (the workspace's refusal, `Not in a Daisy workspace`), when in a workspace `<ws>/.env.sh` is not a file (`❌ .env.sh not found in workspace` and the copy hint on stdout) or cannot be read (`Error: cannot read …` on stderr), or when the swarm's `<state>/env` is readable by a group or others (a `❌` row); otherwise 0 |
| Stdout | `Checking Daisy environment configuration...`; in a workspace `Core paths:` (`DAISY_ROOT`, `DAISY_HOME`), `API tokens:` (Webex, Backstage, JIRA, GitHub); in a swarm, after a blank line when the workspace's report came first, `Swarm <owner>/<repo> (<secrets file>):`, the secrets file `<state>/env`, then a row per variable the swarm reads: `forge.token_env`'s, each `keys` `env`'s in provider order, `DAISY_MAINTAINER_TOKEN` (or, `<state>/env` refused, `❌ <state>/env is readable by others (mode <mode>); run chmod 600 on it` in their place); each row `✅ <name>: <first 8 characters>...` or `⚠️  <name>: Not configured`; then `Note: Only showing first 8 characters of tokens for security` (not after the workspace's `❌` with no swarm). A value is the file's assignment, else the environment's (as after `source`), as the swarm's verbs resolve it (docs/swarm.md section 3, Secret values); a commented line, `# <VAR>=` as `init` writes a variable it found unset, is no assignment, so that row is the environment's value or `Not configured` |
| Stderr | `⚠ swarm.json: <problem>; the swarm's variables are not checked` for a swarm whose `swarm.json` cannot be read for its forge and keys (not a failure); `⚠️  <state>/env line N: …` for the swarm's file, as for `.env.sh`; `⚠️  .env.sh line N: <why>; it and every line after it are not read` for the first line that is not a plain assignment — a command, a substitution (`$(…)`, a backtick), `${…}` with an operator, a special or positional parameter (`$1`, `$$`, `$?`, `$[…]`), `$'…'`/`$"…"` quoting, an unquoted `~` leading or after `:`, brace expansion in an `export` (`export X={a,b}`), shell syntax, a quote or a line continuation left open, a second word. Reading stops there: what the shell would do with the lines after it (an open quote, a heredoc, an `if`) depends on running it, so nothing after it is assigned. A value shown is that of the lines read, else the environment's; when reading stopped, a line after the stop may have changed it, and the warning says so. The bash ran such a line |
| Reads | `[export ]NAME=<word>` lines up to the first problem: `'…'` literal; `"…"` and bare words with `\` escapes and `$NAME`/`${NAME}` expanded from the assignments above, else the environment (a `$` before a quote inside `"…"` is literal); a trailing `# comment`; blank lines, comments and a bare `export NAME` (a comment after it allowed) change nothing; trailing blanks after a value are no part of it; CRLF read as LF |
| Ops | ReadOnly (decided, D30) |
| Writes | none |
| Commits | none |
| Cases | none (`TestCheckSecrets`, `TestParseShellEnv`, `TestSwarmVerbsWithoutAWorkspace`) |

### daisy test

| Field | Value |
|---|---|
| Status | as built (Step 24): Go; runs `go test ./...` then the acceptance suite |
| Legacy | `daisy test [name-substring]` → `test.sh` (built, then `exec`ed `tests/run.sh`), deleted in Step 24 |
| Usage | `daisy test [filter]`: a case runs when its file name contains the filter; a filter also skips `go test` (it asks for those cases) |
| Runs | from `$DAISY_ROOT`, wherever it is invoked (no workspace needed, D30): `make -C <root> --silent build`, then `go test ./...`, then `tests/run.sh [filter]` with `<root>/bin` first on `PATH`, each streaming to the terminal; the first that fails ends the run. `make test` is `daisy test` |
| Exit codes | 0 when every stage passes; 1 when the build fails (`Error: could not build bin/daisy in <root> (see above)`), `go test` fails (`Error: go test failed in <root> (see above); the acceptance suite was not run`) or a case fails; 2 for more than one argument |
| Stdout | `go test`'s lines; then `PASS  <case>` / `FAIL  <case>  (N assertion(s))` per case, failed assertions indented under it, blank, `N passed, N failed`, `Failed: <names>` when any |
| Ops | Real (it starts the build, `go test` and the suite, which builds fixtures) |
| Writes | `bin/daisy`; temporary fixtures only |
| Commits | none |
| Cases | none (`TestTestRunsBuildGoTestThenSuite`) |

### daisy version

| Field | Value |
|---|---|
| Status | as built (Step 1 of the plan remediating the 2026-11-15 design review): Go (`internal/commands/version.go`) |
| Legacy | none |
| Usage | `daisy version`, or `daisy --version`, the same command; an argument is usage, exit 2. The commit is the stamp the Go toolchain writes into a binary built inside a git checkout (Go's `debug.ReadBuildInfo`, through `Ops.BuildInfo`: `vcs.revision`, `vcs.modified`), so `make build`, or any `go build` in the root, carries it with no flag of its own; `make install-dev` compares it with the root's HEAD (`daisy install`, above), `init` stamps a workspace with it (`.daisy/version`), and `status` and `healthcheck` compare that stamp with it (`env.Commit`, `env.StampNotice`) |
| Exit codes | 0; 1 when the binary carries no commit (built outside a git checkout, with `-buildvcs=false`, or by `go test` or `go run`); 2 for a usage error |
| Stdout | the commit's full sha, then ` (modified)` when git status showed changes at build time, an untracked file included, so the binary may hold code the commit does not |
| Stderr | `Error: this binary carries no commit: …` and `  Build it with 'make build' in the root` |
| Ops | ReadOnly |
| Writes | nothing |
| Commits | none |
| Cases | none; `TestCommit`, `TestVersionWithoutAStamp`, `TestInstallDevChecksTheInstalledCommit` |
