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
`swarm status`, `swarm pause`, `swarm resume`, and the maintainer's
`swarm triage`, `swarm accept`, `swarm class`, `swarm approve`, `swarm
merge`, `swarm unblock`, `swarm block` and `swarm close`) joined the
table a verb at a time as each was built; all fourteen are in it, and the tree never names a command the binary does
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
  status                            Home, identity, projects, today's counts, active plan
  files                             Resolved real paths for every home file
  list                              Active prompts and installed skills
  help [<command>|<skill>|--markdown]
                                    This help, detail for one command or skill, or the Command Reference

Day
  new-day [--quick]                 Start a new day (--quick: only if not started; a week if one turned)
  new-week [--quick]                Start a new week (--quick: as new-day --quick)
  log <message...>                  Add a log entry to today.md, quick-starting a stale day
  done <pattern>                    Mark a task complete
  tasks [--all] [--todo|--done] [pattern]
                                    Search tasks (read-only)
  projects [--archived]             List active or archived projects
  rotate                            Rotate journal.md into window files

Plans
  plan new [--spec] <description> [+project]
                                    Create a plan (or an unregistered spec draft)
  plan pickup <path>                Promote a spec draft into a tracked plan
  plan archive                      Archive the active plan
  metrics                           Summary of the home's metrics CSV, by kind and model (read-only)
  metrics add --step <s> [--plan <p>] [--range <r> [--repo <dir>]] [<flag> <value>...]
                                    Record a plan step: a row in the home's metrics CSV and in the plan's ## Metrics table

Homes and identities
  home new <name> [path]            Create a home; prompts for identity and location
  home add <name> <path>            Register an existing home
  home list                         Registered homes and where they live
  home remove <name>                Unregister a home; its files stay where they are
  identity list                     Identities across registered homes

Prompts and skills
  prompt build [<home>]             Rebuild AGENTS.md
  install [--update]                ~/bin/daisy and DAISY_ROOT; nothing under ~/.claude
  feedback [--workflow <n>] <text>  Record a prompt failure for optimization
  optimize [--workflow <n>]         Briefing: pending entries per target, current rules, rewrite flags
  optimize apply --workflow <n> (--rules|--rewrite) <file> [--yes]
                                    Apply a proposal: diff, approve, archive, commit
  eval [<case>] [--record]          List, display, or record an eval case result

Swarm
  swarm init [--defaults] [--api <url>]
                                    Declare the swarm: swarm.json, prompts, labels
  swarm prompt build [--force] [--dry-run]
                                    Reseed prompts/ from this Daisy's defaults; an edited prompt is kept
  swarm apply [--once]              Reconcile the forge: launch members, post their output, move labels
  swarm status                      Stages, gates, what waits for you, running members (read-only)
  swarm pause [<n>]                 Launch no new member until resumed (with <n>, as the maintainer, on that issue)
  swarm resume [<n>]                Launch members again and release the owner's held batch (with <n>, that issue)
  swarm triage <n>                  As the maintainer: triage a proposal, proposal-posted to proposal-accepted
  swarm accept <n>                  As the maintainer: accept the plan, plan-0-posted to plan-1-accepted
  swarm class <n> routine|architecture
                                    As the maintainer: set the job's class; Daisy passes a routine job's triage and plan
  swarm approve <n>                 As the maintainer: approve the issue's open pull request
  swarm merge <n>                   As the maintainer: merge the approved pull request, then next slice or close
  swarm unblock <n> <comment...>    As the maintainer: comment, then clear swarm:blocked from the issue
  swarm block <n> --by <m>          As the maintainer: hold the issue's job until #m is closed
  swarm close <n> [comment...]      As the maintainer: close the issue, after the comment when given

System
  commit [--root] <message>         Commit the home repo (or the root repo)
  healthcheck [--force]             Run the system health check
  check-secrets                     Check which secrets/tokens are configured
  test [filter]                     Run go test, then the hermetic acceptance suite

