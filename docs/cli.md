# The `daisy` CLI — Contract

This is the contract the Go binary must meet: the command tree agreed in the
plan, and under it one entry per command with its arguments, flags, exit
codes, the stdout and stderr the bash acceptance suite asserts, the `Ops`
wrapper it runs under, the files it may write and the commits it makes.

**Status of each entry.** `as built` describes `daisy.sh` and
`daisy/scripts/*.sh` as the suite pins them today. `agreed (Step N)` is a
change the plan's Architecture or Decisions text has decided; the named
step makes it. `decided (D30, Step N)` is a point Architecture left open
that this document settles, recorded in the plan's D30 for review; the
named step makes it. Where the bash has no
explicit exit code or message for a situation the entry says *unspecified;
Go may choose* rather than inventing one.

**Quoted strings.** A string in backticks followed by case numbers in square
brackets — `` `No task found matching:` [04] `` — is asserted verbatim by
those cases in `tests/cases/`; the suite is the authority, this
document quotes it. Strings without a case number are what the bash prints
today and are not asserted; Go may reword them.

**Mapping rule.** Every `daisy.sh` command names exactly one entry (its
`Legacy` row); every case file names exactly one entry (its `Cases` row). A
case that runs several commands belongs to the command its name pins —
case 72 (`done`, `log`, `new-day`, root untouched) is `commit`'s; cases
76–79 (`init`, then `status`, `files`, `clean`) are `init`'s — and the other
entries cross-reference it in prose.

## Command tree

Agreed text of `daisy --help` (Architecture, "Command tree"; agreed in
planning round 2 with the `optimize` and `swarm` lines amended):

```
Usage: daisy <command> [args...]

Workspace
  init <identity> [path]            Bind a workspace to an identity and its home
  clean [-f]                        Remove Daisy from this workspace
  status                            Home, identity, projects, today's counts, active plan
  files                             Resolved real paths for every home file
  list                              Active prompts and installed skills
  help [<command>|<skill>]          This help, or detail for one command or skill

Day
  new-day | new-week                Start a new day / week
  log <message...>                  Add a log entry to today.md
  done <pattern>                    Mark a task complete
  tasks [--all] [--todo|--done] [pattern]
  projects [--archived]
  rotate                            Rotate journal.md into window files

Plans
  plan new [--spec] <description> [+project]
  plan pickup <path>
  plan archive

Homes and identities
  home new <name> [path]            Create a home; prompts for identity and location
  home add <name> <path>            Register an existing home
  home list | home remove <name>
  identity list                     Identities across registered homes

Prompts and skills
  prompt build [<home>]             Rebuild AGENTS.md
  install [--update]                ~/bin/daisy, root skills, permissions
  feedback [--workflow <n>] <text>
  optimize [--workflow <n>]         Briefing: pending entries per target, current rules, rewrite flags
  optimize apply --workflow <n> (--rules|--rewrite) <file> [--yes]
  eval [<case>] [--record]

Swarm
  swarm init [--defaults]           Wizard: forge, repository, personas, image; writes swarm/, creates labels
  swarm apply [--once]              Reconcile the forge: run members whose turn it is, post what they wrote
  swarm status                      Issues by stage; running members
  swarm pause | swarm resume        Stop launching new members / allow it again

System
  commit [--root] <message>         Commit the home repo (or the root repo)
  healthcheck [--force]
  check-secrets
  test [filter]
```

## Renames

| Legacy (`daisy.sh` today) | Agreed | Step |
|---|---|---|
| `daisy build [home]` | `daisy prompt build [<home>]` | 20 |
| `daisy plan-new [--spec] <desc> [+project]` | `daisy plan new [--spec] <description> [+project]` | 22 |
| `daisy plan-pickup <path>` | `daisy plan pickup <path>` | 22 |
| `daisy plan-archive` | `daisy plan archive` | 22 |
| `daisy commit --home <message>` | `daisy commit <message>` | 24 |
| `daisy commit --not-home <message>` | `daisy commit --root <message>` | 24 |
| `daisy init --new <home>` | removed: `daisy home new <home>`, then `daisy init <home>` | 21 |
| `daisy optimize [--workflow <n>] [--dry-run] [--force]`, `--list` | `daisy optimize [--workflow <n>]` (briefing) and `daisy optimize apply …` | 23 |
| — | `daisy identity list` (new) | 21 |
| — | `daisy swarm …` (new) | 26–30 |

Every rename is applied to `prompts/`, `templates/`, `docs/` and `AGENTS.md`
in the step that ports the command; there are no aliases. Everything not in
this table keeps its name and every flag the suite asserts.

## Common behaviour