Every command also accepts --help/-h (as the sole argument) to print its usage.

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
`swarm close` and `swarm unblock` refuse one in `<n>`'s place), with
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
daisy <usage>`, blank, the description). `daisy`, `daisy help`, `daisy --help`,
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
only the root *(decided, D30; built in Step 24)*, and the fourteen
`swarm` verbs, which find their swarm themselves (docs/swarm.md section
11, Finding the swarm: a swarm repository's clone; Step 2 of revision
6's plan, the old layout's workspace no longer read since its Step 7). `healthcheck` needs one
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

**Binding drift.** `status` and `files` print one warning line per fault on
stderr, each ending with the fix `daisy init` [76] `<identity>`: a missing
or wrong link (naming the file, `today.md` [76]), a git key whose value is
not the recorded identity's (naming the key, `user.email` [76]; `differs`
[77]), an identity file that `does not match what init recorded` [77], a
bound identity file that is not a complete identity (naming the file), an
applied identity nothing claims (naming the author, `Pelican Moorhen` [79]).
A clean binding prints nothing on stderr (asserted empty in 76, 77, 79).

**Environment read.** `DAISY_ROOT` (required: without it a command exits 1
with `Error: DAISY_ROOT not set` and the `~/.zshenv` line to add;
`install` alone finds its root without it, from the binary's own path,
Step 21, and refuses one naming another root, Step 35), `DAISY_CONFIG`, `DAISY_LOCAL`, `HOME`, `USER`,
`DAISY_HEALTHCHECK_TTL`, `DAISY_HEALTHCHECK_PASSED` (see `healthcheck`),
`SHELL` (`install` picks the rc file by it), `PATH` (`test` puts the
root's `bin/` first on it for the suite), the variables `check-secrets`
reports, and the swarm's: the variable `forge.token_env` names (`swarm
init`, `apply`, `status`; the maintainer's verbs only to compare it), each
used `keys` `env` (`apply`) and `DAISY_MAINTAINER_TOKEN` (the maintainer's
verbs, and `swarm init` for its writes on the swarm repository), each when
the swarm's secrets file, `<state>/env` (docs/swarm.md section 3), does not assign it. Nothing else: `optimize` holds no API key and reads no
model or editor setting (Step 23, D12).

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

**`Ops` wrappers.** `ReadOnly` for `status`, `files`, `list`, `projects`,
`tasks`, `help` and the briefing, `optimize` (Step 23),
and for `check-secrets`, `home list` and `identity list` (decided, D30),
and `swarm status`, for which `ReadOnly` lets a `GET` or `HEAD` request
and `docker ps` through (docs/swarm.md section 14, item 31), and from
revision 6 git's reading commands on a mirror (item 99);
`Real` for everything else, `eval` included (one wrapper per command;
`--record` writes), `optimize apply`, and `healthcheck`, because a pass
writes the TTL stamp.

**Commits.** A command that commits does so exactly once, in the home's
repository, as that repository's own git config — never in the root (case
72: `done`, `log` and `new-day` add one home commit each and the root
repository's commit count and `git status --porcelain` are unchanged).
As built, the committing commands are `done`, `log`, `new-day`, `new-week`,
`plan pickup`, `plan archive`, `optimize apply` *(Step 23)*, `commit`
and `home new` (the new home's first commit); every mutation of an
existing home the binary makes goes through
`gitx.Repo.Commit`, one commit per command *(built, Step 18)*, and `home
new` makes the new repository's first commit with its own `git init`,
`add -A`, `commit` *(Step 21)*; `plan new`, `feedback`, `eval --record`,
`metrics add` and `rotate` run by hand write home files without committing them, and the next home commit sweeps
them up *(decided, D37 for `plan new`, D38 for the other two; `metrics add`
as `feedback`, Step 16a of the plan that clears the Deferred list)*. The root
is committed only by `commit --root`, by the developer's hand: `optimize
apply` never writes a root target, it writes the home's override of it
(D49).

## Workspace

### daisy init

| Field | Value |
|---|---|
| Status | as built (Step 21): Go (`internal/workspace`, `internal/identity`); `init --new` removed (`home new`, then `init`); the bare form as built (Step 12 of the plan that clears the Deferred list, 2026-11-12); the root's skills and its Read and Edit rules installed in the workspace, where `install` put them in `~/.claude` before (Step 13, 2026-11-12) |
| Legacy | `daisy init [--new] <identity> [path]` → the bash `daisy-init` script, deleted in Step 21 |
| Usage | `daisy init [<identity> [path]]` |
| Arguments | `<identity>`: a registered home's name (its default identity, `gitconfig`) or the name of `<home>/identities/<identity>` in exactly one registered home. `[path]`: the workspace directory, default `.`; it comes only after an identity (a path alone is read as the identity). No argument: the workspace the working directory is in (the nearest ancestor holding a binding, as Common behaviour's Workspace finds it) is re-initialized with the home its `.daisy/home` names and the identity its `.daisy/identity` names, else the home's default: what `daisy init <identity>` run there does, but with the binding's home, not the one the identity's name resolves to now (`TestBareInitReinitsTheBinding`); with no binding above the working directory, or one whose `.daisy/` names no home, the identity is the missing argument: `missing argument` [91], a line naming `daisy init <identity>` [91] (and `daisy identity list`), the usage, exit 2 [91], nothing written. A third argument: usage on stderr, exit 2. After the root is updated, the bare form refreshes everything init writes (`.daisy/AGENTS.md`, the skills, the agent files, the allow rules); `prompt build` rebuilds only `AGENTS.md` and the materialised prompts |
| Exit codes | 0 bound (also when already bound, or re-bound to another identity); 1 refused, nothing written: for the bare form, a broken binding above the working directory (Common behaviour's Workspace messages) and a `.daisy/identity` that is not an identity name (names the file); identity in two homes (`identity 'pelicanmoor' is defined in more than one home: testhome, second` [78], `more than one home` [78]), unknown identity (names it, `nosuchone` [77], and lists the identities that exist, `pelicanmoor` [77]), `not a valid identity name` [77], a registered home that cannot be checked (names it and `does not exist` [78]), a named identity file that is incomplete (names the file `identities/pelicanmoor` [79] and the missing key `email` [79]), a malformed `sshkey` [79] (the default identity's too: it may be incomplete, never malformed), a `linked worktree` [76] (names the main checkout), a registered home's own repository (`is home '<name>''s own repository`: its commits carry the home's identity, `TestInitRefusesAHomesOwnRepository`), the directory holding the `Daisy root` [78], `.daisy` `not a directory` [77] or a `symlink` [77], two leftovers and no `.daisy/` (names both, `.daisy.previous.100` [77], `.daisy.previous.200` [77]), a settings file that is not a JSON object [77, 78], a directory where `.daisy/` holds a file of init's own [77, 78]; every refusal is decided before the first write — an unknown identity, an unreadable settings file, a file where init needs a directory or a directory where it writes a file, a skill (the root's or the home's) named like a core rule (`daisy`, `daisy-logging`, `daisy-plan`), a file of a skill (or of the root skill a layered one merges over) that cannot be read — so a refused init writes nothing, not even `git init`; 1 when a step of the final section fails: `did not finish` [77, 78] on stderr and everything rolled back (binding, git keys byte for byte, stage removed) |
| Stdout | `Initializing Daisy in: <path>`, `Home: testhome` [77], `Identity: testhome` [76] / `Identity: pelicanmoor` [77] (the build's own report is not printed; its warnings go to stderr); on a re-bind `Already initialized` [77] `with identity: <identity>` (`Already initialized with identity: pelicanmoor` [77, 91]; the bare form's first line, `Initializing Daisy in: ` [91], names the workspace, not the working directory) or the switch `pelicanmoor → testhome` [77]; the ✓ lines for each artifact; `Bound .daisy/` [79] `to home: <home>`; a cleared previous identity names its keys (`core.sshCommand` [79]); `Done. Daisy is ready in this workspace.` |
| Stderr | warnings, exit still 0: a root skill the workspace has its own of `is not installed` (Writes), `no complete default identity` [76] (the home's `gitconfig` is template or incomplete; no identity applied), an sshkey that is `not found` [79] (names it, `no-such-key` [79]), a reclaimed leftover `.daisy.previous.12345` [77], `No readable .daisy/gitconfig record` [79] of the previous identity, an aside that `could not be removed` [79]; errors as above |
| Ops | Real |
| Writes | `<ws>/.daisy/` as a whole (staged beside it, swapped in by rename): `home`, `identity` (non-default only), `gitconfig` (the applied triple), `gitconfig.before` (what the three keys held, once the previous identity was taken back, when any held a value), the six links `tasks today.md journal.md projects plans feedback.md` into the home, `AGENTS.md`; `<ws>/.cursor/rules/daisy.mdc daisy-logging.mdc daisy-plan.mdc` (obsolete `daisy.md`, `daisy-logging.md` removed, case 32), `<ws>/.claude/commands/daisy.md`, `<ws>/.agents/skills/daisy/SKILL.md` (Codex's `daisy` skill, from `templates/agents-skill.md`, replaced whole; no root `AGENTS.md` block and no `.codex/` write, D54), `.daisy/prompts/<name>.md` for each layered prompt (as `prompt build`); `<ws>/.claude/skills/<name>/`, its twin `<ws>/.agents/skills/<name>/` (Codex's; both trees written, swept, guarded and removed by `clean` through one list, `skill.WorkspaceTrees`, `TestInitWritesCodexTwinTree`, case 90) and `.cursor/rules/<name>.mdc` for each skill of the root and of the home (by its directory's name, at any depth under `skills/`; a home skill wins a root skill's name: an override of a root skill installs as the root's skill directory, the home's other files over it, its `SKILL.md` the home's — merged when layered, `TestSkillOverrides`; a home skill of a root skill's name at another path installs alone; each root skill's copy is the root's directory byte for byte, `TestInitInstallsRootSkills`, case 90; a root skill whose name the workspace already uses for its own — `.claude/skills/<name>`, `.agents/skills/<name>` or `.cursor/rules/<name>.mdc` there, with no stub of init's claiming it — is not installed: stderr `The root's skill <name> is not installed: this workspace has its own <path>; rename or remove it, then re-run 'daisy init' to install the root's`, exit still 0, and none of its copies, stub or ignore lines is written, so `clean` leaves the workspace's own as it was (`TestInitKeepsAWorkspacesOwnSkillOfARootName`; a home skill's name is the home's choice and is installed over the workspace's, as before); a layer with no root skill refuses init; as a best-effort check, a path init writes that resolves into the root outside the workspace through a user-made link, such as `.claude/skills` or `.agents/skills` linked to the root's `skills/`, refuses init before any write, `TestInitRefusesASkillsLinkIntoTheRoot`) (a stub in exactly the form init writes — the one record of which skills init installed — whose skill neither the root nor the home has now is swept with its copy, case 33; any other rule is the user's; a link standing where a Cursor rule or a stub goes is replaced by init's file, never written through), `<ws>/.claude/settings.local.json` (Daisy's allow rules: `Read(.daisy/**)`, `Edit(.daisy/**)`, `Read(/$DAISY_ROOT/**)` and `Edit(/$DAISY_ROOT/**)` (the `//`-anchored form, as `install` once wrote them in `~/.claude/settings.json`), `Bash(daisy:*)` [90]; the legacy `Bash(…/daisy/scripts/*)` removed), the entries missing from `<ws>/.gitignore` (`.cursorignore` among them, from Step 21) and `.cursorignore`, into the block under Daisy's header line (applying twice equals once); an entry a `.gitignore` of the repository already ignores is not added (`git check-ignore`, which names the rule that decides; an entry only this machine's `core.excludesFile` or `.git/info/exclude` ignores still is, a clone elsewhere lacking it, `TestInitSkipsOnlyWhatTheRepositoryIgnores`, and so is one under or at a directory that is a symbolic link (`.claude` linked elsewhere, a skill's copy or its Codex twin linked), which is outside the repository's tree, `TestInitIgnoresThroughALinkedAgentDirectory`), so the Daisy root, whose `.gitignore` ignores every path init and `plan new` write there wholesale, gets no block, and `init`, `plan new` and `clean` leave it as git has it (`TestBindingTheRootLeavesItAsGitHasIt`); `git init` when the directory is not a repository top level; local git config `user.name`, `user.email`, `core.sshCommand` (`ssh -i <key> -o IdentitiesOnly=yes`, only with `sshkey=`) — the previous identity's keys unset first where their values are still its, and what they held before that identity was applied put back |
| Commits | none |
| Cases | 32, 33, 76, 77, 78, 79, 90, 91; `TestBareInitReinitsTheBinding`; `TestInitInstallsRootSkills`; `TestInitIgnoresWhatItWrites` (the `.gitignore` block ignores every path init and `plan new` write into the workspace, every entry is one of them or a stage or previous name with `*` for exactly its generated suffix, and none ignores a user's own file; each `.cursorignore` line re-includes an entry of the block, and `.daisy/`, `PLAN.md` and each core Cursor rule the block holds has one); `TestInitSkipsOnlyWhatTheRepositoryIgnores`; `TestInitIgnoresThroughALinkedAgentDirectory`; `TestBindingTheRootLeavesItAsGitHasIt` |

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
| Stdout | `Cleared local git identity (user.name, user.email)` [76, 77, 79] naming exactly the keys cleared — `Cleared local git identity (user.name)` [76], `Cleared local git identity (user.email)` [79], `Cleared local git identity (user.name, user.email, core.sshCommand)` [79]; `left as is` [76] when nothing matched; `No readable .daisy/gitconfig record` [76] when there is no record and no identity file resolves; `Removed .daisy/`, `Removed <leftover>` (`.daisy.previous.555` [76]), `Removed Cursor rules`, `Removed Claude command`, `Removed N skill(s)…`, `Cleaned .gitignore`, `Removed empty .gitignore`, `Removed Daisy permissions from .claude/settings.local.json` or `Removed .claude/settings.local.json` (when only Daisy's rules were in it), `Cleaned .cursorignore`, `Removed empty .cursorignore`; `· Local git identity not checked: this workspace is not a repository` / `…this is a linked worktree and its config is the main checkout's` when the keys cannot be checked; `Done. Daisy has been removed from this workspace.` |
| Stderr | `could not unset` [79]; otherwise unspecified |
| Ops | Real |
| Writes | removes everything `init` wrote (above) but the repository a `git init` made, and only that: the `.gitignore`/`.cursorignore` block under Daisy's header (a user's own line that is also an entry stays), Daisy's allow rules (those of the root as `$DAISY_ROOT` names it now, and the legacy forms; the file goes only when nothing else is left in it), the skills init installed, the root's among them (those whose `.cursor/rules/<name>.mdc` is still a regular file in init's stub form, whatever the home holds now), the directories init writes into when they are left empty (below); unsets each of the three git keys when every value it holds is the recorded identity's (the home's identity file stands in for the record for a workspace bound before records existed) or is a `core.sshCommand` in Daisy's exact form (`ssh -i <path> -o IdentitiesOnly=yes`, which nothing but an init writes — taken with or without a record); a hand-set value is the user's and stays; a key it unsets gets back what it held before init (`.daisy/gitconfig.before`), so init then clean leaves the keys and `git status` as they were [76], but for what init keeps no record of: an ignore file with no final newline keeps the one init gave it; an ignore file that was empty before init, or a settings file that held nothing (`{}`) before it, is removed; a settings file it takes rules out of is re-indented; the directories init writes into (`.cursor`, `.cursor/rules`, `.claude`, `.claude/commands`, `.claude/skills`, `.agents`, `.agents/skills`, `.agents/skills/daisy`) go when empty, whether init made them or they were there before; reclaims `.daisy.stage.*` leftovers |
| Commits | none |
| Cases | none of its own; exercised by cases 76/77/79 (`init`'s) and 78; the inverse is `TestInitThenCleanRestores` |

### daisy status

| Field | Value |
|---|---|
| Status | as built (Step 17): Go |
| Legacy | `daisy status` → built in to `daisy.sh` (`cmd_status`), deleted in Step 24 |
| Usage | `daisy status` |
| Exit codes | 0; 1 outside a workspace or when the home does not resolve (`cannot be a home` [68], `not registered` [68]); 2 with any argument (the bash ignored them) |
| Stdout | line 1 `Daisy \| home: <home> \| identity: <identity> \| workspace: <path>` (asserted below; `home: second` [68], `workspace: ` [78]); `Projects: <+tags> (from .daisy/project)` / `(from directory name)` or `Projects: (none)` (`+chessgrid` [45]); blank; `Tasks:  N pending, N completed today`, `Log:    N entries`, `Backlog: N tasks in todo.txt`; `Plan:   <title>` and its path when `PLAN.md` is a symlink; or `  No today.md — run 'daisy new-day' to start.` |
| Stderr | drift warnings (Common behaviour); empty on a clean binding |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 45 |

The first line is asserted with the pipes: `home: testhome | identity: testhome` [76]
and `home: testhome | identity: pelicanmoor` [77].

### daisy files

| Field | Value |
|---|---|
| Status | as built (Step 17): Go |
| Legacy | `daisy files` → `files.sh`, deleted in Step 24 with its `--healthcheck` hook (it checked only that the home resolves; `files` has none) |
| Usage | `daisy files` |
| Exit codes | 0 (a project that does not exist is not fatal, case 44); 1 outside a workspace; 2 with any argument |
| Stdout | the same first line as `status` (`identity: testhome` [76], `identity: pelicanmoor` [77]); sections `Identity:` (the resolved identity file: the home's `gitconfig` [76] or `identities/pelicanmoor` [77], ` (missing)` when absent), `Journal:`, `Tasks:`, `Projects:` (`workspace` row: `+chessgrid` [43] `(from ` `directory name` [43] `)`, `+chessgrid +jobsearch` [43] in file order, `+jobsearch` [42] `(from ` `.daisy/project` [42] `)`, or `(none)` (`none` [44])), `Feedback:`, `Plans:` (with `current plan` when `PLAN.md` is a link), `Metrics:` (`metrics.csv`, which `metrics add` writes, listed whether or not it exists yet), `Prompts:`; each row `  %-12s <real path>`; last `AGENTS.md:    <real path>` (`%-14s`) |
| Stderr | empty when the convention default matches nothing (asserted `""`, case 44); a declared project with no file is named (`typoed-name` [44]); drift warnings |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 42, 43, 44 |

### daisy list

| Field | Value |
|---|---|
| Status | as built (Step 20): Go |
| Legacy | `daisy list` → the bash `list` script, deleted in Step 20 |
| Usage | `daisy list [--full]` (`--full` prints each skill's `description` instead of `short_description`; not in the help line, kept as an unasserted flag) |
| Exit codes | 0; 1 outside a workspace, for an `include.txt` that cannot be read or a line of it with an unmatched quote (where the bash died mid-listing), or a prompt or SKILL.md that cannot be read; 2 with any other argument |
| Stdout | `Prompts (<home>):` then one `  %-20s <trigger summary or (always loaded)>` per `include.txt` entry (the width in bytes, as the bash's `printf`), `(missing: <path>)` for an absent prompt, `(no include.txt found)`; the summary is the trigger's first line that is not `Read the full…` and not empty once a leading `- ` is dropped, cut to 97 characters and `...` past 100, else `(lazy, no trigger summary)`; a home override then ` [layered]`, ` [full]` or ` [full, behind]` (the root changed since the copy, `feedback/.overrides`) [88], skills too; blank; `Skills:` then one `  %-20s <text>` per installed skill, user-level (`~/.claude/skills`) then workspace (`.claude/skills`), workspace overriding by name, in byte order of name (the bash's `sort` collated by locale); `<text>` is `short_description`, else `description`, one surrounding `"` dropped; or `(none installed — run: daisy init <home>)` |
| Stderr | the refusals above |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 85 |

### daisy help

| Field | Value |
|---|---|
| Status | as built (Step 20): Go. The tree (`daisy`, `daisy help`, `--help`, `-h`) is rendered from the `cli` table; so, since Step 24, is AGENTS.md's Command Reference (`--markdown`) |
| Legacy | `daisy help [<command>\|<skill>]` → built in to `daisy.sh` (`show_help`, `show_command_help` and the skill lookup), deleted in Step 24 |
| Usage | `daisy help [<command>\|<skill>\|--markdown]`; `daisy`, `daisy --help`, `daisy -h` are `daisy help` |
| Exit codes | 0; 1 for a name that is neither (`Unknown command or skill: <name>` and `Run 'daisy help' for commands, 'daisy list' for skills.` on stderr; the bash died silently there) or outside a workspace for a skill; 2 for surplus words |
| Stdout | no argument: the tree. A command name (its words: `help prompt build`): exactly what `daisy <command> --help` prints — `Usage: daisy <usage>`, blank, the description, from the table — then, when a skill of that name exists (installed or source; none looked up outside a workspace), a note — the command's help, not the skill's (not `Collide politely` [52]), and the word `skill` [52]: blank, `Note: a skill named '<name>' also exists; the command name takes precedence.`, `  Read it with: daisy list, or open its SKILL.md directly.` A noun (`help prompt`): the usage of each of its commands. A skill name: `<name>  (<SKILL.md path>)`, blank, the full `description:` value (`when to stop making them` [51]; the line after `description: `, `: ` inside it kept), blank, the `## Rules` section up to the next `#` or `##` heading, `optimize`'s boundary (`Measure before cutting` [51]; until Step 23 an `# ` heading and what followed it to the next `## ` were printed as rules), `Other sections: <H2 titles>` joined by `, ` (`Worked example` [51]). The installed copy (workspace over user) is preferred over the source (home over root); a home source that is a layered override is shown merged (`TestSkillOverrides`). `--markdown` as the only word: the Command Reference block for AGENTS.md — the BEGIN marker line, a `\| Group \| Command \| What it does \|` table with one row per table command in table order (each usage in backticks after `daisy `, `\|` escaped), the END marker line — which `healthcheck` compares with the root's AGENTS.md (6b) |
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
| Usage | `daisy new-day [--quick]` (no argument; any is usage on stderr, exit 2 — the bash ignored them) |
| Quick start | `--quick` starts only a day not yet started: when `today.md`'s first `### YYYY-MM-DD` heading is today or later it prints `✅ Day already started: <YYYY-MM-DD> <Day>`, writes nothing and commits nothing (exit 0). Otherwise it starts a new week when that date is in an earlier ISO week than today, else a new day, whichever of `new-day` and `new-week` was run; a `today.md` that is absent or has no such heading starts the command's own. Before the start's own lines it prints `⚡ Quick start: today.md was <YYYY-MM-DD>; retrospective not checked` (a day) or `…; retrospective not checked, daisy optimize not run` (a week), without `today.md was <date>; ` when there was no date: what the agent's New Day and New Week workflows do around the command and this start did not (`TestQuickStart`) |
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
| Usage | `daisy new-week [--quick]` (no argument; `--quick` as `new-day`'s) |
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
| Writes | `tasks/todo.txt`: the one matched line removed and `x <today> <creation date> <description>` appended last, no priority [10] — the line read by `todo.Parse`, so an undated task completes as `x <today> <description>` with one space and its priority dropped (the bash left the priority in the description and wrote two spaces); `journal/today.md`: the first line equal to `- [ ] <the line as new-day carries it>` (an undated `(A)`/`(B)` line whole, `TestDoneWithoutTheTaskInToday`), else the first equal to `- [ ] <description>`, gets its `[ ]` made `[x]`, every other byte unchanged [01, 02, 12]; then the log entry `Done: <description>` in-process (no `log` sub-call) |
| Commits | home, exactly once for the whole completion, holding both files [13, 14], subject `Completed: <description>` *(decided, D33, Step 18: the bash's one commit was `log`'s, `Log: HHMM - Done: …`, and its own `Completed:` commit always found nothing)*; a failed journal entry still leaves the edits committed [17]; a failed commit after a successful completion is an error, exit 1 (the bash printed `(commit not committed)` and exited 0) |
| Cases | 01-18 |

### daisy tasks

| Field | Value |
|---|---|
| Status | as built (Step 18): Go |
| Legacy | `daisy tasks [--all] [--todo\|--done] [pattern]` → the bash `tasks` script, deleted in Step 18 |
| Usage | `daisy tasks [--all] [--todo\|--done] [pattern]` |
| Arguments | scope: this workspace's projects by default — a line is in scope when one of its `+tag` words (whitespace-delimited, compared without case) is a workspace project — `--all` every project [19, 46, 47]; files: both by default [22], `--todo` = `todo.txt` [20], `--done` = `done.txt` [21], the last of the two given wins; `[pattern]`: literal, case-insensitive substring [24] (an empty pattern is none), a leading `-` is a pattern (`-1 point deduction task` [25]) once the three flags above have been consumed — except the sole-argument `--help`/`-h` and `--healthcheck`, and a word shaped like a long flag, an unknown flag (Common behaviour); flags before the pattern; a second positional is a usage error, exit 2 |
| Exit codes | 0 when at least one line is printed [19–25, 27, 29, 46–48], and with no pattern whatever the scope matched; 1 when a pattern is given and matches nothing, with empty stdout [26] and `No tasks match: <pattern>` on stderr; 1 when none of the selected files exists (`Error: no task file to search: …`); a missing `done.txt` alone is not an error [27] |
| Stdout | the matching lines verbatim and nothing else — no header (`Active projects` [29]), no decoration [29], blank lines skipped; every task line when no pattern is given [23] |
| Stderr | when the workspace resolves to no project and `--all` was not given: `⚠️  This workspace resolves to no project; listing every project's tasks.` (`no project` [48]) and `  Bind it by creating .daisy/project, or pass --all to ask for this deliberately.`; the `.daisy/project` typo warnings, as `status` prints them |
| Ops | ReadOnly (`todo.txt` and `done.txt` byte-identical and no commit after a query [28]) |
| Writes | none |
| Commits | none |
| Cases | 19-30, 46, 47, 48 |

### daisy projects

| Field | Value |
|---|---|
| Status | as built (Step 18): Go |
| Legacy | `daisy projects [--archived]` → the bash `projects` script, deleted in Step 18 |
| Usage | `daisy projects [--archived]` |
| Exit codes | 0; 1 outside a workspace; 2 with a positional argument |
| Stdout | `Active projects:` (or `Archived projects:` for `projects/_archive/`) then `  %-30s <real path>` per `*.md` file, in byte order; `Active projects: none` when there are none |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | none (cases 42–44 pin project *resolution* through `files`) |

### daisy rotate

| Field | Value |
|---|---|
| Status | as built (Step 19): Go; byte-identical to the bash on a copy of a real journal over 33 `new-day`/`new-week` runs across a month and a year boundary, and on 270 synthetic journals (D34) |
| Legacy | `daisy rotate` → the bash `rotate` script, deleted in Step 19 (also run by `new-day` and `new-week`, in-process in Go) |
| Usage | `daisy rotate` (no argument) |
| Exit codes | 0, also when `journal.md` is missing, empty or holds five day-blocks or fewer (nothing written, nothing printed) [83]; 1 when `journal.md` has non-blank content before its first `### YYYY-MM-DD` heading, but for a preamble ending in `---` with a day-block after it (`TestRotateKeepsALeadingRetrospective`) (`Error: rotate: journal.md has non-blank content before its first '### YYYY-MM-DD' heading — aborting, nothing written` on stderr [83]), when the month-to-date or year-to-date window holds no day-block (`Error: rotate: <file> has no '### YYYY-MM-DD' day-block …`; the bash died part-way there, with the last-14 window already written), or when a block would move and a rolling window is not a regular file (`Error: rotate: <file> is not a regular file …`; the bash appended through a link, then replaced it with a file). Nothing is written on either refusal [83] |
| Stdout | `✅ Rotated N day-block(s); journal.md retains its last 5` when anything moved [83]; nothing otherwise [83] |
| Ops | Real |
| Writes | `journal/journal.md` keeps its last five `### YYYY-MM-DD` blocks [81, 83]; a block runs to the next such heading, less its trailing blank lines and one trailing `---`, so a preamble before a heading (new-week's Weekly Retrospective) stays with the block before it, and one before the first heading, ending in `---` (a home whose journal was empty when its first week began), stays at the top of `journal.md`, never rotated. Each older block, oldest first, is appended to all three rolling files — `journal-<YYYYMMDD>-last14.md` (which then drops its oldest block beyond 14 and takes the date of its new first block), `journal-<YYYYMMDD>-mtd.md` and `journal-<YYYYMMDD>-ytd.md`, each named by the full date of its first block [83] — and a month or year window whose first block is of another period than the incoming one closes first into `journal-YYYYMM.md` / `journal-YYYY.md` [83] (replacing a file of that name). So every rotated block is in exactly one month file and one year file, and in the last-14 window while it is among the fourteen most recently rotated (D34 restates Step 19's "exactly one window file" so) *(decided, D34, Step 19)*. Each window file carries `# <label>`, `## Table of Contents` (one date per block), `## Summary` (`N entries, <first> to <last>.` [83]), `---`, then its blocks separated by `---` |
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
| Exit codes | 0 [55]; 1 when `PLAN.md` already exists — as a dangling link too (`Error: PLAN.md already exists in this workspace`; the bash wrote the plan file, then failed at `ln`) —, or with `--spec` when `<slug>_PLAN.md` does, when the slug is empty (`Error: description produced an empty slug`), when the project holds `/` or a NUL (`cannot be part of a file name`; the bash failed at the write) — with `--spec` too, where it names no file yet but `plan pickup` would refuse it, when the plan file already exists (two plans named in the same second; the bash overwrote it), when `templates/plan.md` is missing, when `.gitignore` cannot be read, or when the filename would reach 255 bytes: `the limit is 255` [55] and `Shorten the project tag` [55] on stderr, never the shell's `File name too long` [55]. Every refusal is decided before the first write; a failed write takes back the plan file and link already made (`Nothing was created.`) |
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
| Exit codes | 0; 2 with no path or a second one; 1 for a path that is not a regular file (`Error: File not found: <path>`), an existing `PLAN.md` (a dangling link included), no `# <Description>` H1 in the file (`Could not find a '# {Description}' header`), an empty slug, a project holding `/` or a NUL, a filename of 255 bytes or more (`the limit is 255` — the bash did not bound the slug here, so a long H1 died on the filesystem), an existing plan file of that name, or a home not in its own git repository (checked before the first write; the bash copied and linked, then failed at the commit); a failed link takes back the copy; a failed commit leaves the plan picked up and says so |
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
| Usage | `daisy plan archive` (any argument: usage, exit 2; the bash ignored them) |
| Exit codes | 0 [54]; 1 when `PLAN.md` does not exist, is not a symlink (`not a Daisy plan`), its target is not in `plans/` (`The symlink may be broken…`), `plans/archive/<filename>` already exists (the bash's `mv` replaced it), a project file cannot be read, or the home is not in its own git repository — each decided before the first write; 1 when a write fails, with the move, the link and every project file put back (`Nothing was archived.`, or which undo failed); a failed commit leaves the plan archived and says so |
| Stdout | `✅ Archived: <filename>`; `  ✓ Repointed links in N project file(s)` when any (`project file` [54]); then `commit`'s line |
| Ops | Real |
| Writes | moves `plans/<filename>` to `plans/archive/<filename>`, removes `<ws>/PLAN.md`; in every regular `projects/*.md` and `projects/_archive/*.md` (not dotfiles, as the glob) rewrites each literal `../plans/<filename>` to `../plans/archive/` [54] `<filename>`, written through a link (the bash's `sed -i` replaced a linked project file with a copy), leaving unrelated project files byte-identical [54] |
| Commits | home, once: `Archive plan: <filename>` |
| Cases | 54 |

### daisy metrics

| Field | Value |
|---|---|
| Status | as built (Step 16a of the plan that clears the Deferred list, 2026-11-12): Go (`internal/metrics`, `internal/commands/metrics.go`) |
| Legacy | none |
| Usage | `daisy metrics` (any argument: usage, exit 2) |
| Exit codes | 0, the summary or none recorded yet; 1 outside a workspace, or when the CSV cannot be read, its header is not the columns `metrics add` writes, or a row has the wrong number of cells or a cell `metrics add` would refuse (the row named) |
| Stdout | `No metrics recorded yet (<path> does not exist)` and the line that records one; else `Metrics: <path>`, `  N rows, K superseded by a later record of the same step or run` (a later row with the same kind, plan or swarm, step or run and role supersedes an earlier one: the CSV is only appended to, so a correction is a new row), then one line per kind and model, in that order: rows, tokens (`tokens_total`, else `tokens_in` and `tokens_out` added), cost (`cost_reported`), estimated (`cost_estimated`), measured (rows with lines), code, tests and prose (`+added/-removed` over the measured rows); a sum over only some of the rows reads `<sum> (k/n)`, over none `—`; a model left empty is `(none)` |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | none; `TestMetricsAddRecordsAStep`, `TestSummarize` |

### daisy metrics add

| Field | Value |
|---|---|
| Status | as built (Step 16a of the plan that clears the Deferred list, 2026-11-12): Go (`internal/metrics`, `internal/commands/metrics.go`) |
| Legacy | none |
| Usage | `daisy metrics add --step <s> [--plan <p>] [--range <r> [--repo <dir>]] [<flag> <value>...]`: every word a value flag, in any order, no positional argument. `--step` the step as the plan numbers it (`16a`); `--plan` a plan file's path (the working directory's when relative), else one plan in the home's `plans/` or `plans/archive/` named in full, without `.plan.md`, by its timestamp or by its slug, and without `--plan` the plan `PLAN.md` links to; `--range` the step's commits, `<a>..<b>` or `<commit>^!` (a lone commit would be diffed against the working tree, and a word starting with `-` is refused), measured in `--repo` (default the working directory); `--when` an RFC 3339 time (default now); `--role` (default `step`), `--provider`, `--model`, `--effort`, `--outcome`, `--prompt-hash` and `--notes` free text; `--review-rounds`, `--tokens-in`, `--tokens-out` and `--tokens` (the total, when only that is known) whole numbers; `--cost` (reported) and `--cost-estimated` dollars (`1.25`); `--root-sha` hex, default `git rev-parse HEAD` in `$DAISY_ROOT`, the root the step ran under; `--home-sha` hex, default `git rev-parse HEAD` in the bound home, so the two name every prompt and skill in use (the root's by the root's sha, the home's skills and overrides by the home's). A flag not given is an empty cell, unknown, never zero |
| Lines | `git -C <repo> diff --numstat -z -M <range> --`, never a count the caller gives: each file classed by the path it ends at (a rename by its new name): a `_test.go` file, or one under a directory named `tests` or `test` at any depth, is **tests**; else a `.md` file, or one under a directory named `docs`, is **prose**; anything else is **code**. A binary file (git's `-` counts) is counted as no lines and in `binary_files`. No `--range`: the line cells are empty |
| Exit codes | 0 recorded; 1 outside a workspace, no `--plan` and no `PLAN.md` (or a broken link), a `--plan` naming no plan or more than one (each listed), a cell refused (`Error: --tokens must be a whole number (got: 12k)`), a range refused or `git diff` failing (its first line), the CSV's header not these columns, a file unreadable, the metrics lock held past 30 seconds, a write failing (the CSV put back: `Nothing was recorded.`); 2 for `--step` missing, `--repo` without `--range`, a value flag without a value, a positional word |
| Stdout | `  ✓ Recorded step <s> of <plan>: code +a/-r, tests +a/-r, prose +a/-r` (then ` (N binary file(s), counted as no lines)` when any; ` (no --range: no lines measured)` without one), `  CSV:  <path>`, `  Plan: <real path>` |
| Stderr | errors; `⚠️  root_sha left empty: …` when `$DAISY_ROOT` is no clone and no `--root-sha` is given, `⚠️  home_sha left empty: …` when the home has no commit and no `--home-sha` is given |
| Ops | Real |
| Lock | `$DAISY_LOCAL/metrics.lock` (`env.MetricsLock`), taken before the CSV is read and held until the plan is written (or the CSV put back), so that a row `swarm apply` appends meanwhile (a swarm's run, `docs/swarm.md` 7.6, The run's record) is not written over; held by another writer, it waits, and past 30 seconds refuses, naming the holder (`Error: cannot take <lock>: <lock> is held: <holder> pid <pid>`, `Nothing was recorded.`). One lock a machine, outside the home, whose next commit would sweep a lock file up |
| Writes | `<home>/metrics/metrics.csv` (`env.MetricsFile`; directory created; written atomically, through a link): the header (the columns, in order: `when`, `kind` — `plan-step` here, `swarm-run` for a swarm's run, which `swarm apply` appends to the home `<state>/home` names (`docs/swarm.md` 7.6, The run's record) —, `plan_or_swarm` the plan's file name without `.plan.md`, `step_or_run`, `role`, `provider`, `model`, `effort`, `root_sha`, `home_sha` (empty for a row with no home), `prompt_hash`, `outcome`, `review_rounds`, `tokens_in`, `tokens_out`, `tokens_total`, `cost_reported`, `cost_estimated`, `code_added`, `code_removed`, `tests_added`, `tests_removed`, `prose_added`, `prose_removed`, `binary_files`, `range`, `notes`) when the file is new, then one row, quoted as CSV quotes (Go's CSV writer); the bytes before it are the file's, which must start with that header (else refused, nothing written). The plan, at its real path (never through `PLAN.md`): the step's row of the `## Metrics` table (`\| Step \| Role \| Model \| Effort \| Root \| Home \| Outcome \| Reviews \| Tokens \| Cost \| Code \| Tests \| Prose \| Range \| Notes \|`, the two shas cut to 7 digits, a cell's `\|` escaped and its line ends made spaces): the row of the same step and role replaced when the table has one, else this one appended; no table, it is added at the end of the section; no section, the section goes after `## Decisions`, else at the end of the plan; every other byte of the plan as it was, but the blank line before an added section |
| Commits | none: the next home commit sweeps both files up, as for `feedback` |
| Cases | none; `TestMetricsAddRecordsAStep`, `TestMetricsAddRefusals`, `TestClassify`, `TestCount`, `TestAppendCSV`, `TestPlanRowInsertsAfterDecisions`, `TestPlanRowPlacement`, `TestAppendUnderTheLockLosesNoRow`, `TestLockGivesUpNamingTheHolder` |

## Homes and identities

### daisy home

| Field | Value |
|---|---|
| Status | as built (Step 21): Go, four rows of the `cli` table (`home` is a noun, D35) |
| Legacy | `daisy home [list\|add\|remove\|new]` → the bash `home` script, deleted in Step 21 |
| Usage | `daisy home list`, `daisy home add <name> <path>`, `daisy home remove <name>`, `daisy home new <name> [path]`; bare `daisy home` or another verb: exit 2 listing the four usages (bare `daisy home` was `home list` in bash) |
| Exit codes | `list`: 0 [68] (69 and 70 run it and assert stdout only). `add`: 0 registered [68]; 1 when the name is `already registered` [68], invalid (`bad=name`, case 68), the path is not a directory [68], has a path problem (`Error: <path> <problem>`: the root itself — `cannot be a home` [68] — or a directory containing it, `contains the Daisy root; …`, which case 68 pins through a registry line and `status`, not through `home add`; the path is already resolved here, so the other two problems cannot arise), has no `include.txt` [68], is `not in its own git repository` [70] (the root's is not one, since Step 24), or nothing under it can be staged. `remove`: 0 [68]; 1 for an unregistered name [68]. `new`: 0 created [69]; 1 when the name is registered (nothing written, case 69) or the location `already exists` [69]; 1 when any step after the first write fails, up to the registration — the copy is removed: `was removed and nothing is registered` [70] on stderr (the build of `<home>/AGENTS.md` after it is a warning, exit 0, the home kept: its `Error: …` and `⚠ AGENTS.md not built; run 'daisy prompt build <name>'` on stderr); 1 when a prompt gets EOF |
| Stdout | `list`: `%-14s %s` name and path per home (`testhome` [68] and its path [68, 69]; ` (missing)` when the directory is gone); no homes: `No homes registered in <registry>` on stderr, exit 0. `add`: `✓ Registered home '<name>' at <path>`. `remove`: `✓ Unregistered home '<name>' (its files at <path> are untouched)`. `new`: the prompts `  Location [<default>]: ` (only without `[path]`), `Git identity for home '<name>':`, `  Name: `, `  Email: ` through `Ops.Prompt` (the questions on stderr); `  ✓ Saved git identity to <path>/gitconfig` or `  ⚠ Skipped (both name and email required)…`; `  ✓ Initialized git repository with first commit`; `✅ Created home: <name>`, `   Location: <path>`, `📋 Next steps:` … `daisy init <name>` |
| Stderr | the errors above, each with its fix on a second line (`include.txt` [68] names what a home must contain); `add` refusing a non-repository says `not in its own git repository` [70] |
| Ops | `list` ReadOnly; `add`, `remove`, `new` Real (decided, D30) |
| Writes | the registry `$DAISY_CONFIG/homes` (`name=/absolute/path` lines; `add` stores the resolved real path). `new`: copies `templates/home/` to the location (default `$DAISY_LOCAL/homes/<name>`), writes `gitconfig` as a `name=` [69] line and an `email=` [69] line, `git init` with `user.name`/`user.email` set and a first commit `New home: <name>` as that identity [69], registers, builds `<home>/AGENTS.md`; the template `.gitignore` ignores `.healthcheck.ttl` and `AGENTS.md` [69]; any failure up to the registration removes the copy and registers nothing [70] (a failed build is a warning) |
| Commits | `new`: the new home's first commit; otherwise none |
| Cases | 68, 69, 70 |

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
| Stdout | one line per identity across the registered homes, `%-14s %s`, identity then home: each home's own name first, then its `identities/*` in name order — the list `init`'s unknown-identity error prints |
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
| Writes | the output file (mode 0644, replaced atomically) and, for the workspace's build, `.daisy/prompts/<name>.md` for each layered override in the home's `prompts/`, included or not (D49) — the merged prompt, byte for byte `prompt.Merge`: the root's file with the home's rules appended to its `## Rules` section after one blank line, renumbered after the root's rules (a root without the section gets one where `optimize apply --rules` puts it) — any other `*.md` there removed [89]; a layered file that is not exactly the marker and one `## Rules` section fails the build, naming the file and line. When the home overrides any root prompt, a `# Home overrides: wherever a prompt names the root's file, read the home's` block follows the header, one line per override (`` - `<root file>`: read `<file>` instead (a layered\|full override) ``; a home's own build, which materialises nothing, says of a layered one `` - `<root file>`: read it, then the rules in `<home file>`, which follow its own (a layered override) ``), and every `$DAISY_ROOT/prompts/<name>.md` of an overridden prompt in the text written — a lazy stub or a whole prompt — names that file instead: the home's for a full override, the materialised one for a layered [89]; a home without overrides builds as before. A layered file with no root file under it fails the build. A home's own build (`<home>`) materialises nothing and takes a lazy layered prompt in whole. The output: a header — `# Daisy Prompt - Auto-generated`, `# Home: <home>`, `# Built: <YYYY-MM-DD HH:MM TZ>`, `# Source: <home>/include.txt`, `#`, `# This file is generated by daisy prompt build`, `# Output: <file>`, `# Do not edit directly - edit source prompts in prompts/ directory`, `# To rebuild: <command>` (`daisy prompt build <home>` for a home's file; `daisy prompt build`, run in the workspace, for any other), a blank line — then per `include.txt` entry a banner and the prompt: whole for `name`, the `## Trigger` section (the lines up to the next line starting `#`, trailing blank lines dropped) for `~name`, resolved in `<home>/prompts/` then `$DAISY_ROOT/prompts/`, every literal `$DAISY_ROOT` replaced by the root. `include.txt`: blank lines and lines whose first non-space is `#` skipped; each other line's words read as `xargs` reads them (blanks collapsed, quotes and backslashes honoured) and a leading `~` making it lazy. For a home without overrides, byte for byte the bash's output except the `Built:`, `This file is generated by` and `To rebuild:` lines (Step 20, D35) |
| Commits | none |
| Cases | 84; every `init` case (32, 33, 76–79) and `home new` (69) build through it |

### daisy install

| Field | Value |
|---|---|
| Status | as built (Step 21): Go; non-interactive in both forms (the bash asked for a default home and wrote a variable naming it, which nothing reads since Step 24); writes nothing under `~/.claude` since Step 13 of the plan that clears the Deferred list (2026-11-12): skills and permissions are `init`'s, per workspace |
| Legacy | `daisy install [--update]` → built in to `daisy.sh`, deleted in Step 21 |
| Usage | `daisy install [--update]`: `--update` re-applies the symlink and takes back what earlier installs put under `~/.claude` (Writes); plain `install` also writes the shell rc line and refuses when already installed, the link and the rc line both naming this root (`Error: Daisy is already installed.`, exit 1; a link already right with the rc line naming another root is installed, D47). The root is the one whose `bin/daisy` is running (`Ops.Executable()`, `<exe>/../..` with links resolved; a `$DAISY_ROOT` that resolves to it is used as set), so `<root>/bin/daisy install` bootstraps a shell with nothing set; a `$DAISY_ROOT` that resolves to another root is refused before any write, both forms (`Error: DAISY_ROOT is <set>, but this binary (<exe>) is in <root>` [86], then the fix, plain `install` from either form since it also writes the rc line the export came from: `DAISY_ROOT=<root> <root>/bin/daisy install` or unset it, and `<set>/bin/daisy install` for the other root, and a `grep` for an export in another rc file, for a new shell refused again; D47 — a stale export had sent an install, and `~/bin/daisy`, to the old root). It does not build: `make build` writes the binary, and `install` warns when it is not built yet |
| Exit codes | 0 [31]; 1 already installed, `~/bin/daisy` exists and is not a symlink, `~/.claude/settings.json` cannot be read or is not a JSON object (decided before any write), `HOME` unset, no `DAISY_ROOT` and the binary not under a root's `bin/`, a `DAISY_ROOT` other than the binary's root [86] or set with the binary's own path unknown |
| Stdout | `Installing Daisy...` / `Updating Daisy...`, `  DAISY_ROOT: <root>`, ✓ lines per step, `  ✓ Removed N copies of root skills from ~/.claude/skills (<names>); Daisy installs skills in each workspace now: run 'daisy init' there` (`Removed 1 copy of a root skill from ~/.claude/skills (testing)` [31]) and `  · Kept ~/.claude/skills/<name>: named as a root skill but holding a file no version of it has (an edit, or a file of your own); remove it by hand if it is Daisy's` (`Kept ~/.claude/skills/git` [31]) per directory kept, `  ✓ Removed Daisy's permissions from ~/.claude/settings.json (each workspace's 'daisy init' grants them now)` or `  ✓ Removed ~/.claude/settings.json (it held only Daisy's permissions, …)`, each only when it did so, `  Shell config: <rc>` and `  ✓ Added DAISY_ROOT to <rc>` / `  ✓ DAISY_ROOT already set correctly` / `  ✓ Updated DAISY_ROOT (was: <old>)`, `Done.` / `Done. To activate, run:` … `daisy init <home>` (or `daisy home new <name>` first when none is registered) |
| Ops | Real |
| Writes | `$HOME/bin/daisy` symlink; under `$HOME/.claude` it creates nothing and rewrites a file only to take one of Daisy's rules out [31]: it takes back what earlier installs put there, and only that. Of `$HOME/.claude/skills`, it removes each copy of a root skill (`skill.RootCopies`): a directory named as a skill the root has or has had (a directory holding a `SKILL.md` under `skills/`, in the working tree or any commit of the root's history, `git log --relative --raw -- skills`), holding a `SKILL.md`, every file in it (a link by its target) byte for byte a version of the file at that path under such a directory, in the working tree or the history (compared by git blob id, in the root's object format), so a copy an older root installed is removed [31] and a copy edited, or holding a file of the user's, is kept and named [31]; a skill of another name, `synced` (where Claude Code's own skill sync writes) [31], a hidden name and a link are never touched; a root that is not a repository has its working tree alone (`TestRootCopies`). Of `$HOME/.claude/settings.json`, it removes Daisy's allow rules in every form an install wrote (`Read(/$DAISY_ROOT/**)`, `Edit(/$DAISY_ROOT/**)` — the leading slash plus the absolute root gives the `//`-anchored form — `Bash(daisy:*)`, the legacy `Bash(…/daisy/scripts/*)`, single-slash and triple-slash rules), the rest kept in place, the file re-indented when a rule came out and removed when nothing else is left, as `clean` does; a file holding none of them is not written [31] (`TestInstallRemovesOnlyRootSkillCopies`); without `--update`: the rc file by `$SHELL` (`~/.zshenv`, `~/.bashrc`, `~/.config/fish/config.fish`, else `~/.profile`): every line starting (after any indent, which is kept) with `export DAISY_ROOT=` (fish: `set -gx DAISY_ROOT `) replaced in place unless every one already names the root, quoted or not (a later stale one is what the shell keeps; the lines checked are the lines rewritten, so a comment or a line using `$DAISY_ROOT` is neither, D47), else, with no such line, `# daisy` and the line appended; a file kept as a symlink is written through. Applying twice equals once |
| Commits | none |
| Cases | 31, 86; `TestInstall`, `TestInstallBootstrapsFromTheBinary`, `TestInstallRefusesAnotherRoot`, `TestInstallRewritesAStaleRcLine`, `TestInstallRemovesOnlyRootSkillCopies`, `TestInstallRoot`, `TestRootCopies` |

### daisy feedback

| Field | Value |
|---|---|
| Status | as built (Step 23): Go; the bash's bytes but where D38 says |
| Legacy | `daisy feedback [--workflow <n>] <text>` → `feedback.sh`, deleted in Step 23 |
| Usage | `daisy feedback [--workflow <n>] <text...>`; `echo <text> \| daisy feedback [--workflow <n>]`; `-w` is `--workflow` (the last given wins); the words are joined by one space; piped text loses its trailing newlines |
| Arguments | `<n>`: `prompts/<name>`, `skills/<name>`, or a bare name resolved as a prompt (the home's, then the root's), a skill source (the home's over the root's), or an alias for `prompts/daisy` — the home's over the root's, as for `daisy` — (`new-day`, `new-week`, `done`, `log`, `add`, `priority`, `cancel`, `status`, `sync`, `project`, `projects`, `home`); a name holding `/` names nothing; a tag that names no file is refused before anything is written [67] |
| Exit codes | 0 recorded [67]; 1 for an unresolvable workflow [67], no message on a terminal (usage and the workflow list on stderr), an empty message (`feedback ""` included: an empty word is a message given, so stdin is not read), a missing `feedback/feedback.md`; 2 for `--workflow` without a value |
| Stdout | `  ✓ Feedback recorded (workflow: <n>, <YYYY-MM-DD HH:MM>)` (no `workflow: …, ` without one), `  File: <path>` |
| Stderr | `Error: workflow '<n>' names no prompt or skill` (`names no prompt or skill` [67]), `  Qualify the target, e.g. --workflow prompts/daisy or --workflow skills/git.`, `  Available:` and every target, byte order (`prompts/daisy` [67], `skills/git` [67]) |
| Ops | Real |
| Writes | appends to `feedback/feedback.md` (through a link, atomically): blank, `## <YYYY-MM-DD HHMM>`, `**Workflow:** <n>` (`**Workflow:** skills/git` [67]) when given, blank, the text; nothing on refusal (byte-identical, case 67) |
| Commits | none (decided, D38: the entry is swept up by the next home commit, as for `plan new`) |
| Cases | 67; `TestFeedback` |

### daisy optimize

| Field | Value |
|---|---|
| Status | as built (Step 23): Go: the redesign agreed in D12 — no API key; the briefing and `apply` |
| Legacy | `daisy optimize [--workflow <n>] [--dry-run] [--force] [--list]` → `optimize.sh`, deleted in Step 23 with the 2-entry floor, `--force`, `--dry-run`, `--list`, `EDITOR` and the `y/N/e` prompt |
| Usage | `daisy optimize [--workflow <n>]` prints the briefing; `daisy optimize apply --workflow <n> (--rules\|--rewrite) <file> [--yes]` applies text the agent proposed (`-w` is `--workflow`; `<file>` is the working directory's when relative) |
| Targets | entries group by the file their `**Workflow:**` tag resolves to (as `feedback` resolves it), in order of first appearance; two tags naming one file are one target (`[prompts/plan] 2 entries` [38]); an unheadered entry is `prompts/daisy`'s (`[prompts/daisy] 1 entry` [41]); `--workflow <n>` scopes to the target `<n>` names (`[prompts/plan] 1 entry` [40]; one naming nothing is refused, exit 1 [40]); a skill resolves to its source `SKILL.md` (`Target: <home>/skills/widgetcraft/SKILL.md` [49]); a tag naming no file is a group of its own, briefed with the reason (`[daisy-write] 1 entry` [67], `names no prompt or skill` [67]) and never defaulted to daisy.md |
| Rules | the `## Rules` section runs from its heading (`## Rules`, or `## Rules` and more after a space — a heading such as `## Rules of the loop`, which no shipped file has today but `TestRulesSectionBoundary` pins — as the bash's `/^## Rules/` read it, bounded at the word; closing `#`s allowed) to the next `#` or `##` heading; an `###` is inside it [53]; Daisy reads ATX headings, LF or CRLF line ends (up to three spaces, one to six `#`, then a space, a tab or the line's end) and fenced code (three or more backticks or tildes, a backtick fence's info string holding no backtick — "```x``` …" is inline code — closed only by a run of the same character at least as long, then only spaces or tabs); a line in a fence is no heading. Other markdown (setext headings, containers, HTML blocks) is not read: where a section, the target's or a proposal's, holds a line of only `=` or `-` straight after a non-blank line other than a heading (to another reader, perhaps a setext underline; a list item above is no exception), `apply --rules` refuses, naming the line and the file, rather than guess where the section ends; a blank line above it makes it a thematic break. So does a fence opened in the target's section and never closed (the section would run to the end of the file), or, without a section, a fence left open at the end of the file when no `## ` heading other than the Trigger precedes it (the rules would be appended inside it); the briefing prints the refusal under the target (`--rules refused: …`) (D38). A rule is a list item at the left margin (`N. `, `- `, `* `) outside a fence (`Rules: 2` [53]). `daisy help <skill>` prints the same section |
| Briefing | under `ReadOnly` [57]. `Pending feedback: N entries for N targets (<feedback.md>)` [38], or `Nothing pending in <feedback.md>` [56]; per target `[<label>] N entries` [38], `Target: <path>` [38] — then ` (root — apply writes a home override)` for a target in the root, ` (home override, layered)` or ` (home override, full)` for one the home overrides (`, behind the root: …` added when `feedback/.overrides` says the root changed since the copy) [62, 88]; an override's text, its rules and their count are the merged ones (D49) —, `Rules: N` [39, 56] (`Rules: N — more than 10: rewrite flagged` [56]; `Rules: none …`), `--- entries ---` and each entry as written, `--- current ## Rules ---` and the section [39, 53], `--- end ---`; a vendored skill (carries `LICENSE`/`NOTICE.md`) is listed with the reason (`vendored` [50]) and its entries stay pending; then `Rewrite flagged (more than 10 rules, nothing pending):` and `[<label>] rewrite flagged: N rules` [56] for every active target over 10 rules (the prompts `include.txt` names, in its order, then every skill source by name; 10 is not flagged [56]); then how to apply (`daisy optimize apply --workflow <label> --rules` [39], `--rewrite` [56]). No floor: one entry is briefed (`Skipped` [39] and `floor` [39] absent) |
| Apply | `--rules <file>`: the file must be one `## Rules` section — its first non-blank line the heading (`must begin with` [61]), no other `#`/`##` heading outside a fence (named with its line, `## Another Section` [61]), no fence left open; and the result must read back as the section written, so nothing after it is swallowed; it replaces the target's section, keeping the blank lines between the section and the next heading as they were, so replacing the rules with themselves changes nothing, byte for byte [53]; `# Document Title` [53, 58], `## Next Section` [53] and `Body text` [53, 58] survive; a target without a section gets one before its first `## ` heading other than the Trigger, or at the end. `--rewrite <file>`: the file, byte for byte [60]; an empty one is refused. Both print the unified diff (`-1. Measure before cutting.` [58], `+2. Cut once.` [58]) and ask `Apply these changes to <label>? [y/N] ` [58] on stderr through `Ops.Prompt`, unless `--yes` (never asks [60]). `y`/`yes` (any case) applies; anything else is `Cancelled` [59]; EOF is `Cancelled — no answer on stdin (EOF); nothing written.` (`EOF` [64]); any other failure to read the answer is `Error: cannot read the answer: …`, exit 1, nothing written. The writes are computed from what was read before the question: if the target, `feedback.md` or `archive.md` changed while it waited (an entry recorded meanwhile), the apply is refused, exit 1, nothing written. Applied: the target (`  ✓ Updated ` [58] `<path>`), the target's pending entries — every tag naming it [58] — moved from `feedback.md` to `archive.md`, each followed one blank line down by `**Addressed by:** `` `daisy optimize` `` <YYYY-MM-DD> — `` `<target>` `` (rules\|rewrite\|layered rules\|full rewrite)` [63, 62, 88], `<target>` relative to the home — the override's path for a root target [62] — or, when nothing is written (rules adding nothing to a root target), the file as it stands (`$DAISY_ROOT/…`) with the plain mode; `feedback.md` keeps its header and the other entries, one blank line apart, byte for byte [66]; `feedback/.last-optimized` gets `<YYYY-MM-DD HHMM> <label>` [63]; for a prompt, the home's `AGENTS.md` and the workspace's `.daisy/AGENTS.md` and `.daisy/prompts/` (see `prompt build`) are rebuilt from the prompts as written (for a skill, the run says `daisy init` in each workspace propagates it [49]; for the home's override of a root skill, `daisy init` in this one); the home is committed once [58, 60, 62]. **A target in the root is never written (D49).** For one with no home file, or a layered override, apply writes the home's file at the same path (`<home>/prompts/<name>.md`, `<home>/skills/<name>/SKILL.md`), says `Writes the home's <form> override <path>; the root's <path> is not written.`, and the root's tree is byte-identical [62, 88]: `--rules` a **layered** override — the line `<!-- daisy: upstream body -->` then `## Rules` holding the home's rules: the proposal is the merged section as briefed, the root's section must stand first in it unchanged (a root rule changed or dropped is refused with the proposal's line, `use --rewrite`) and the rules after it are the home's, renumbered from 1; applying again rewrites the override only [62]; adding nothing to a root target writes no override. `--rewrite` a **full** override, the proposal whole, the diff from the root's (or the merged) text, and `<rel> <sha256 of the root file>` in `feedback/.overrides` [88]; over a layered override it says `The layered override is replaced by a full override`. A home prompt, and a full override, are written in place as before; a `--rewrite` of a full override records the root's hash as it is now. Applying text already there with nothing pending: `Nothing to apply: …`, exit 0 |
| Exit codes | 0 (nothing pending, a briefing, an applied, declined or EOF apply, nothing to apply); 1 when `feedback/feedback.md` does not exist (`Error: No feedback file found at <path>` and the `mkdir -p … && touch …` that makes it), a target the briefing cannot read (printed in its place, the errors after the briefing, `The briefing above is incomplete.`), a file changed during the question, `archive.md` absent with no template to start it from when entries are to move, a workflow naming no file [40, 61] (`names no prompt or skill`), a vendored target on `apply` [50], a proposal that cannot be read (`cannot read` [61]) or is not one rules section [61], the home not a repository, a write or the AGENTS.md build failing (every write taken back: `Nothing was applied.`), the commit failing (the files written, the `daisy commit <message>` to run printed), a target `--rules` cannot write (the open-fence and setext refusals of the Rules row, naming the target); 2 for a usage error [57, 61]: `--dry-run`, `--force`, `--list` [57] or any argument to the briefing, bare `optimize apply` [61], no `--workflow`, neither or both of `--rules`/`--rewrite`, a value flag without a value (an empty word, or one of the command's own flags, is none) [61], a stray word [61] |
| Ops | briefing `ReadOnly` [57]; `apply` Real |
| Writes | `apply` only, every refusal before the first write and a failed write taking back the ones before it: the target prompt or `SKILL.md` in the home (never the root's [62, 88]), `feedback/feedback.md`, `feedback/.overrides` (a full override over the root), `feedback/archive.md` (started from `templates/home/feedback/archive.md` when absent; no template is an error when entries are to move), `feedback/.last-optimized` (`healthcheck` Check 7 reads it), the home's `AGENTS.md` and the workspace's `.daisy/AGENTS.md` and `.daisy/prompts/`. Daisy writes by path and never at a root path; as a best-effort check, not a guarantee, a write whose path, its links resolved, lies in the root (a user-made link: a home file or directory linked to the root's, a `.daisy/prompts` linked into it) is refused before any write, `Error: <path> leads into the root (<real path>), which Daisy never writes; …`, `Nothing was applied.` (`TestNoWriteReachesTheRootThroughALink`; `prompt build` the same); a registered home inside the root's directory is not the root, as for a root commit (`TestHomeInsideTheRootIsWritten`); a link at a materialised path is replaced, never written through. The suite's fixture copies `prompts/` and `skills/`, so not even a fault could write the repository's (case 65) |
| Commits | `apply`: the home, once, `Optimize <label> (<mode>)` [58], holding the override and the feedback files, never the materialised prompts (the workspace's) [62]; never the root [58, 62, 88] |
| Cases | 38, 39, 40, 41, 49, 50, 53, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 88; `TestApplyRulesOnARootTargetLayers`, `TestApplyRewriteOnARootTargetIsFull`, `TestOptimizeBriefsOverrides`, `TestSkillOverrides`, `TestOptimizeApplyApproval`, `TestOptimizeApplyArchivesAndCommitsOnce`, `TestOptimizeApplyRollsBack`, `TestOptimizeUsage`, `TestOptimizeBriefing` |

### daisy eval

| Field | Value |
|---|---|
| Status | as built (Step 23): Go; the bash's bytes but where D38 says |
| Legacy | `daisy eval [<case>] [--record]` → `eval.sh`, deleted in Step 23 |
| Usage | `daisy eval` lists the cases in `$DAISY_ROOT/prompts/evals/*.md`; `daisy eval <case>` prints one; `daisy eval --record <case> pass\|fail ["notes"]` records a result (the help line `eval [<case>] [--record]` abbreviates this form; `--record` leads). With no notes and a terminal, asks `Notes (optional): ` through `Ops.Prompt` |
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
| Status | as built (Step 1 of the swarm build; finding the swarm, `<state>/env` and the unit's root at Step 2 of revision 6's plan, the product repositories by forge name and the mirrors' remotes at Step 3, the swarm repository's mirror and question 32's swarm-repository paths at Step 4, `repos/<name>.md`'s seed at Step 5 (the declared repositories' mirrors fetched before the files, the swarm repository's after them), the rest of revision 6 at its Step 6 (the swarm repository created or adopted with the maintainer's token, cloned, seeded, committed and pushed, `<state>/env` written; section 14, item 109), a variable unset at `init` written commented out at Step 6a (item 110), question 3's default from `--api` or `$GITEA_API_URL`/`$GITHUB_API_URL` at Step 6b (item 111), docs/swarm.md section 11, Finding the swarm's and init's As built): Go (`internal/swarm/declare`, `internal/swarm/forge`, `internal/commands/swarm.go`, `internal/commands/swarm_init.go`); the contract is [docs/swarm.md](swarm.md) section 11, with `swarm.json` in section 3 and the forge calls in section 10 |
| Legacy | none |
| Usage | `daisy swarm init [--defaults] [--api <url>]`: in the swarm repository's clone, or in any folder holding none (the clones directly under it giving the defaults, the clone made there as `<repo>`; docs/swarm.md section 11), asks the wizard's questions (below) through `Ops.Prompt` on stderr, an empty answer taking the default in brackets (the existing `swarm.json`'s value, else detection, else section 3's example); `--defaults` asks nothing; `--api <url>` is question 3's (the forge API's) default, over the file's, with or without `--defaults` |
| Exit codes | 0 written or unchanged, or cancelled at EOF (`Cancelled — no answer on stdin (EOF); nothing written.`); 1 an invalid file or default (`Error: swarm.json: <field>: <problem>`), a token unset (`Error: swarm: $<VAR> is not set (in <state>/env or the environment)`), `DAISY_MAINTAINER_TOKEN` the swarm's or its account, kind `github` (no adapter yet), the maintainer's token unable to read, create or open the swarm repository (`Error: swarm: $DAISY_MAINTAINER_TOKEN cannot …`), the clone refused (`<dir>/<repo>` not a clone of the swarm repository, or one holding `swarm.json`, which `init` is run in instead), the swarm's token unable to read the swarm repository, a declared repository or its own account (`Error: swarm: $<VAR> cannot read …`), a write failing, the push refused (the commit stays in the clone), a mirror's fetch failing, `EnsureLabels` failing (a refused rename or create names every label still to rename and to create; the files stay written and a re-run completes it); 2 usage. A readiness warning never changes it |
| Stdout | each write named with its token: `✓ Maintainer <login>: <owner>/<repo> created` (or `  … exists`) and `✓ Maintainer <login>: <swarm-login> may write to <owner>/<repo>`; `✓ Cloned <owner>/<repo> into <swarm>` when it cloned; `✓ Token $<VAR> reads <owner>/<repo> as <login>`; `✓ Wrote swarm.json` or `  Unchanged swarm.json`; per role `✓ Wrote prompts/<role>.md` or `  Kept prompts/<role>.md (exists)`, and so for each seeded file; `✓ Committed and pushed <short sha> to <owner>/<repo>'s <branch> as <git author>` or `  Nothing to commit`; `✓ Wrote <state>/env (mode 0600)` or `  Kept <state>/env (exists)`; `✓ Mirrors: <repo>, <name>, … fetched`; `✓ Token $<VAR>: labels 16 present (<c> created, <r> renamed)`; the systemd user service that runs `apply`'s loop (restarted one `interval` after a failure; a stop waiting one `timeout`, `TimeoutStopSec`, for the drain: Step 35 of the swarm build), and its install line |
| Stderr | the questions; a refused answer's message, then the question again; `⚠️  <state>/env <problem>` for a line the swarm's secrets file cannot be read past; `⚠ <state>/env: $<VAR> is not set; its line is commented out: fill it in, or set $<VAR> in apply's environment` for each variable unset (or empty) in its environment, written commented out; the four readiness warnings (`⚠ Readiness: …`) |
| Ops | Real |
| Writes | on the forge with `DAISY_MAINTAINER_TOKEN`, the swarm repository when absent and the swarm's account's write access to it; the swarm repository's clone when `init` runs outside one; in the clone, `swarm.json` when the answers differ from it and each other file of docs/swarm.md section 2's table (`prompts/<role>.md`, `repos/<name>.md`, the READMEs, `.gitignore`) that is absent, then one commit of those files and of each file of that table the clone holds and its `HEAD` lacks, and its push; `<state>/env` when absent, mode `0600`, a line `export <VAR>='<value>'` per variable and `# <VAR>=` for one unset in its environment, which assigns nothing, so the environment supplies it; the mirrors under `<state>/git/`; on the forge with the swarm's token, the organization's labels of the sixteen (the nine stages, the flags `swarm:gated`, `swarm:blocked`, `swarm:paused`, `swarm:blocking` and `swarm:class-architecture`, the terminals `swarm:done` and `swarm:rejected`: docs/swarm.md section 5), each of the seven stage labels found under its name before Step 15 of the swarm build renamed in place (section 14, item 74) and every other missing one created |
| Commits | in the swarm repository's clone, the files it wrote and each file of docs/swarm.md section 2's table the clone holds and its `HEAD` lacks (an earlier run's whose commit failed), by the clone's own git identity (the maintainer's), its message `swarm: written by init`, pushed to the repository's default branch |
| Cases | none; `TestSwarmInitLabelsThroughGitea`, `TestSwarmInitCreatesThroughGitea`, `TestSwarmInitRefusals`, `TestSwarmInitCreatesTheSwarmRepository`, `TestSwarmInitAdoptsTheSwarmRepository`, `TestSwarmInitRefusesInAFolder`, `TestSwarmInitSeedsRepositoryNotes`, and the `internal/commands` and `internal/swarm` tests |

The wizard's questions, in order (defaults in docs/swarm.md section 11):
1 the product repositories (forge names, `<owner>/<name>`, the product
first; each one's `ssh_command` taken from its clone when one lies
directly under the working folder); 2 the forge kind;
3 the forge API (`--api`'s value, else the file's, else `$GITEA_API_URL` or
`$GITHUB_API_URL` by kind when set and not empty, else detected from the
origin); 4 the swarm repository, which holds the issues; 5 the token variable; 6 the base
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
`<name>:<path>` in a declared one; `briefs/` and `docs/` by default);
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
| Status | as built (Step 7 of the plan that clears the Deferred list, 2026-11-12): Go (`internal/swarm/declare/prompts.go`, the table of earlier defaults and `ComparePrompt`; `internal/commands/swarm.go`); the contract is [docs/swarm.md](swarm.md) section 11, prompt build, and section 14, item 117 |
| Legacy | none |
| Usage | `daisy swarm prompt build [--force] [--dry-run]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); each `prompts/<role>.md` compared with this Daisy's default for its role: the current default left as it is, an earlier default (its sha256 in the binary's table of every default the role has had) or an absent prompt written as the current, an edited one kept with its diff from the default printed; `--force` overwrites an edited one too; `--dry-run` runs read-only, writes nothing and wins over `--force`, which it previews; any argument is usage on stderr, exit 2 |
| Exit codes | 0 every prompt current or written; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, the root's commit, a prompt or the origin's default branch unreadable, a write, the commit, the push or the mirror's fetch failing (the push refused: `Error: swarm: push <swarm>: <git's error>; the commit stays in the clone: pull, then push it`), and an edited prompt kept, reported after the others are written, committed and pushed (`Error: swarm: edited, so kept: prompts/<role>.md, … (each diff above); merge the default's changes in by hand, or run 'daisy swarm prompt build --force' to overwrite them`), under `--dry-run` too; 2 usage |
| Stdout | per role, in order: `  Unchanged prompts/<role>.md`, `✓ Rebuilt prompts/<role>.md (an earlier default)`, `✓ Wrote prompts/<role>.md` (absent), `  Kept prompts/<role>.md (edited), its diff from the default:` or with `--force` `✓ Rebuilt prompts/<role>.md (edited, overwritten by --force), its diff from the default:`, each followed by the unified diff (`--- prompts/<role>.md (default)`, `+++ prompts/<role>.md`); under `--dry-run`, `  Would write` and `  Would rebuild` for the writes, then `  Dry run: nothing written; would commit "<message>" and push it` or `  Dry run: nothing to write`; else `✓ Committed and pushed <short sha> to <owner>/<repo>'s <branch> as <git author>` and `✓ Mirrors: <repo> fetched`, or `  Nothing to commit` |
| Stderr | errors |
| Ops | Real; `ReadOnly` with `--dry-run` |
| Writes | in the clone, the prompts above; the swarm repository's mirror, `<state>/git/<repo>`, fetched after the push; no forge call, no secret read |
| Commits | in the swarm repository's clone, the prompts it wrote and nothing else, by the clone's own git identity, its message `swarm: prompts rebuilt from the root at <short sha> (<role>, …)`, `<short sha>` the Daisy root's `HEAD`, pushed to the repository's default branch as its origin's `HEAD` names it (`git ls-remote --symref origin HEAD`); nothing written, no commit |
| Cases | none; `TestSwarmPromptBuild`, `TestComparePrompt`, `TestDefaultHistoryHoldsEachCurrentDefault` |

### daisy swarm status

| Field | Value |
|---|---|
| Status | as built (Steps 4, 8, 9, 14, 15, 20, 27 and 29 of the swarm build; finding the swarm and `<state>/env` at Step 2 of revision 6's plan, the briefs read in the mirrors at Step 4, Drift at Step 5): Go (`internal/swarm/forge`, `internal/swarm/reconcile`, `internal/swarm/brief`, `internal/commands/swarm.go`); the contract is [docs/swarm.md](swarm.md) section 11 |
| Legacy | none |
| Usage | `daisy swarm status`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); any argument is usage on stderr, exit 2 |
| Exit codes | 0, also when Docker cannot be reached; 1 no swarm found, `swarm.json` absent or invalid (`Error: swarm.json: <field>: <problem>`), the token unset (`Error: swarm: $<VAR> is not set (in <state>/env or the environment)`), a forge call failing (`Error: swarm: forge: …`), a brief, `<state>/auto`, a run's `result` or a batch run's `out.md` unreadable; 2 usage; with exit 1, nothing on stdout: the report is written whole once every read is made |
| Stdout | `Swarm: <owner>/<repo> on <forge-host> (<kind>), repositories <name>, …`; `State: <state>` with the paused and held notes (` — owner batch held: no brief is active` while no brief under `resources` is `active`: the owner's batch waits for one, docs/swarm.md section 6); `Gates: …`; a line per stage, `  %-26s %d` and its issue numbers; `Waiting for you: …`, the open issues the predicate `apply` sets `swarm:gated` by holds for (`triage`, `plan`, a `review-2-approved` issue under `merge`, `next slice`, `close` or, its current pull request closed unmerged, `pull closed` as that pull request calls for, and `blocked` every issue carrying `swarm:blocked`, at whatever stage), each `#<n>` under `triage` and `plan`, where only an architecture job waits, and `#<n> (<class>)`, its job class (`routine` or `architecture`, docs/swarm.md section 5) beside it, under the others, or `nothing`; `Held: …`, `paused #…` for the issues carrying `swarm:paused` and `#<n> blocked by #<m>` for each whose newest maintainer comment names an open blocker, separated by `; `, or `nothing`; `Running: …`, `none` or `unknown (docker: <message>)`; `Personas:` and a line per role; `Throughput:` and its six lines (`auto` the gates Daisy passed for routine jobs, per issue the larger of its lines in `<state>/auto` and its auto notes on the open and recently closed issues; `stage time` the median run time per stage, a run recorded before Step 15 of the swarm build counted under its stage's new name), then a `#<n>` line of correction counts per open or recently closed issue with a plan, a pull request or a block; `Briefs:` and a line per `active` brief under `resources` (its milestones' open and closed issues as the forge counts them in the milestones that mirror them, those with none or not yet on the forge, and `met (owner run <k>)` when the newest batch run reports it; one whose file name an earlier `active` brief has, `<path>: shares the name <name> with <other path>; mirrored to no milestone`), or `Briefs: none active` |
| Stderr | `⚠️  <state>/env <problem>` for a line the swarm's secrets file cannot be read past; `⚠ <swarm>/.env.sh: …`; in a swarm repository's clone, each time, `⚠ Drift: <path> differs from <owner>/<repo>'s <branch> at <short sha>; commit and push it` (`is not in`, `is only in`) per path of `swarm.json` and `prompts/` that differs from the swarm repository's default branch as its mirror was last fetched (docs/swarm.md section 11, Drift; never an exit code); `⚠ #<n>: Blocked by #<m> names no issue; not held` (no hold listed for it); errors |
| Ops | ReadOnly, with a forge `GET` or `HEAD`, `docker ps` and (revision 6) git's reading commands on a mirror let through |
| Writes | nothing, anywhere |
| Commits | none |
| Cases | none; `TestSwarmStatusThroughGitea`, `TestSwarmStatusStages`, `TestSwarmStatusStep7`, `TestSwarmStatusCorrections`, `TestSwarmApplyGatedFlag`, `TestSwarmApplyRoutineJob`, `TestSwarmStatusWritesNothingOnAFailure`, `TestSwarmStatusPullClosedUnmerged`, `TestSwarmStatusBriefsShareAName`, `TestSwarmStatusAutoKeepsOldJobs`, `TestSwarmApplyNoteOnAMovedIssueBlocksNothing`, `TestSwarmDrift`, and the other `internal/commands` and `internal/swarm` tests |

### daisy swarm apply

| Field | Value |
|---|---|
| Status | as built (Steps 5, 6, 9, 14, 15, 18, 19, 20, 23, 24, 25, 26, 27, 29 and 32 of the swarm build; finding the swarm, `<state>/env` and `docker` and `id` in the swarm's root at Step 2 of revision 6's plan, the resources, references and briefs read from the mirrors at the pass's commits at Step 4, `repos/<name>.md` in a member's `system.md` and Drift at Step 5): Go (`internal/swarm/provider`, `internal/swarm/runner`, `internal/swarm/reconcile`, `internal/swarm/brief`, `internal/commands/swarm.go`); the contract is [docs/swarm.md](swarm.md) section 11, whose As built paragraph says what a pass leaves out (the board and the budget) |
| Legacy | none |
| Usage | `daisy swarm apply [--once]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); with `--once`, one pass, its runs waited for and collected (the maintainer's smokes); without, the loop the unit `init` prints runs: a pass every `interval` whatever runs, each run collected by the first pass after it ends, until SIGINT or SIGTERM; any other argument is usage on stderr, exit 2 |
| Exit codes | 0 the pass or passes ran (a member's failure is on the forge, not in the exit code; a pause on stdout); 1 no swarm found, `swarm.json` absent or invalid at the start, the token or an `env` credential unset (a slot without its login is not checked at the start: it is the swarm fault *slot refused* of the run that takes the slot, `fault: slot <provider>/<j>: no login; run '<login>'`, the login line `CODEX_HOME=<slot> codex login -c cli_auth_credentials_store=file` for `openai` and `GROK_HOME=<slot> grok login --device-auth` for `xai`, docs/swarm.md section 8), `docker ps` failing at the start (`Error: swarm: docker: <message>`), the lock held (`Error: swarm apply is already running for <owner>/<repo> (<holder line>); lock: <state>/lock`), `EnsureLabels` failing (a refused rename or create among the causes), and with `--once` a forge, git or `docker ps` error during the pass (`Error: swarm: …`; without `--once` it is printed and the next pass runs; a worker run's fetch from its clone failing is one the first time, and the second time in a row that run's failure *fetch failed*, its issue blocked (an abandoned run's: its commits not kept, the run collected); a run's clone failing, or a resource no longer passing its checks, at a launch is not one but that run's swarm fault, *clone failed* or *resource refused*, docs/swarm.md section 7.7); 130 a second signal, during the pass the first ended or while the loop collects its running members; 2 usage |
| Stdout | `✓ Labels: 16 present (<c> created, <r> renamed)` at the start when `EnsureLabels` made a label; `#<n> <role> run <k>: launched` (the owner's batch: `owner run <k>: launched`), then for a worker, and a reviewer launched at `swarm:review-0-ready`, `#<n> <role> run <k>: <stage> → <in-progress stage>`; at collection, `#<n> <role> run <k>: ` then `posted comment`, `posted review (<verdict>)`, `check started`, `check exited <code>` or `check timeout`, `pushed swarm/<n>-<s> <short sha>`, `created pull <name>#<m>`, `<from> → <to>`, `blocked`, `failed (<reason>)`, `fault (<reason>)`, `pushed notes <short sha>` (with `notes`, docs/swarm.md section 7.6, The notes); the batch's `owner run <k>: proposed #<n> <title>` or `owner run <k>: <reason>; batch held (<state>/owner-held)`; `#<n> pull <name>#<m>: <label>` (in step (5), and after the flag for an issue the pass moved since, docs/swarm.md section 5); the merge step's `#<n> merged pull <name>#<m> (slice <s> of <S>)`, `#<n> swarm:review-2-approved → swarm:plan-1-accepted`, `#<n> closed (last slice merged)` then `#<n> swarm:review-2-approved → swarm:done`, `#<n> merge of pull <name>#<m> refused (<message>)` then `#<n> blocked` (`no head both reviews approve` its message when none is), `#<n> not mergeable: back to the worker` then `#<n> swarm:review-2-approved → swarm:review-3-changes-requested` (T25, the direction naming the files that conflict on the mirror), `#<n> not mergeable a second time at <short sha>: blocked` then `#<n> blocked` (T25's second time on one head), `#<n> merge of pull <name>#<m> refused (<message>); its mergeability is not settled, so the next pass tries again`, `#<n> merge of pull <name>#<m> refused (<message>); its head merges into <base> on the mirror, so the forge is taken to be recomputing and the next pass tries again`; a routine job's gate passed, `#<n> <stage> → <stage> (auto)` and `#<n> auto note posted` (the auto note, docs/swarm.md section 5); the maintainer's flag's `#<n> gated`, `#<n> gate note posted` (the gate note, at each gating event: the flag added, or, the flag on, a new stage or a new plan, docs/swarm.md section 5) and `#<n> gated cleared`; `Paused since <time>[: <reason>] (<state>/paused): launching nothing`; the milestones' `milestone created: <title>` and `#<n> milestone: <title>`; in a swarm repository's clone, once its drift is gone, `✓ Drift cleared: swarm.json and prompts/ are <owner>/<repo>'s <branch> at <short sha>` (docs/swarm.md section 11, Drift: the drift lines on stderr at the first pass and then only when the set of drifting paths changes); `Pass <HH:MM:SS>: launched <a>, collected <c>, running <r>` |
| Stderr | `⚠️  <state>/env <problem>`; `⚠ <swarm>/.env.sh: …` and the drift warnings (docs/swarm.md sections 3 and 11); `⚠ #<n>: Blocked by #<m> names no issue; not held` (the forge holds no `#<m>`: it holds nothing, each pass while the comment stands), `⚠ #<n>: carries <label> and <label>; skipped`, `⚠ #<n>: <label> but its current pull request is not open; skipped`, `⚠ #<n> closed during the run; nothing posted`, `⚠ #<n>: relabelled to <label> during the run; not relabelled` (`during the merge step` for the merge step's), `⚠ #<n>: pull <name>#<m>'s head <short sha> not merged on the mirror (<error>); the forge's report stands`, `⚠ #<n> worker run <k>: commit <short sha> has no Co-Authored-By: <co_author>`, `⚠ #<n> worker run <k>: abandoned; commits not kept (<reason>)`; with `notes`, `⚠ #<n> <role> run <k>: notes conflict with <owner>/<repo>'s head; pushed to swarm/notes-<n>-<role>-<k>` and `⚠ #<n> <role> run <k>: notes not pushed (<reason>); kept in <run>/notes`; the flag's `⚠ #<n>: gated: <error>`, `⚠ #<n>: gate note: <error>`, `⚠ #<n>: class: <error>`, `⚠ #<n>: auto: <error>` and `⚠ gated: <error>`, the copy's `⚠ #<n>: pull request label: <error>` and the milestones' warnings (docs/swarm.md section 11), never an exit code; errors |
| Ops | Real |
| Writes | `<state>/lock`, `<state>/paused` (two swarm faults in a row), `<state>/owner-held`, `<state>/auto` (a line per gate passed for a routine job), `<state>/git/`, `<state>/references/` (revision 6), with `notes` `<state>/notes/` (the host's clone of the swarm repository), `<state>/runs/`, a slot's lock and the clearing of the slot; on the forge, issues, comments, reviews, pull requests, label changes, pushed `swarm/<n>-<s>` branches, with `notes` the research notes' commits pushed to the swarm repository's `notes/` directory (its default branch; on a conflict to `swarm/notes-<n>-<role>-<k>`), the sixteen labels when one is missing (an old stage name renamed in place), `swarm:class-architecture` on each of the batch's proposals but those it classes routine, a routine job's `swarm:proposal-posted` and `swarm:plan-0-posted` relabelled with an auto note, `swarm:gated` set and cleared on open issues, a gate note on each issue the flag is added to, `swarm:blocked` set on a failed or blocked run's issue and on a refused merge's, a milestone per `active` brief's milestone and an issue's milestone where it has none, and with `gates.merge` false merges and closed issues, each closed issue then `swarm:done` |
| Commits | none |
| Cases | none; `TestSwarmApplyLifecycle`, `TestSwarmApplyLoopCollectsALaterPass`, `TestSwarmApplyCollectsARunAnotherApplyStarted`, `TestSwarmApplyGatesOff`, `TestSwarmApplyMilestones`, `TestSwarmApplyGatedFlag`, `TestSwarmApplyHolds`, `TestSwarmInProgressTakenBack`, `TestSwarmApplyResearchNotes`, `TestSwarmApplyResearchNotesMerges`, `TestSwarmApplyNotesInTheSwarmRepository`, `TestSwarmApplyOwnNotesNotADirectory`, `TestSwarmApplyOwnNotesNotADirectoryNothingWritten`, `TestSwarmApplyOwnNotesBecomeALink`, `TestSwarmSecondReviewerGoesHome`, `TestSwarmApplyContinuedPullRepositoryRemoved`, `TestSwarmApplyRoutineJob`, `TestSwarmBatchCountsRoutineProposals`, the pass's and the gates' (`TestSwarmApplyGateNoteOnEachGating`, `TestSwarmApplyBlockedGateNote`, `TestSwarmApplyMergeRefusedNoHead`, `TestSwarmFaultPauseOrder`, `TestSwarmApplyLoopTwoSignalsInAPass`, `TestSwarmApplyLoopSecondSignalAfterAPass`, `TestSwarmApplyLoopLeavesNoWatcher`, `TestSwarmApplyLoopSignalSkipsTheMergeStep`), the rounds' and the milestones' reads (`TestSwarmApplyDescriptionIsNoAnswer`, `TestSwarmApplyLoopMilestonesReadClosedOnce`), the collection's (`TestSwarmAbandonedWorkerKeepsItsCommits`, `TestSwarmOrphanEndedByTheExitMark`, `TestSwarmCheckRestartedAfterItsApplyDied` and the others of `swarm_collection_test.go`), the merge step's `TestSwarmMergeStepUnsettled`, `TestSwarmMergeStepRecomputing`, `TestSwarmMergeStepMirrorCannotTell` and `TestSwarmMergeStepSentBackTwice`, and the other `TestSwarmApply` tests, and the `internal/swarm` tests |

### daisy swarm pause

| Field | Value |
|---|---|
| Status | as built (Step 6 of the swarm build; `<n>` at Step 15; finding the swarm at Step 2 of revision 6's plan): Go (`internal/commands/swarm.go`, `internal/commands/swarm_maintainer.go`); the contract is [docs/swarm.md](swarm.md) section 11 |
| Legacy | none |
| Usage | `daisy swarm pause [<n>]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); without `<n>`, the whole swarm; with it, as the maintainer, issue `<n>` alone (the maintainer's verbs' start, below); a second argument is usage on stderr, exit 2 |
| Exit codes | 0 paused or already paused; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, the write failing, and with `<n>` the maintainer's verbs' refusals and a forge failure; 2 usage |
| Stdout | `Paused: no new member will launch (<state>/paused); running members finish and are posted`, or `Already paused since <time>[: <reason>]`; with `<n>`, `Maintainer <login> on <owner>/<repo>`, then `#<n> paused` or `#<n> already paused` |
| Ops | Real |
| Writes | without `<n>`, `<state>/paused` (and `<state>` when absent), no forge call, no secret read; with it, on the forge, `Relabel(n, "", "swarm:paused")` unless it carries it |
| Commits | none |
| Cases | none; `TestSwarmPauseAndResume`, `TestSwarmMaintainerHolds`, `TestSwarmApplyHolds` |

### daisy swarm resume

| Field | Value |
|---|---|
| Status | as built (Step 6 of the swarm build; `<n>` at Step 15; finding the swarm at Step 2 of revision 6's plan): Go (`internal/commands/swarm.go`, `internal/commands/swarm_maintainer.go`); the contract is [docs/swarm.md](swarm.md) section 11 |
| Legacy | none |
| Usage | `daisy swarm resume [<n>]`: in the swarm repository's clone (docs/swarm.md section 11, Finding the swarm); without `<n>`, the whole swarm; with it, as the maintainer, issue `<n>` alone; a second argument is usage on stderr, exit 2 |
| Exit codes | 0; 1 no swarm found, `swarm.json` absent or unparsable or its forge fields invalid, a removal failing, and with `<n>` the maintainer's verbs' refusals and a forge failure; 2 usage |
| Stdout | `Resumed: members launch at the next pass` when `<state>/paused` was present, `Owner batch released` when `<state>/owner-held` was; `Not paused` when neither was; with `<n>`, `Maintainer <login> on <owner>/<repo>`, then `#<n> paused cleared` or `#<n> not paused` |
| Ops | Real |
| Writes | without `<n>`, removes `<state>/paused`, whoever wrote it, and `<state>/owner-held`, no forge call, no secret read; with it, on the forge, `Relabel(n, "swarm:paused", "")` when it carries it |
| Commits | none |
| Cases | none; `TestSwarmPauseAndResume`, `TestSwarmApplyBatchTrigger`, `TestSwarmMaintainerHolds` |

### The maintainer's verbs

`swarm triage`, `swarm accept`, `swarm class`, `swarm approve`, `swarm
merge`, `swarm unblock`, `swarm block` and `swarm close`, and `swarm pause
<n>` and `swarm resume <n>`, are the maintainer's writes on the forge
(docs/swarm.md section 11, built at Step 10 of the swarm build, `block`
and the issue's `pause` and `resume` at Step 15, `class` at Step 20), each
under the maintainer's own token, `DAISY_MAINTAINER_TOKEN`, resolved as
`check-secrets` resolves a variable (`<state>/env`, read never run, else
the environment); `apply` and `status` never read it, and `init` only for
its writes on the swarm repository (docs/swarm.md section 8, rule 5). Each verb
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
a file, launches a member or runs a process (git aside, for the
validation), or commits; a refusal writes nothing. A relabel, or
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

### daisy swarm triage

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build; finding the swarm and `<state>/env` at Step 2 of revision 6's plan): Go (`internal/commands/swarm_maintainer.go`); the contract is [docs/swarm.md](swarm.md) section 11 |
| Usage | `daisy swarm triage <n>`: G1, T3, for an issue at `swarm:proposal-posted` |
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
| Usage | `daisy swarm accept <n>`: G2, T7, for an issue at `swarm:plan-0-posted` |
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
| Usage | `daisy swarm class <n> routine\|architecture`: the job class of issue `<n>`, at any stage (docs/swarm.md section 5): `architecture` sets `swarm:class-architecture`, `routine` clears it, and either then posts the class statement `Class: <class>`, which Daisy reads and no member answers; an issue of that class already is left as it is, but for an architecture job without `swarm:class-architecture` (no class statement, or the maintainer's own issue), which is given the flag alone |
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
| Usage | `daisy swarm approve <n>`: a formal approving review of issue `<n>`'s current pull request, open, at its head, at any stage |
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
| Usage | `daisy swarm merge <n>`: G3 for an issue at `swarm:review-2-approved`: its current pull request merged at the head both the swarm's newest reviews approve, as the merge step merges (`do: merge`, `head_commit_id`), or found merged; then T13 when the accepted plan declares a slice after it, else the issue closed and labelled `swarm:done`; a merge the forge refuses of a pull request it reports not mergeable (`base` moved since the branch took it in) is T25 instead: the direction to merge `base` posted on the pull request (docs/swarm.md section 12) and the issue relabelled `swarm:review-3-changes-requested`, nothing merged; the verb reads the forge alone, so its direction names no file; a head sent back once already, the thread holding a direction on it before its last comment, is refused |
| Exit codes | 0 merged (or found merged) and relabelled or closed, or sent back to the worker (T25); 1 the maintainer's verbs' refusals, no swarm pull request, one closed unmerged, no head both reviews approve, the forge refusing the merge of a pull request it reports mergeable (`Error: swarm: #<n>: merge of pull <name>#<m> at <short sha> refused: <message>; nothing changed`: no note and no block, unlike the merge step) or whose mergeability it has not computed (`Error: swarm: #<n>: merge of pull <name>#<m> at <short sha> refused: <message>; the forge has not yet computed whether it merges, so run merge again shortly; nothing changed`), a head sent back once already that the forge still reports not mergeable (`Error: swarm: #<n>: pull <name>#<m> was sent back to merge <base> at <short sha> once already, was approved again at that head, and the forge still reports it not mergeable; merge <base> into swarm/<n>-<s>, or relabel the issue; nothing changed`), another forge failure; 2 usage |
| Stdout | `Maintainer <login> on <owner>/<repo>`; `#<n> merged pull <name>#<m> (slice <s> of <S>)` or `#<n> pull <name>#<m> already merged (slice <s> of <S>)`; then `#<n> swarm:review-2-approved → swarm:plan-1-accepted` or `#<n> closed (last slice merged)` and `#<n> swarm:review-2-approved → swarm:done`; `#<n> <flag> cleared` for each flag the close cleared, or `#<n> gated cleared` after the relabel (not for an issue still blocked); sent back, `#<n> not mergeable: back to the worker`, `#<n> commented on pull <name>#<m>` (`#<n> comment already posted on pull <name>#<m>` when the direction is the thread's newest already, a re-run), `#<n> swarm:review-2-approved → swarm:review-3-changes-requested` and `#<n> gated cleared` when it carried the flag |
| Ops | Real |
| Writes | on the forge, `Merge(m, <approved head>)`; refused for a pull request not mergeable, `Comment(m, <the direction>)`, `Relabel(n, "swarm:review-2-approved", "swarm:review-3-changes-requested")` and the gated flag's removal; else `Relabel(n, "swarm:review-2-approved", "swarm:plan-1-accepted")` and the gated flag's removal when it carried it, or `CloseIssue(n)`, `Relabel(n, "swarm:review-2-approved", "swarm:done")` and the removal of each flag it carried |
| Commits | none |
| Cases | none; `TestSwarmMaintainerVerbs`, `TestSwarmMaintainerCloseUnblockMerged`, `TestSwarmMaintainerFlagAndTerminals`, `TestSwarmMaintainerRefusals`, `TestSwarmMaintainerMergeNotMergeable`, `TestSwarmMaintainerMergeBackRunAgain`, `TestSwarmMaintainerMergeSentBackTwice`, `TestSwarmFlagClearedOnEveryTransition` |

### daisy swarm unblock

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build; `swarm:blocked` a flag at Step 15), as `swarm triage` |
| Usage | `daisy swarm unblock <n> <comment...>`: G4, T18, for an issue carrying `swarm:blocked` beside its stage, which the block kept: the comment, then the flag cleared; refused at the round limit's `swarm:review-1-in-progress` (or `swarm:review-0-ready`, T21's write refused), where no role runs; the words after `<n>` are one comment, saying what changed |
| Exit codes | 0 commented and unblocked; 1 the maintainer's verbs' refusals, the issue not blocked (`Error: swarm: #<n> is not blocked; nothing changed`), blocked at the round limit (`Error: swarm: #<n>: blocked at the review round limit, …; nothing changed`, naming the relabels `swarm:review-3-changes-requested` and `swarm:plan-0-posted` to make first), a forge failure; 2 usage (no comment, or blank words only, among them) |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<n> commented` (or `#<n> commented on pull <name>#<m>`, at `swarm:review-3-changes-requested` with the pull request open; `#<n> comment already posted`, `… on pull <name>#<m>`, when it is the thread's newest already, a re-run), `#<n> blocked cleared`, then `#<n> gated cleared` (a blocked issue carries the flag after a pass) unless the stage the block kept is a gate that is on and holds its class, where the flag stays |
| Stderr | `⚠ <login> is not in trusted: no member reads your comment (docs/swarm.md section 6)`, never an exit code |
| Ops | Real |
| Writes | on the forge, `Comment(n, <comment>)` (on the current pull request instead, when it is open and the stage is `swarm:review-3-changes-requested`; none when it is that thread's newest already), then `Relabel(n, "swarm:blocked", "")` and the gated flag's removal when it carried it and the stage is not gated |
| Commits | none |
| Cases | none; `TestSwarmMaintainerCloseUnblockMerged`, `TestSwarmUnblockReviewedRunsTheWorker`, `TestSwarmMaintainerRefusals`, `TestSwarmMaintainerBlankComment`, `TestSwarmMaintainerRunAgainPostsOnce`, `TestSwarmMaintainerKeepsTheFlagWhereGated`, `TestSwarmApplyGateNoteAfterABlockLifted`, `TestSwarmMaintainerStep29Refusals` |

### daisy swarm block

| Field | Value |
|---|---|
| Status | as built (Step 15 of the swarm build), as `swarm triage` |
| Usage | `daisy swarm block <n> --by <m>`: hold issue `<n>`'s job until issue `<m>` of `forge.repo`, open, is closed: `#<m>` given `swarm:blocking`, then the maintainer's comment `Blocked by #<m>` on `#<n>`, which the role rule reads while it is `#<n>`'s newest maintainer comment (docs/swarm.md section 6, rule 1); `--by <m>` may also lead, as the cli reads a value flag |
| Exit codes | 0 flagged and commented; 1 the maintainer's verbs' refusals for `#<n>`, `#<m>` the same issue (`Error: swarm: #<n> cannot block itself; nothing changed`), closed or a pull request (`Error: swarm: #<m> is not an open issue; nothing changed`), a forge failure; 2 usage, `--by` missing or `<m>` not an issue number among it |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<m> blocking` (none when it carries the flag already), `#<n> commented` (`#<n> comment already posted` when it is `#<n>`'s newest comment already) |
| Stderr | `⚠ <login> is not in trusted: the role rule does not read your Blocked by #<m>, so it holds nothing (docs/swarm.md section 6)`, never an exit code |
| Ops | Real |
| Writes | on the forge, `Relabel(m, "", "swarm:blocking")` unless it carries it, then `Comment(n, "Blocked by #<m>")` |
| Commits | none |
| Cases | none; `TestSwarmMaintainerHolds`, `TestSwarmMaintainerRefusals`, `TestSwarmApplyHolds`, `TestSwarmMaintainerStep29Refusals` |

### daisy swarm close

| Field | Value |
|---|---|
| Status | as built (Step 10 of the swarm build), as `swarm triage` |
| Usage | `daisy swarm close <n> [comment...]`: T19, at any stage, the rejection (a job done is closed by `swarm merge`, as `swarm:done`); the words after `<n>`, when given, are one comment posted first |
| Exit codes | 0 closed and relabelled; 1 the maintainer's verbs' refusals, the terminal refused after the close, a flag's removal refused after the terminal, a forge failure; 2 usage |
| Stdout | `Maintainer <login> on <owner>/<repo>`, `#<n> commented` when a comment was given (`#<n> comment already posted` when it is the issue's newest already, a re-run after a refused close), `#<n> closed`, `#<n> <stage> → swarm:rejected`, and `#<n> <flag> cleared` for each flag it carried |
| Ops | Real |
| Writes | on the forge, `Comment(n, <comment>)` when given, then `CloseIssue(n)`, `Relabel(n, <stage>, "swarm:rejected")` and the removal of each flag it carried; the pull requests are left as they are |
| Commits | none |
| Cases | none; `TestSwarmMaintainerCloseUnblockMerged`, `TestSwarmMaintainerFlagAndTerminals`, `TestSwarmMaintainerRefusals`, `TestSwarmMaintainerBlankComment`, `TestSwarmMaintainerRunAgainPostsOnce`, `TestSwarmMaintainerFlagRemovalRefused` |

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
| Stdout | one `✓ <check>` line per pass: `DAISY_ROOT:`, `DAISY_HOME:`, `Git repository: <hash>`, `today.md format valid`, `Component: <command>` (`Component:` [37]) per hook in table order, `Publication hygiene: no identifying terms outside the homes`, `Feedback: N entries (last optimized: …\|never optimized)`, blank, `System healthy`; cached: `Checks cached — valid for Ns (--force to re-run)`. The marks carry the bash's colour codes, whatever the stream is |
| Stderr | `✗ <error>` and `⚠ <warning>` lines. Hygiene (Check 6c): `Publication hygiene: 'zebrahome'` [35, 37] / `Publication hygiene: 'Zebra Quillfeather'` [75] ` appears in publishable file(s) — …` then each file indented (`leak.md` [35, 75], `public-leak.md` [36]); terms are every registered home's name (placeholders `work`, `personal`, `example`, `testhome` exempt), the git identity of each usable home's repository and of the root's, `$USER` and `$HOME/`, each once without case; the surface is every path the root tracks (`git ls-files --cached`, wherever it lies) and every untracked, unignored one (`--others --exclude-standard`) but those under a registered home inside the root (`env.RootExclusions`) — an ignored file is never reported (`ignored-leak.md` [36] absent) and an untracked, unignored one is [36]; a regular file is read for its content, a symlink for its link text (what git publishes) and, pointing at a file, that file's content (the bash read no link text: Step 13's Deferred item); every path's name; and every file or link entry of the index (`ls-files -s`, any stage; gitlinks skipped) whose object the worktree no longer matches is read by object id (`cat-file blob`), so a staged version — assume-unchanged, skip-worktree, conflicted or not — is scanned as the next commit would publish it (the bash read neither names nor the index); a term matches without case, whole-word at each edge that is a word character, so `$HOME/` is found inside a longer path (`grep -w` required a non-word character after the `/` and missed `$HOME/src`). A failing hook: `Component: <command> failed health check` and its message indented. Warnings: `<HOME>/.claude/skills/testing is a copy of the root's skill 'testing' that an earlier 'daisy install' left; skills are installed in each workspace by 'daisy init' now — run 'daisy install --update' to remove it` [34] (Check 4c: what `install` would remove, `skill.RootCopies`; never a skill of the user's own [34], an edited copy or `synced`, where Claude Code's own skill sync writes [31], `TestSkillDriftNamesRootCopies`), `Installed skill '<name>' has no matching source in skills/` (4b, in `.claude/skills` and `.agents/skills` alike, naming the tree; never for `.agents/skills/daisy`, which init generates, `TestSkillDriftCoversBothTrees`), `journal.md has N day-blocks (expected ≤ 5)` and `<window file> is missing TOC/Summary front matter` (5b), AGENTS.md's Command Reference missing or differing from `daisy help --markdown` (6b, the drift check; the bash checked a script table), `Feedback has N entries — consider running 'daisy optimize'` (7, at 20 or more), `Override of prompts/<name>.md is behind the root (root changed since the copy)` and how to fold it in (8, per `feedback/.overrides` line whose full override's root file hash differs; silent when unchanged, never for a layered override) [88], `<ws>/.daisy/AGENTS.md is not what a build makes now (the root's prompts or the home's overrides changed since) — run 'daisy init'` or `<ws>/.daisy/prompts is not the merge … — run 'daisy init'` (8b: wherever an override is involved — in the home now, or in the workspace's build — the workspace's build compared with one made now, the `Built:` line aside — the root moved on, an override added, changed in form or removed elsewhere), `<ws>/.claude/skills/<name> is not the home's override of <root skill dir> as it is now — run 'daisy init <identity>'`, and the same for `<ws>/.agents/skills/<name>` (each tree's copy compared, `TestHealthCheckComparesBothSkillCopies`; every file init would install compared: the root's directory, the home's files, the SKILL.md as merged), `The workspace's AGENTS.md cannot be rebuilt: …` (a malformed layer, said the moment it appears; `TestHealthcheckSaysAStaleMaterialisedPrompt`, `TestHealthcheckSaysAMalformedLayer`); `✗ N issue(s) found` last, each issue counted once (the bash counted a failing component and a missing directory twice). The pre-split warning (3b) is gone: the bound home inside the root repository is now Check 3's error [71], and another registered home placed under the root is no warning (`zebranest` [74] absent) |
| Ops | Real (the stamp) |
| Writes | `$DAISY_HOME/.healthcheck.ttl` only |
| Commits | none |
| Cases | 00, 34, 35, 36, 37, 74, 75 |

### daisy check-secrets

| Field | Value |
|---|---|
| Status | as built (Step 24): Go; `.env.sh` is parsed (`env.ParseShellEnv`), never run (decided, D30); the swarm's variables at Step 2 of revision 6's plan (docs/swarm.md section 14, item 105) |
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