**Exit codes.** `Run` returns 0 on success, 1 on failure, 2 on a usage
error (unknown command, unknown flag, missing or surplus positional
argument), with the command's usage on stderr. The bash exits 1 for usage
errors too; no case asserts a usage exit code, so 2 is free to take
*(agreed, Step 17)*. A value the command refuses — an unknown identity, an
invalid name, an ambiguous pattern, no match — is a failure, exit 1, as the
cases pin it (`home add bad=name` exits 1 in case 68; `not a valid identity
name` [77] exits 1) *(decided, D30: Architecture says only "exit 2 and
usage on bad args")*.

**Unknown command.** Bash: `Unknown command: <name>`, a blank line and the
help text, all on stderr, exit 1. Same text, exit 2 *(decided, D30, Step
17)*. Not asserted.

**`--help` / `-h`.** As the sole argument of any command prints that
command's usage on stdout and exits 0 without running it; the bash reads it
from the script's header comment. `daisy`, `daisy help`, `daisy --help`,
`daisy -h` print the help: as built the flat `Commands:` list of `show_help`,
the agreed tree above from Step 17. *The four forms and the sole-argument
rule are as built and kept (decided, D30: Architecture does not mention
them).*

**Workspace.** Most commands run inside a workspace: the nearest ancestor of
the working directory holding a `.daisy/` directory that is not the Daisy
root. Outside one the bash dispatcher prints `Not in a Daisy workspace` [78]
`(no .daisy/ directory found)` and `Run 'daisy init <identity>' to initialize
this workspace.` on stderr and exits 1. A directory whose binding is broken
is refused rather than walked past, exit 1, stderr naming the fault and
`daisy init` [78] as the repair: `.daisy` `not a directory` [78],
`symlink` [78], a leftover `.daisy.previous.4242` [78] from an interrupted
init, or `.daisy/` with no `home` file. Commands that need no workspace:
`install`, `init`, `home`, `help` (tree or command), `prompt build <home>`.
`test` and `check-secrets` are dispatched through the workspace check today
although their scripts read only the root (`test`) or the workspace's
`.env.sh` (`check-secrets`); Go keeps the requirement for `check-secrets`
(it reads a workspace file) and drops it for `test` *(decided, D30, Step 24)*.

**Home resolution.** `.daisy/home` names the home; the name is looked up in
the registry `$DAISY_CONFIG/homes` (default `~/.config/daisy/homes`). Errors,
exit 1, stderr: a name with no registry line — `not registered` [68], and
the fix `daisy home add second` [68] (the name substituted); a registry
path with a problem — `Error: Home '<name>' is registered at <path>, which
<problem>`, the problem being `is ` `not an absolute path` [68], `does not
exist`, `is the Daisy root` [68] `; the root ` `cannot be a home` [68] or
`contains the Daisy root` [68] `; the root cannot be inside a home`. With no workspace, `DAISY_DEFAULT_HOME` is a fallback
that warns on stderr (`⚠️  No .daisy/home found; falling back to
$DAISY_DEFAULT_HOME (<name>)`). The identity is `.daisy/identity`, else the
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

**Environment read.** `DAISY_ROOT` (required; the bash dispatcher exits 1
with `Error: DAISY_ROOT not set.` without it), `DAISY_CONFIG`,
`DAISY_LOCAL`, `DAISY_DEFAULT_HOME`, `HOME`, `USER`, `DAISY_HEALTHCHECK_TTL`,
`DAISY_HEALTHCHECK_PASSED` (see `healthcheck`), `SHELL` (`install` picks the
rc file by it). As built only, gone at Step 23 with the edit branch and the
API call: `EDITOR`, `TMPDIR`, `ANTHROPIC_API_KEY`, `DAISY_OPTIMIZE_MODEL`,
`DAISY_OPTIMIZE_RESPONSE_FILE` (the suite's LLM stub). Nothing else.

**`--healthcheck` hook.** The scripts listed in `healthcheck.sh`'s
`HEALTHCHECK_SCRIPTS` (`new-day`, `new-week`, `done`, `log`, `home`,
`feedback`, `optimize`, `eval`, `files`, `list`, `plan-pickup`, `projects`,
`rotate`, `test`, `tasks`) accept `--healthcheck` as the sole argument:
check their own preconditions, print nothing on success, exit 0; else a
one-line `Error: …` on stderr, exit 1. Two cases pin it from outside
(`done --healthcheck` [18], `tasks --healthcheck` [30]). Go runs the same
checks inside `healthcheck` and keeps the flag on those two commands at
least, so 18 and 30 pass unchanged *(decided, D30, Step 18; the flag is
hidden from the help text)*.

**`Ops` wrappers.** `ReadOnly` for `status`, `files`, `list`, `projects`,
`tasks`, `help` and the briefing form of `optimize` (agreed, Architecture),
and for `check-secrets`, `home list` and `identity list` (decided, D30);
`Real` for everything else, `eval` included (one wrapper per command;
`--record` writes) and `healthcheck`, because a pass writes the TTL stamp.

**Commits.** A command that commits does so exactly once, in the home's
repository, as that repository's own git config — never in the root (case
72: `done`, `log` and `new-day` add one home commit each and the root
repository's commit count and `git status --porcelain` are unchanged).
As built, the committing commands are `done`, `log`, `new-day`, `new-week`,
`plan pickup`, `plan archive`, `commit` and `home new` (the new home's
first commit); `plan new`, `feedback`, `eval --record` and `optimize` write
home files without committing them, and the next home commit sweeps them
up. Architecture adds `optimize apply` to the committing commands (Step
23). The root is committed only by `commit --not-home` (`--root` after
Step 24).

## Workspace

### daisy init

| Field | Value |
|---|---|
| Status | as built (`init --new` removed at Step 21) |
| Legacy | `daisy init [--new] <identity> [path]` → `daisy-init.sh` (`exec`) |
| Usage | `daisy init <identity> [path]` |
| Arguments | `<identity>`: a registered home's name (its default identity, `gitconfig`) or the name of `<home>/identities/<identity>` in exactly one registered home. `[path]`: the workspace directory, default `.`. No `<identity>`: usage and the identity list on stderr, exit 1 |
| Exit codes | 0 bound (also when already bound, or re-bound to another identity); 1 refused, nothing written: identity in two homes (`identity 'pelicanmoor' is defined in more than one home: testhome, second` [78], `more than one home` [78]), unknown identity (names it, `nosuchone` [77], and lists the identities that exist, `pelicanmoor` [77]), `not a valid identity name` [77], a registered home that cannot be checked (names it and `does not exist` [78]), a named identity file that is incomplete (names the file `identities/pelicanmoor` [79] and the missing key `email` [79]), a malformed `sshkey` [79], a `linked worktree` [76] (names the main checkout), the directory holding the `Daisy root` [78], `.daisy` `not a directory` [77] or a `symlink` [77], two leftovers and no `.daisy/` (names both, `.daisy.previous.100` [77], `.daisy.previous.200` [77]); 1 when a step fails after writing began: `did not finish` [77, 78] on stderr and everything rolled back (binding, git keys byte for byte, stage removed) |
| Stdout | `Initializing Daisy in: <path>`, `Home: testhome` [77], `Identity: testhome` [76] / `Identity: pelicanmoor` [77]; on a re-bind `Already initialized` [77] `with identity: <identity>` (`Already initialized with identity: pelicanmoor` [77]) or the switch `pelicanmoor → testhome` [77]; the ✓ lines for each artifact; `Bound .daisy/` [79] `to home: <home>`; a cleared previous identity names its keys (`core.sshCommand` [79]); `Done. Daisy is ready in this workspace.` |
| Stderr | warnings, exit still 0: `no complete default identity` [76] (the home's `gitconfig` is template or incomplete; no identity applied), an sshkey that is `not found` [79] (names it, `no-such-key` [79]), a reclaimed leftover `.daisy.previous.12345` [77], `No readable .daisy/gitconfig record` [79] of the previous identity, an aside that `could not be removed` [79]; errors as above |
| Ops | Real |
| Writes | `<ws>/.daisy/` as a whole (staged beside it, swapped in by rename): `home`, `identity` (non-default only), `gitconfig` (the applied triple), the six links `tasks today.md journal.md projects plans feedback.md` into the home, `AGENTS.md`; `<ws>/.cursor/rules/daisy.mdc daisy-logging.mdc daisy-plan.mdc` (obsolete `daisy.md`, `daisy-logging.md` removed, case 32), `<ws>/.claude/commands/daisy.md`, `<ws>/.claude/skills/<name>/` and `.cursor/rules/<name>.mdc` for each home skill (stale ones swept, case 33), `<ws>/.claude/settings.local.json` (Daisy's allow rules), `<ws>/.gitignore` and `.cursorignore` entries; `git init` when the directory is not a repository top level; local git config `user.name`, `user.email`, `core.sshCommand` (`ssh -i <key> -o IdentitiesOnly=yes`, only with `sshkey=`) — the previous identity's keys unset first where their values are still its |
| Commits | none |
| Cases | 32, 33, 76, 77, 78, 79 |

Case 76 also pins `init` under an inherited `GIT_DIR` (ignored, exit 0),
`log` under a hook's `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_COUNT` and
`GIT_AUTHOR_*`/`GIT_COMMITTER_*` (ignored; the commit lands in the home),
and that `init` in a subdirectory of a linked worktree is allowed.

### daisy clean

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy clean [-f]` → built in to `daisy.sh` (`cmd_clean`) |
| Usage | `daisy clean [-f]` (`--force` accepted as well; kept) |
| Arguments | without `-f`: prints `This will remove Daisy from: <path>`, `Home:`, `Identity:` and asks `Continue? [y/N]` through `Ops.Prompt`; anything but y/yes prints `Cancelled.` and exits 0 |
| Exit codes | 0 removed (also when nothing matched); 1 outside a workspace (beside the Daisy root, case 78); 1 when a git key `could not unset` [79] — the record is kept for the retry, and the message is not `left as is` [79] |
| Stdout | `Cleared local git identity (user.name, user.email)` [76, 77, 79] naming exactly the keys cleared — `Cleared local git identity (user.name)` [76], `Cleared local git identity (user.email)` [79], `Cleared local git identity (user.name, user.email, core.sshCommand)` [79]; `left as is` [76] when nothing matched; `No readable .daisy/gitconfig record` [76] when there is no record and no identity file resolves; `Removed .daisy/`, `Removed <leftover>` (`.daisy.previous.555` [76]), `Removed Cursor rules`, `Removed Claude command`, `Removed N skill(s)…`, `Cleaned .gitignore`, `Removed empty .gitignore`, `Removed Daisy permissions from .claude/settings.local.json` or `Removed .claude/settings.local.json` (when only Daisy's rules were in it), `Cleaned .cursorignore`, `Removed empty .cursorignore`; `· Local git identity not checked: this workspace is not a repository` / `…this is a linked worktree and its config is the main checkout's` when the keys cannot be checked; `Done. Daisy has been removed from this workspace.` |
| Stderr | `could not unset` [79]; otherwise unspecified |
| Ops | Real |
| Writes | removes everything `init` wrote (above), unsets each of the three git keys when every value it holds is the recorded identity's (the home's identity file stands in for the record for a workspace bound before records existed) or is a `core.sshCommand` in Daisy's exact form (`ssh -i <path> -o IdentitiesOnly=yes`, which nothing but an init writes — taken with or without a record); a hand-set value is the user's and stays; reclaims `.daisy.stage.*` leftovers |
| Commits | none |
| Cases | none of its own; exercised by cases 76/77/79 (`init`'s) and 78 |

### daisy status

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy status` → built in to `daisy.sh` (`cmd_status`) |
| Usage | `daisy status` |
| Exit codes | 0; 1 outside a workspace or when the home does not resolve (`cannot be a home` [68], `not registered` [68]) |
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
| Status | as built |
| Legacy | `daisy files` → `files.sh` |
| Usage | `daisy files` |
| Exit codes | 0 (a project that does not exist is not fatal, case 44); 1 outside a workspace |
| Stdout | the same first line as `status` (`identity: testhome` [76], `identity: pelicanmoor` [77]); sections `Identity:` (the resolved identity file: the home's `gitconfig` [76] or `identities/pelicanmoor` [77], ` (missing)` when absent), `Journal:`, `Tasks:`, `Projects:` (`workspace` row: `+chessgrid` [43] `(from ` `directory name` [43] `)`, `+chessgrid +jobsearch` [43] in file order, `+jobsearch` [42] `(from ` `.daisy/project` [42] `)`, or `(none)` (`none` [44])), `Feedback:`, `Plans:` (with `current plan` when `PLAN.md` is a link), `Prompts:`; each row `  %-12s <real path>`; last `AGENTS.md:    <real path>` (`%-14s`) |
| Stderr | empty when the convention default matches nothing (asserted `""`, case 44); a declared project with no file is named (`typoed-name` [44]); drift warnings |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 42, 43, 44 |

### daisy list

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy list` → `list.sh` |
| Usage | `daisy list [--full]` (`--full` prints each skill's `description` instead of `short_description`; not in the agreed help line, kept as an unasserted flag) |
| Exit codes | 0; 1 outside a workspace |
| Stdout | `Prompts (<home>):` then one `  %-20s <trigger summary or (always loaded)>` per `include.txt` line, `(missing: <path>)` for an absent prompt, `(no include.txt found)`; blank; `Skills:` then one `  %-20s <text>` per installed skill, user-level (`~/.claude/skills`) then workspace (`.claude/skills`), workspace overriding by name; or `(none installed — run: daisy install, then daisy init <home>)` |
| Stderr | unspecified |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | none |

### daisy help

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy help [<command>\|<skill>]` → built in to `daisy.sh` (`show_help`, `show_command_help`, `show_skill_help`) |
| Usage | `daisy help [<command>\|<skill>]`; `daisy`, `daisy --help`, `daisy -h` are `daisy help` |
| Exit codes | 0; 1 for a name that is neither (`Unknown command or skill: <name>` and `Run 'daisy help' for commands, 'daisy list' for skills.` on stderr); the skill form needs a workspace |
| Stdout | no argument: the tree. A command name: its usage (the script header today; the `cli` table in Go), and when a skill of that name also exists a note — the command's help, not the skill's (not `Collide politely` [52]), and the word `skill` [52]: `Note: a skill named '<name>' also exists; the command name takes precedence.` A skill name: `<name>  (<SKILL.md path>)`, blank, the full `description:` (`when to stop making them` [51]), blank, the `## Rules` section (`Measure before cutting` [51]), `Other sections: <H2 titles>` (`Worked example` [51]). The installed copy is preferred over the source |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 51, 52 |

## Day

### daisy new-day

| Field | Value |
|---|---|
| Status | as built; the `optimize --list` nudge at the end goes with `--list` (decided, D30, Step 23) |
| Legacy | `daisy new-day` → `new-day.sh` |
| Usage | `daisy new-day` |
| Exit codes | 0; 1 when `healthcheck` fails first (`Error: System health check failed` on stderr), or when a step fails |
| Stdout | `📦 Archived yesterday to journal.md` (when `today.md` existed), `rotate`'s line, `🗑️  Deleted N cancelled task(s)` (when any), `✅ New day started: <YYYY-MM-DD> <Day>`, `   High priority tasks: N`, `   Next priority tasks: N`, `   Inbox tasks: N`, `   GitHub tasks: N`, then `commit`'s line. Bash then prints `💡 New feedback since last optimize run:` and the pending entries; this block is removed at Step 23 — the New Week workflow runs `daisy optimize` itself |
| Ops | Real |
| Writes | `journal/journal.md` (appends `today.md` after a `---` separator when non-empty), the rotation window files (see `rotate`), `tasks/todo.txt` (drops `z ` lines), `journal/today.md` from `templates/journal-day.md` (`{DATE}`, `{DAY}`, `{TIME}`, `{HIGH_PRIORITY_TASKS}` = `(A)`, `{NEXT_PRIORITY_TASKS}` = `(B)`, `{INBOX_TASKS}` = no priority, `{GITHUB_TASKS}` = any active line containing `@git`/`@github`, whatever its priority, decided before the priority branches; `(C)`/`(D)` lines without such a tag are not carried) |
| Commits | home, once: `New day: <date> <day>` (case 72) |
| Cases | none of its own; case 72 (`commit`'s) runs it |

### daisy new-week

| Field | Value |
|---|---|
| Status | as built; nudge removed as for `new-day` (decided, D30, Step 23) |
| Legacy | `daisy new-week` → `new-week.sh` |
| Usage | `daisy new-week` |
| Exit codes | as `new-day` |
| Stdout | as `new-day`, plus `📦 Archived N completed task(s) to done.txt` (when any) and `✅ New week started: <date> <day>` |
| Ops | Real |
| Writes | as `new-day`, plus `tasks/done.txt` (appends the `x ` lines of `todo.txt`, which are then removed from it); `today.md` from `templates/journal-week.md` |
| Commits | home, once: `New week: <date> <day>` |
| Cases | none |

### daisy log

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy log <message...>` → `log.sh` (the dispatcher joins the arguments with single spaces: `"$*"`) |
| Usage | `daisy log <message...>` |
| Exit codes | 0; 1 with no message (`Error: Log message required`), when `healthcheck` fails first, when `today.md` is missing (`Error: today.md not found`) or has no `#### Log` section (`Error: Log section not found in today.md`) |
| Stdout | `✅ Logged: <HHMM> <message>` then `commit`'s line (`done` suppresses both: not `Logged:` [15]) |
| Ops | Real |
| Writes | `journal/today.md`: inserts `- HHMM message` at the first blank line after the `#### Log` section's first entry, or appends at end of file when none follows. As built the scan does not stop at the next `####` heading: when the Log section's last entry abuts the next heading with no blank line, the entry lands at the first blank line of a later section (healthcheck Check 5 requires the blank line that prevents this) |
| Commits | home, once: `Log: <HHMM> - <first 60 chars>` (case 72; case 71 pins that with the home still inside the root repository the commit holds only the home's file) |
| Cases | none of its own; cases 71/72 (`commit`'s) and 76/78 (`init`'s) run it |

### daisy done

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy done <pattern>` → `done.sh` |
| Usage | `daisy done <pattern>` (one argument) |
| Arguments | `<pattern>`: literal, case-insensitive substring (`.` is not a wildcard [08]; `/` is safe [07]; a leading `-` is a pattern, not a flag — except the sole-argument `--help`/`-h` and `--healthcheck`, which the dispatcher and the hook take first) matched against the active lines of `todo.txt` only (`x `/`z ` lines excluded before matching, cases 05, 06). Missing: `Error: Task pattern required`, exit 1 |
| Exit codes | 0 exactly one match completed [01, 02, 05–08, 10–15]; 1 no match [04, 16], several matches [03], an Inbox-only item that is not in `todo.txt` [09], `healthcheck` failing first, or the journal entry failing after the edits [17] |
| Stdout | success: exactly one non-blank line [15], `✅ Done: <description> (commit <hash>)` — names the task (`Delta target task` [15]) and suppresses the sub-calls' chatter (not `Logged:` [15], `Committed:` [15], `No changes to commit` [15]); nothing with `✅` [16] when nothing completed; no `✅ Done` [17] when journaling failed |
| Stderr | `No task found matching:` [04] `<pattern>`; `Multiple tasks match "<pattern>":` and each candidate on its own `  N) <line>` (`Shared token Alpha` [03], `Shared token Beta` [03]); `⚠️  Task not found in today.md (may not be in active tasks)` when `todo.txt` is updated but no `- [ ] <description>` line exists (`today.md` [11]; exit still 0); on a journal failure `Error: journal entry failed — …` naming whether the edits were committed, then the sub-calls' output (`Log` [17]). Cases 03, 04, 11, 17 assert on stdout and stderr combined, so the stream is not pinned |
| Ops | Real |
| Writes | `tasks/todo.txt`: the one matched line removed and `x <today> <creation date> <description>` appended last, no priority [10]; `journal/today.md`: the first line equal to `- [ ] <description>` becomes `- [x] <description>`, every other byte unchanged [01, 02, 12]; then the log entry `Done: <description>` through `log` |
| Commits | home, exactly once for the whole completion, holding both files [13]; `log` makes it and the trailing `commit` is a no-op [14]; a failed journal entry still leaves the edits committed [17] |
| Cases | 01-18 |

### daisy tasks

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy tasks [--all] [--todo\|--done] [pattern]` → `tasks.sh` |
| Usage | `daisy tasks [--all] [--todo\|--done] [pattern]` |
| Arguments | scope: this workspace's projects (`+tag` at a word boundary) by default, `--all` every project [19, 46, 47]; files: both by default [22], `--todo` = `todo.txt` [20], `--done` = `done.txt` [21]; `[pattern]`: literal, case-insensitive substring [24], a leading `-` is a pattern (`-1 point deduction task` [25]) once the three flags above have been consumed — except the sole-argument `--help`/`-h` and `--healthcheck`; flags before the pattern |
| Exit codes | 0 when at least one line is printed [19–25, 27, 29, 46–48]; 1 when a pattern is given and matches nothing, grep-style, with empty stdout [26] (as built, with no pattern and a scope that matches nothing the bash exits 0 with empty stdout, the pipeline ending in `cat`; unasserted — Go may choose); 1 when none of the selected files exists; a missing `done.txt` alone is not an error [27] |
| Stdout | the matching lines verbatim and nothing else — no header (`Active projects` [29]), no decoration [29]; every line when no pattern is given [23] |
| Stderr | when the workspace resolves to no project and `--all` was not given: `⚠️  This workspace resolves to no project; listing every project's tasks.` (`no project` [48]) and `  Bind it by creating .daisy/project, or pass --all to ask for this deliberately.` |
| Ops | ReadOnly (`todo.txt` and `done.txt` byte-identical and no commit after a query [28]) |
| Writes | none |
| Commits | none |
| Cases | 19-30, 46, 47, 48 |

### daisy projects

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy projects [--archived]` → `projects.sh` |
| Usage | `daisy projects [--archived]` |
| Exit codes | 0; 1 outside a workspace |
| Stdout | `Active projects:` (or `Archived projects:` for `projects/_archive/`) then `  %-30s <real path>` per `*.md`; `Active projects: none` when there are none |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | none (cases 42–44 pin project *resolution* through `files`) |

### daisy rotate

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy rotate` → `rotate.sh` (also run by `new-day` and `new-week`) |
| Usage | `daisy rotate` |
| Exit codes | 0, also when `journal.md` is missing, empty or holds five day-blocks or fewer (nothing written); 1 when `journal.md` has non-blank content before its first `### YYYY-MM-DD` heading or a block does not start with one (`Error: rotate.sh: …` on stderr, nothing written) |
| Stdout | `✅ Rotated N day-block(s); journal.md retains its last 5` when anything moved; nothing otherwise |
| Ops | Real |
| Writes | `journal/journal.md` keeps its last five `### YYYY-MM-DD` blocks; each older block is appended to all three rolling files — `journal-<YYYYMMDD>-last14.md` (which then drops its oldest block beyond 14), `journal-<YYYYMMDD>-mtd.md` and `journal-<YYYYMMDD>-ytd.md`, each named by the full date of its first block — and the month and year files close into `journal-YYYYMM.md` / `journal-YYYY.md` at the calendar boundaries, so a block is in the last-14 window and in exactly one month and one year file (as built; the plan's Step 19 text says "exactly one window file", which does not describe the bash — see D30); each window file carries `# <label>`, `## Table of Contents`, `## Summary`. Step 19's claim is byte equality with `rotate.sh` on a real journal at three dates |
| Commits | none of its own (`new-day`/`new-week` commit) |
| Cases | none |

## Plans

### daisy plan new

| Field | Value |
|---|---|
| Status | agreed rename (Step 22); behaviour as built |
| Legacy | `daisy plan-new [--spec] <desc> [project-tag]` → `plan-new.sh` |
| Usage | `daisy plan new [--spec] <description> [+project]` |
| Arguments | `<description>`: required (`Error: Description required`, exit 1); slugged (`[^a-zA-Z0-9]` → `-`, runs collapsed, trimmed, at most 100 characters, `fix-four-defects` [55] stays readable); `[+project]`: leading `+` optional, stripped |
| Exit codes | 0 [55]; 1 when `PLAN.md` already exists (`Error: PLAN.md already exists in this workspace`), or with `--spec` when `<slug>_PLAN.md` does, when the slug is empty, or when the filename would reach 255 bytes: `the limit is 255` [55] and `Shorten the project tag` [55] on stderr, never the shell's `File name too long` [55] |
| Stdout | `✅ Plan created: <filename>`, `   Linked as: <ws>/PLAN.md`, and `  ✓ Added PLAN.md to .gitignore` / `  ✓ Created .gitignore with PLAN.md` when it was not ignored; `--spec`: `✅ Spec created: <slug>_PLAN.md`, `   Location: <path>`, `   Not registered with Daisy — promote later with: daisy plan pickup <path>` (renamed from `plan-pickup`), and the `*_PLAN.md` gitignore line |
| Ops | Real |
| Writes | `$DAISY_HOME/plans/<yymmddHHMMSS>.<project>.<slug>.plan.md` (no project: `<ts>.<slug>.plan.md`) from `templates/plan.md` (`{Description}{project_suffix}`, `**Project:** +{project}`, `{YYYY-MM-DD HH:MM}`, `$DAISY_ROOT` filled); the relative symlink `<ws>/PLAN.md` → `.daisy/plans/<filename>`; `<ws>/.gitignore` gains `PLAN.md`. `--spec`: `<ws>/<slug>_PLAN.md` only, `.gitignore` gains `*_PLAN.md`, nothing under the home |
| Commits | none (as built: the plan file is committed by the next home commit) |
| Cases | 55 |

### daisy plan pickup

| Field | Value |
|---|---|
| Status | agreed rename (Step 22); behaviour as built |
| Legacy | `daisy plan-pickup <path>` → `plan-pickup.sh` |
| Usage | `daisy plan pickup <path>` |
| Exit codes | 0; 1 with no path, a missing file (`Error: File not found: <path>`), an existing `PLAN.md`, or no `# <Description>` H1 in the file |
| Stdout | `✅ Picked up: <basename> → <filename>`, `   Linked as: <ws>/PLAN.md`, `   Original left in place at: <path>`, then `commit`'s line |
| Ops | Real |
| Writes | copies the spec to `$DAISY_HOME/plans/<ts>.<project>.<slug>.plan.md` (description and `— +project` read from the H1), links `PLAN.md`; never touches the original |
| Commits | home, once: `Pick up plan: <filename>` |
| Cases | none |

### daisy plan archive

| Field | Value |
|---|---|
| Status | agreed rename (Step 22); behaviour as built |
| Legacy | `daisy plan-archive` → `plan-archive.sh` |
| Usage | `daisy plan archive` |
| Exit codes | 0 [54]; 1 when `PLAN.md` does not exist, is not a symlink (`not a Daisy plan`), or its target is missing (`The symlink may be broken…`) |
| Stdout | `✅ Archived: <filename>`; `  ✓ Repointed links in N project file(s)` when any (`project file` [54]); then `commit`'s line |
| Ops | Real |
| Writes | moves `plans/<filename>` to `plans/archive/<filename>`, removes `<ws>/PLAN.md`; in every `projects/*.md` and `projects/_archive/*.md` rewrites `../plans/<filename>` to `../plans/archive/` [54] `<filename>`, leaving unrelated project files byte-identical [54] |
| Commits | home, once: `Archive plan: <filename>` |
| Cases | 54 |

## Homes and identities

### daisy home

| Field | Value |
|---|---|
| Status | as built (Steps 1–3) |
| Legacy | `daisy home [list\|add\|remove\|new]` → `home.sh` |
| Usage | `daisy home list` (also bare `daisy home`), `daisy home add <name> <path>`, `daisy home remove <name>`, `daisy home new <name> [path]`; anything else: `Usage: daisy home list \| add <name> <path> \| remove <name> \| new <name> [path]` on stderr, exit 1 |
| Exit codes | `list`: 0 [68] (69 and 70 run it and assert stdout only). `add`: 0 registered [68]; 1 when the name is `already registered` [68], invalid (`bad=name`, case 68), the path is not a directory [68], has a path problem (`Error: <path> <problem>`: the root itself — `cannot be a home` [68] — or a directory containing it, `contains the Daisy root; …`, which case 68 pins on the validator `daisy_home_path_problem` directly, not through `home add`; the path is already resolved here, so the other two problems cannot arise), has no `include.txt` [68], is `not in its own git repository` [70] (or the root's), or nothing under it can be staged. `remove`: 0 [68]; 1 for an unregistered name [68]. `new`: 0 created [69]; 1 when the name is registered (nothing written, case 69) or the location `already exists` [69]; 1 when any step after the first write fails — the copy is removed: `was removed and nothing is registered` [70] on stderr; 1 when a prompt gets EOF |
| Stdout | `list`: `%-14s %s` name and path per home (`testhome` [68] and its path [68, 69]; ` (missing)` when the directory is gone); no homes: `No homes registered in <registry>` on stderr, exit 0. `add`: `✓ Registered home '<name>' at <path>`. `remove`: `✓ Unregistered home '<name>' (its files at <path> are untouched)`. `new`: the prompts `  Location [<default>]: ` (only without `[path]`), `Git identity for home '<name>':`, `  Name: `, `  Email: ` through `Ops.Prompt`; `  ✓ Saved git identity to <path>/gitconfig` or `  ⚠ Skipped (both name and email required)…`; `  ✓ Initialized git repository with first commit`; `prompt build`'s report; `✅ Created home: <name>`, `   Location: <path>`, `📋 Next steps:` … `daisy init <name>` |
| Stderr | the errors above, each with its fix on a second line (`include.txt` [68] names what a home must contain); `add` refusing a non-repository says `not in its own git repository` [70] |
| Ops | `list` ReadOnly; `add`, `remove`, `new` Real (decided, D30) |
| Writes | the registry `$DAISY_CONFIG/homes` (`name=/absolute/path` lines; `add` stores the resolved real path). `new`: copies `templates/home/` to the location (default `$DAISY_LOCAL/homes/<name>`), writes `gitconfig` as a `name=` [69] line and an `email=` [69] line, `git init` with `user.name`/`user.email` set and a first commit `New home: <name>` as that identity [69], registers, builds `<home>/AGENTS.md`; the template `.gitignore` ignores `.healthcheck.ttl` and `AGENTS.md` [69]; any failure removes the copy and registers nothing [70] |
| Commits | `new`: the new home's first commit; otherwise none |
| Cases | 68, 69, 70 |

Case 68 also pins `init` against the registered homes (`init second` binds
the links into that home) and `status` after `home remove` (`not registered`
[68], exit 1); case 69 that `init fresh` binds `.daisy/today.md` into the
new home.

### daisy identity list

| Field | Value |
|---|---|
| Status | agreed (Architecture): the command; decided (D30, Step 21): its output |
| Legacy | none |
| Usage | `daisy identity list` |
| Exit codes | 0; 1 when a registered home cannot be read (unspecified; Go may choose whether to list the rest) |
| Stdout | one line per identity across the registered homes: the identity name and its home, as `daisy init` prints the list in its usage and unknown-identity errors today (`%-14s %s`, from `daisy_identities`: each home's own name first, then its `identities/*`). Nothing else is agreed |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | none (the Acceptance suite section says new behaviour gets a bash-form case before any Go) |

## Prompts and skills

### daisy prompt build

| Field | Value |
|---|---|
| Status | agreed rename (Step 20); behaviour as built |
| Legacy | `daisy build [home]` → `build-prompt.sh` |
| Usage | `daisy prompt build [<home>]`: with `<home>` builds `<home>/AGENTS.md` (no workspace needed); without, builds the current workspace's `.daisy/AGENTS.md` for its bound home (the dispatcher passes `--output <ws>/.daisy/AGENTS.md`). `--output <file>` is an internal flag the dispatcher and `init` use (`home new` and `optimize` call the script with the home name, writing `<home>/AGENTS.md`); Go calls the function directly and need not expose it |
| Exit codes | 0; 1 for an unregistered home or a missing `include.txt` (`Error: <home>/include.txt not found`) |
| Stdout | `✅ Built AGENTS.md for home: <home>`, `   Full: <names>`, `   Lazy: <names>`, `   Output: <file>`, `   Size: N lines` |
| Stderr | `Warning: Prompt not found: <path>` (an error block is written into the output and the build continues); `Warning: No ## Trigger section found in <path>` (included in full) |
| Ops | Real |
| Writes | the output file only: a header (`# Daisy Prompt - Auto-generated`, `# Home:`, `# Built: <timestamp>`, `# Source:`, …), then per `include.txt` line a banner and the prompt — whole for `name`, the `## Trigger` section for `~name` — resolved in `<home>/prompts/` then `$DAISY_ROOT/prompts/`, with `$DAISY_ROOT` substituted. Step 20's claim is byte equality with the live `.daisy/AGENTS.md` except the `Built:` line |
| Commits | none |
| Cases | none |

### daisy install

| Field | Value |
|---|---|
| Status | as built; the symlink target becomes `$DAISY_ROOT/bin/daisy` (agreed, Step 17) |
| Legacy | `daisy install [--update]` → built in to `daisy.sh` (`cmd_install`); the only command that derives `DAISY_ROOT` from its own location instead of the environment |
| Usage | `daisy install [--update]`: `--update` re-runs the symlink, root skills and permissions only; plain `install` is the interactive bootstrap (default home, shell rc) and refuses when already installed (`Error: Daisy is already installed.`, exit 1) |
| Exit codes | 0 [31]; 1 already installed, `~/bin/daisy` exists and is not a symlink, an invalid home selection |
| Stdout | `Installing Daisy...` / `Updating Daisy...`, `  DAISY_ROOT: <root>`, ✓ lines per step, `  ✓ Installed N skill(s) into <dir>`, `  ✓ Pruned N stale skill(s) from <dir>` (case 31: a `$HOME/.claude/skills/<name>` with no source under `skills/` is removed and current skills are installed), `Done.` / `Done. To activate, run:` … |
| Ops | Real |
| Writes | `$HOME/bin/daisy` symlink; `$HOME/.claude/skills/<name>/` for every `SKILL.md` found under `skills/` (copies, named by the containing directory; stale ones pruned); `$HOME/.claude/settings.json` allow rules (`Read(/$DAISY_ROOT/**)`, `Edit(/$DAISY_ROOT/**)` — the leading slash plus the absolute root gives the `//`-anchored form — and `Bash(daisy:*)`; the legacy `Bash(…/daisy/scripts/*)`, single-slash and triple-slash rules are removed); without `--update`: the shell rc (`DAISY_ROOT`, `DAISY_DEFAULT_HOME`) and, with no homes, `home new` |
| Commits | none |
| Cases | 31 |

### daisy feedback

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy feedback [--workflow <n>] <text>` → `feedback.sh` |
| Usage | `daisy feedback [--workflow <n>] <text...>`; `echo <text> \| daisy feedback [--workflow <n>]`; `-w` is accepted for `--workflow` (kept, unasserted) |
| Arguments | `<n>`: `prompts/<name>`, `skills/<name>`, or a bare name resolved as a prompt, a skill, or an alias for `prompts/daisy` (`new-day`, `new-week`, `done`, `log`, `add`, `priority`, `cancel`, `status`, `sync`, `project`, `projects`, `home`); a tag that names no file is refused before anything is written [67] |
| Exit codes | 0 recorded [67]; 1 for an unresolvable workflow [67], no message (usage and the workflow list on stderr), an empty message, a missing `feedback/feedback.md` |
| Stdout | `  ✓ Feedback recorded (workflow: <n>, <YYYY-MM-DD HH:MM>)`, `  File: <path>` |
| Stderr | `Error: workflow '<n>' names no prompt or skill` (`names no prompt or skill` [67]), `  Qualify the target, e.g. --workflow prompts/daisy or --workflow skills/git.`, `  Available:` and every target (`prompts/daisy` [67], `skills/git` [67]) |
| Ops | Real |
| Writes | appends to `feedback/feedback.md`: blank, `## <YYYY-MM-DD HHMM>`, `**Workflow:** <n>` (`**Workflow:** skills/git` [67]) when given, blank, the text; nothing on refusal (byte-identical, case 67) |
| Commits | none (as built) |
| Cases | 67 |

### daisy optimize

| Field | Value |
|---|---|
| Status | as built for the bash loop (cases 38–41, 56–64 pin it); agreed redesign at Step 23 (D12): no API key, the briefing, `apply` |
| Legacy | `daisy optimize [--workflow <n>] [--dry-run] [--force] [--list]` → `optimize.sh` (`-w` for `--workflow`, kept) |
| Usage (agreed) | `daisy optimize [--workflow <n>]` prints the briefing; `daisy optimize apply --workflow <n> (--rules\|--rewrite) <file> [--yes]` applies text the agent proposed. `--dry-run`, `--force`, `--list`, the 2-entry floor, `EDITOR` and the `y/N/e` prompt go with the API call |
| Briefing (agreed) | under `ReadOnly`: for each target with pending entries, the target path, the entries, its current `## Rules` section and rule count; for every active prompt or skill whose `## Rules` holds more than 10 rules, a rewrite flag, pending entries or not; `Nothing pending` and exit 0 otherwise. Grouping as built: entries group by `**Workflow:**` header, an unheadered entry is `daisy`'s (`[daisy]` [41]); `--workflow <n>` scopes to entries declaring it (`1 entry` [40]); a target that is a skill resolves to its `SKILL.md` (`skills/widgetcraft/SKILL.md` [49]); a `vendored` [50] skill (carries `LICENSE`/`NOTICE.md`) is refused with the reason and the run continues. Exact briefing wording: unspecified; Go may choose. Step 23 rewrites cases 38–41 and 56–64 to it before any Go |
| Apply (agreed) | `--rules <file>` replaces the target's `## Rules` section only — the section ends at the next `#` or `##` heading, an `###` is inside it, and replacing the rules with themselves changes nothing (case 53, `extract_rules_section`/`replace_rules_section`; `# Document Title` [53, 58], `## Next Section` [53] and `Body text` [53, 58] survive); `--rewrite <file>` replaces the whole file. Both show the diff and ask through `Ops.Prompt` unless `--yes`. On approval: write the target (`Updated ` [58] `<path>`), move the target's pending entries from `feedback.md` to `archive.md` with the provenance line `**Addressed by:**` [63] `` `daisy optimize` <date> — `<target>` (rules\|rewrite)`` (as built: `## Rules` [63] after the target; the target named relative to the home, or `$DAISY_ROOT/…` for a shared file), each entry of the group annotated [63]; `feedback.md` keeps its header and the untrained entries with their original spacing, byte for byte (case 66); rebuild `AGENTS.md`; commit the home. A target under `$DAISY_ROOT/prompts/` or `skills/` is written and left for `daisy commit --root`, and the run says so (wording unspecified). Declining: `Cancelled` [59], nothing written, feedback and archive byte-identical [59]; EOF on the prompt is not a failure, exit 0, and says `EOF` [64] |
| Exit codes | 0 (including nothing pending, a declined apply, EOF); 1 when `feedback/feedback.md` does not exist (`Error: No feedback file found at <path>`); 1 for a bad option (bash: `Unknown option: <x>`; agreed exit 2), a target file that does not exist, an apply whose file cannot be read. As built only: 1 when the LLM call fails (`credit balance is too low` [61] passed through, naming the workflow `widgetflow` [61]) or returns an `Empty response` [61], with nothing written and nothing archived [61]; 1 without `ANTHROPIC_API_KEY` [57] once a group reaches the LLM |
| Stdout (as built) | `Found N entries across N workflow(s)` (or `for workflow '<n>'`); per group `[<wf>] N entries` (`[plan]` [38, 41, 50], `[retrospective]` [38]), `  Target: <path>` (`Target: ` [57]) (never `prompts/daisy.md` [38] for a headered group), `  Skipped — below the 2-entry floor.` (`Skipped` [39, 40, 62], `2-entry floor` [39, 62]) with the `--force` [62] hint, or `running anyway` [62] with `--force`; `  Entries: N` (`Entries: 2` [56]) and no `Test:` [56] / `Train:` [56] split; the diff block; `dry-run` [57] note writing nothing [57]; `Apply these changes to <file>? [y/N/e(edit)] `; `  ✓ Updated <path>`; `Rebuilding AGENTS.md...`; `  ✓ Archived N entries to <archive>`; `Done. Run 'daisy feedback' to continue collecting failures for the next cycle.` |
| Ops | briefing ReadOnly; `apply` Real |
| Writes | `apply` only: the target prompt or `SKILL.md` (a home file, or a root file left uncommitted), `feedback/feedback.md`, `feedback/archive.md`, the home's `AGENTS.md`, `feedback/.last-optimized` (as built; `healthcheck` Check 7 reads it). The suite's fixture copies `prompts/` and `skills/` so an applying run writes the fixture's copy, never the repository's (case 65) |
| Commits | as built: none (the bash `optimize` writes the target and the archive and leaves them for the next home commit). Agreed (Step 23): `apply` with a home target commits the home once; a root target: none (`daisy commit --root` is the user's) |
| Cases | 38, 39, 40, 41, 49, 50, 53, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66 |

### daisy eval

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy eval [<case>] [--record]` → `eval.sh` |
| Usage | `daisy eval` lists the cases in `$DAISY_ROOT/prompts/evals/*.md`; `daisy eval <case>` prints one; `daisy eval --record <case> pass\|fail ["notes"]` records a result (the agreed help line `eval [<case>] [--record]` abbreviates this form). With no notes and a terminal, asks `Notes (optional): ` |
| Exit codes | 0; 1 for a missing evals directory, an unknown case (`Error: Eval case not found: <case>`), a result other than `pass`/`fail`, `--record` without a case |
| Stdout | list: `No eval cases found in <dir>` (exit 0) or `Available eval cases:`, `  %-20s  <H1 title>` per file, the usage lines; one case: the file, then `  Record result: daisy eval --record <case> pass\|fail "notes"`; record: `  ✓ Recorded <result> for eval '<case>' (<YYYY-MM-DD HH:MM>)`, `  File: <path>` |
| Ops | Real (decided, D30: one wrapper per command; `--record` writes) |
| Writes | `--record`: `$DAISY_HOME/feedback/eval/results.md` (created with a header), appending `## <YYYY-MM-DD HHMM>`, `**Case:**`, `**Result:**`, the notes |
| Commits | none |
| Cases | none |

## Swarm

### daisy swarm

| Field | Value |
|---|---|
| Status | agreed (Steps 26–30); specified in `docs/swarm.md` (Step 26), not here |
| Legacy | none |
| Usage | `swarm init [--defaults]`, `swarm apply [--once]`, `swarm status`, `swarm pause`, `swarm resume` — the five help lines above and nothing more |
| Cases | none |

## System

### daisy commit

| Field | Value |
|---|---|
| Status | as built for `--home`/`--not-home`; agreed `commit <message>` / `commit --root <message>` at Step 24 (no aliases) |
| Legacy | `daisy commit --home\|--not-home <message>` → `commit.sh` (the dispatcher requires a workspace and a resolved home first) |
| Usage | `daisy commit <message>` (the home: `--home` today); `daisy commit --root <message>` (the root: `--not-home` today). Missing mode: the usage line on stderr, exit 1; missing message: `Error: Commit message required` and the usage line, exit 1. Step 24 rewrites case 73's `--not-home` to `--root` (decided, D30: the case passes the flag itself) |
| Exit codes | 0 committed [71, 73] or nothing to commit [73]; 1 when the home is not in its own repository (`Error: Home '<name>' at <path> is not in its own git repository (or the root's)`, `  Run: git -C <path> init`), the root is not a repository, or `git add`/`git commit` fails |
| Stdout | `📝 Committed: <message> (<short hash>)` |
| Stderr | `No changes to commit` [73] ` in <scope>` (scope `home '<name>'`, `root`, or `root (excluding <paths>)`), exit 0 |
| Ops | Real |
| Writes | nothing but the repository |
| Commits | home: `git -C $DAISY_HOME add -- .` and one commit; while the home is a subdirectory of the root's repository the commit is path-scoped to it, so a pre-staged root change stays staged (case 71: the commit holds `home/zebranest/journal/today.md` [71], not `code.txt` [71]); root: the root repository minus `daisy_root_exclusions` (every registered home inside it, and a top-level `home/` while it exists), as `:(exclude,literal)` pathspecs on `add` and, while anything is excluded, on `commit` (case 73: `code.txt` [73] committed, `zebranest` [73] and `stray` [73] not, the home's change left staged); with nothing excluded a bare `git commit` that also takes pre-staged changes (`more.txt` [73]). The root commit is the publication gate's subject: `daisy healthcheck --force` before it |
| Cases | 71, 72, 73 |

### daisy healthcheck

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy healthcheck [--force]` → `healthcheck.sh` (also run silently first by `done`, `log`, `new-day`, `new-week`) |
| Usage | `daisy healthcheck [--force]` |
| Caching | a passing run writes the unix expiry time to `$DAISY_HOME/.healthcheck.ttl`; a run within `DAISY_HEALTHCHECK_TTL` seconds (default 300; `0` disables) exits 0 after Checks 1–2 without re-checking (no `Component:` [37] lines) and honours the stamp even over a new leak [37]; `--force` ignores it [37]; a failure removes the stamp [37]; a corrupt or too-far-ahead stamp is no cache [37]; `DAISY_HEALTHCHECK_PASSED=1` in the environment short-circuits unless `--force` [37] |
| Exit codes | 0 healthy [00, 34, 35, 36, 37, 74, 75] — warn-only checks never fail it [34, 74]; 1 on any error: `DAISY_ROOT` unset or not a repository, the home unresolvable, the home `not in its own git repository` [74] (or a foreign one [74]; the fix `git -C ` [74] `<path> init && … commit` printed), a missing required directory, `today.md` format faults, a component hook failing, a publication-hygiene hit [35, 36, 37, 75] |
| Stdout | one `✓ <check>` line per pass: `DAISY_ROOT:`, `DAISY_HOME:`, `Git repository: <hash>`, `today.md format valid`, `Component: <script>` (`Component:` [37]) per hook, `Publication hygiene: no identifying terms outside the homes`, `Feedback: N entries (last optimized: …\|never optimized)`, `System healthy`; cached: `Checks cached — valid for Ns (--force to re-run)` |
| Stderr | `✗ <error>` and `⚠ <warning>` lines. Hygiene (Check 6c): `Publication hygiene: 'zebrahome'` [35, 37] / `Publication hygiene: 'Zebra Quillfeather'` [75] ` appears in publishable file(s) — …` then each file indented (`leak.md` [35, 75], `public-leak.md` [36]); terms are every registered home's name (placeholders `work`, `personal`, `example`, `testhome` exempt), `$USER`, the root repository's `user.name`/`user.email` and each usable home's, and `$HOME/`; the surface is `git ls-files --cached --others --exclude-standard` in the root minus `daisy_root_exclusions`, regular files only — an ignored file is never reported (`ignored-leak.md` [36] absent) and an untracked, unignored one is [36]. Warnings: `Root-installed skill 'definitely-not-a-real-skill' has no matching source` [34] (Check 4c), `Installed skill '<name>' has no matching source in skills/` (4b), `Home 'zebranest' lies inside the root repository` [74] (3b; absent when no home is inside, `lies inside` [74]), `journal.md has N day-blocks (expected ≤ 5)` and `<window file> is missing TOC/Summary front matter` (5b), `<script>.sh is missing from AGENTS.md's Script Reference table` (6b; becomes the `cli` table drift check at Step 24), `Feedback has N entries — consider running 'daisy optimize'` (7, at 20 or more); `✗ N issue(s) found` last |
| Ops | Real (the stamp) |
| Writes | `$DAISY_HOME/.healthcheck.ttl` only |
| Commits | none |
| Cases | 00, 34, 35, 36, 37, 74, 75 |

### daisy check-secrets

| Field | Value |
|---|---|
| Status | as built |
| Legacy | `daisy check-secrets` → `check-secrets.sh` |
| Usage | `daisy check-secrets` |
| Exit codes | 1 when `<ws>/.env.sh` is missing (`❌ .env.sh not found in workspace` and the copy hint on stdout); otherwise 0 |
| Stdout | `Checking Daisy environment configuration...`, `Core paths:` (`DAISY_ROOT`, `DAISY_HOME`), `API tokens:` (Webex, Backstage, JIRA, GitHub), each `✅ <name>: <first 8 chars>...` or `⚠️  <name>: Not configured`, `Note: Only showing first 8 characters of tokens for security` |
| Ops | ReadOnly (decided, D30; the bash `source`s the file, Go parses it) |
| Writes | none |
| Commits | none |
| Cases | none |

### daisy test

| Field | Value |
|---|---|
| Status | as built; at Step 24 runs `go test ./...` then the acceptance suite |
| Legacy | `daisy test [name-substring]` → `test.sh` (`exec`s `tests/run.sh`) |
| Usage | `daisy test [filter]`: a case runs when its file name contains the filter |
| Exit codes | 0 when every case passes; 1 otherwise |
| Stdout | `PASS  <case>` / `FAIL  <case>  (N assertion(s))` per case, failed assertions indented under it, blank, `N passed, N failed`, `Failed: <names>` when any |
| Ops | Real (the suite builds fixtures) |
| Writes | temporary fixtures only |
| Commits | none |
| Cases | none |
