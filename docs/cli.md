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
| `plan` and its verb as one hyphenated word, three bash scripts | `daisy plan new [--spec] <description> [+project]`, `daisy plan pickup <path>`, `daisy plan archive` | 22 |
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
argument, and — for a command whose arguments have places, every one but
`log` — an empty word before a given one, `Error: empty argument`, so that
`plan new "" +proj` is not the description `+proj` *(D37, Step 22)*), with
the command's usage on stderr. The bash exits 1 for usage
errors too; no case asserts a usage exit code, so 2 is free to take
*(agreed, Step 17)*. A value the command refuses — an unknown identity, an
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
`daisy -h` print the help: as built the flat `Commands:` list of `show_help`;
from Step 17 the tree, in the agreed groups and order, each command under the
name this build answers to (`plan new`, `plan pickup`, `plan archive`,
`home new`, `home add`, `home list` and `home remove` one row each, no `swarm` until it exists) — the tree never names a
command the binary does not run. *The four forms and the sole-argument
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
with `Error: DAISY_ROOT not set.` without it; the binary, for a command it
runs itself, with `Error: DAISY_ROOT not set` and the `~/.zshenv` line to
add; for every command it has not taken over the binary execs
`$DAISY_ROOT/daisy.sh`, and without `DAISY_ROOT` refuses with the same
error; `install` alone finds its root without it, from the binary's own
path, Step 21), `DAISY_CONFIG`,
`DAISY_LOCAL`, `DAISY_DEFAULT_HOME`, `HOME`, `USER`, `DAISY_HEALTHCHECK_TTL`,
`DAISY_HEALTHCHECK_PASSED` (see `healthcheck`), `SHELL` (`install` picks the
rc file by it). Nothing else: `optimize` holds no API key and reads no
model or editor setting (Step 23, D12).

**`--healthcheck` hook.** The scripts listed in `healthcheck.sh`'s
`HEALTHCHECK_SCRIPTS` (`files`, `test`)
and the Go commands listed in its `HEALTHCHECK_COMMANDS` (`done`, `log`,
`tasks`, `projects`, `new-day`, `new-week`, `rotate`, `list`, `home new` —
the template is there —, `plan new` — `plans/` and `templates/plan.md` —,
`plan pickup` — `plans/` —, `plan archive` — `plans/archive/` —,
`feedback`, `eval` — the workspace and its home resolve —, `optimize` —
the home's `feedback/` —, asked
through the binary) accept `--healthcheck` as the
sole argument: check their own preconditions, print nothing on success,
exit 0; else a one-line `Error: …` on stderr, exit 1. Two cases pin it from
outside (`done --healthcheck` [18], `tasks --healthcheck` [30]). In the
binary the hook is a per-command `Health` function in the `cli` table,
hidden from the help text; `healthcheck` runs the same checks in-process at
Step 24 *(decided, D30, Step 18; built in Step 18)*.

**`Ops` wrappers.** `ReadOnly` for `status`, `files`, `list`, `projects`,
`tasks`, `help` and the briefing, `optimize` (Step 23),
and for `check-secrets`, `home list` and `identity list` (decided, D30);
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
`add -A`, `commit` *(Step 21)*; `plan new`, `feedback` and `eval --record`
write home files without committing them, and the next home commit sweeps
them up *(decided, D37 for `plan new`, D38 for the other two)*. The root
is committed only by `commit --not-home` (`--root` after Step 24):
`optimize apply` writes a root target and leaves it for that commit.

## Workspace

### daisy init

| Field | Value |
|---|---|
| Status | ported (Step 21): Go (`internal/workspace`, `internal/identity`); `init --new` removed (`home new`, then `init`) |
| Legacy | `daisy init [--new] <identity> [path]` → the bash `daisy-init` script, deleted in Step 21 |
| Usage | `daisy init <identity> [path]` |
| Arguments | `<identity>`: a registered home's name (its default identity, `gitconfig`) or the name of `<home>/identities/<identity>` in exactly one registered home. `[path]`: the workspace directory, default `.`. No `<identity>` or a third argument: usage on stderr, exit 2 (`daisy identity list` lists the identities) |
| Exit codes | 0 bound (also when already bound, or re-bound to another identity); 1 refused, nothing written: identity in two homes (`identity 'pelicanmoor' is defined in more than one home: testhome, second` [78], `more than one home` [78]), unknown identity (names it, `nosuchone` [77], and lists the identities that exist, `pelicanmoor` [77]), `not a valid identity name` [77], a registered home that cannot be checked (names it and `does not exist` [78]), a named identity file that is incomplete (names the file `identities/pelicanmoor` [79] and the missing key `email` [79]), a malformed `sshkey` [79], a `linked worktree` [76] (names the main checkout), the directory holding the `Daisy root` [78], `.daisy` `not a directory` [77] or a `symlink` [77], two leftovers and no `.daisy/` (names both, `.daisy.previous.100` [77], `.daisy.previous.200` [77]), a settings file that is not a JSON object [77, 78], a directory where `.daisy/` holds a file of init's own [77, 78]; every refusal is decided before the first write — an unknown identity, an unreadable settings file, a file where init needs a directory or a directory where it writes a file, a home skill named like a core rule (`daisy`, `daisy-logging`, `daisy-plan`) — so a refused init writes nothing, not even `git init`; 1 when a step of the final section fails: `did not finish` [77, 78] on stderr and everything rolled back (binding, git keys byte for byte, stage removed) |
| Stdout | `Initializing Daisy in: <path>`, `Home: testhome` [77], `Identity: testhome` [76] / `Identity: pelicanmoor` [77] (the build's own report is not printed; its warnings go to stderr); on a re-bind `Already initialized` [77] `with identity: <identity>` (`Already initialized with identity: pelicanmoor` [77]) or the switch `pelicanmoor → testhome` [77]; the ✓ lines for each artifact; `Bound .daisy/` [79] `to home: <home>`; a cleared previous identity names its keys (`core.sshCommand` [79]); `Done. Daisy is ready in this workspace.` |
| Stderr | warnings, exit still 0: `no complete default identity` [76] (the home's `gitconfig` is template or incomplete; no identity applied), an sshkey that is `not found` [79] (names it, `no-such-key` [79]), a reclaimed leftover `.daisy.previous.12345` [77], `No readable .daisy/gitconfig record` [79] of the previous identity, an aside that `could not be removed` [79]; errors as above |
| Ops | Real |
| Writes | `<ws>/.daisy/` as a whole (staged beside it, swapped in by rename): `home`, `identity` (non-default only), `gitconfig` (the applied triple), `gitconfig.before` (what the three keys held, once the previous identity was taken back, when any held a value), the six links `tasks today.md journal.md projects plans feedback.md` into the home, `AGENTS.md`; `<ws>/.cursor/rules/daisy.mdc daisy-logging.mdc daisy-plan.mdc` (obsolete `daisy.md`, `daisy-logging.md` removed, case 32), `<ws>/.claude/commands/daisy.md`, `<ws>/.claude/skills/<name>/` and `.cursor/rules/<name>.mdc` for each home skill (a stub in exactly the form init writes — the one record of which skills init installed — whose skill is gone is swept with its copy, case 33; any other rule is the user's; a link standing where a Cursor rule or a stub goes is replaced by init's file, never written through), `<ws>/.claude/settings.local.json` (Daisy's allow rules), the entries missing from `<ws>/.gitignore` (`.cursorignore` among them, from Step 21) and `.cursorignore`, into the block under Daisy's header line (applying twice equals once); `git init` when the directory is not a repository top level; local git config `user.name`, `user.email`, `core.sshCommand` (`ssh -i <key> -o IdentitiesOnly=yes`, only with `sshkey=`) — the previous identity's keys unset first where their values are still its, and what they held before that identity was applied put back |
| Commits | none |
| Cases | 32, 33, 76, 77, 78, 79 |

Case 76 also pins `init` under an inherited `GIT_DIR` (ignored, exit 0),
`log` under a hook's `GIT_CONFIG_PARAMETERS`, `GIT_CONFIG_COUNT` and
`GIT_AUTHOR_*`/`GIT_COMMITTER_*` (ignored; the commit lands in the home),
and that `init` in a subdirectory of a linked worktree is allowed.

### daisy clean

| Field | Value |
|---|---|
| Status | ported (Step 21): Go (`internal/workspace`) |
| Legacy | `daisy clean [-f]` → built in to `daisy.sh`, deleted in Step 21 |
| Usage | `daisy clean [-f]` (`--force` accepted as well; kept) |
| Arguments | without `-f`: prints `This will remove Daisy from: <path>`, `Home:`, `Identity:` and asks `Continue? [y/N]` through `Ops.Prompt` (on stderr, as the bash's `read -p`); anything but y/yes, end of input included, prints `Cancelled.` and exits 0 |
| Exit codes | 0 removed (also when nothing matched); 1 outside a workspace (beside the Daisy root, case 78); 1, before anything is removed, when `.claude/settings.local.json` is not a JSON object; 1 when a git key `could not unset` [79] — the record is kept for the retry, and the message is not `left as is` [79] |
| Stdout | `Cleared local git identity (user.name, user.email)` [76, 77, 79] naming exactly the keys cleared — `Cleared local git identity (user.name)` [76], `Cleared local git identity (user.email)` [79], `Cleared local git identity (user.name, user.email, core.sshCommand)` [79]; `left as is` [76] when nothing matched; `No readable .daisy/gitconfig record` [76] when there is no record and no identity file resolves; `Removed .daisy/`, `Removed <leftover>` (`.daisy.previous.555` [76]), `Removed Cursor rules`, `Removed Claude command`, `Removed N skill(s)…`, `Cleaned .gitignore`, `Removed empty .gitignore`, `Removed Daisy permissions from .claude/settings.local.json` or `Removed .claude/settings.local.json` (when only Daisy's rules were in it), `Cleaned .cursorignore`, `Removed empty .cursorignore`; `· Local git identity not checked: this workspace is not a repository` / `…this is a linked worktree and its config is the main checkout's` when the keys cannot be checked; `Done. Daisy has been removed from this workspace.` |
| Stderr | `could not unset` [79]; otherwise unspecified |
| Ops | Real |
| Writes | removes everything `init` wrote (above) but the repository a `git init` made, and only that: the `.gitignore`/`.cursorignore` block under Daisy's header (a user's own line that is also an entry stays), Daisy's allow rules (the file goes only when nothing else is left in it), the skills init installed (those whose `.cursor/rules/<name>.mdc` is still a regular file in init's stub form, whatever the home holds now), the empty directories init made; unsets each of the three git keys when every value it holds is the recorded identity's (the home's identity file stands in for the record for a workspace bound before records existed) or is a `core.sshCommand` in Daisy's exact form (`ssh -i <path> -o IdentitiesOnly=yes`, which nothing but an init writes — taken with or without a record); a hand-set value is the user's and stays; a key it unsets gets back what it held before init (`.daisy/gitconfig.before`), so init then clean leaves the keys and `git status` as they were [76]; reclaims `.daisy.stage.*` leftovers |
| Commits | none |
| Cases | none of its own; exercised by cases 76/77/79 (`init`'s) and 78; the inverse is `TestInitThenCleanRestores` |

### daisy status

| Field | Value |
|---|---|
| Status | ported (Step 17): Go, as built |
| Legacy | `daisy status` → built in to `daisy.sh` (`cmd_status`), kept for `~/bin/daisy` links not yet switched to the binary |
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
| Status | ported (Step 17): Go, as built |
| Legacy | `daisy files` → `files.sh`, kept as `status`'s is |
| Usage | `daisy files` |
| Exit codes | 0 (a project that does not exist is not fatal, case 44); 1 outside a workspace; 2 with any argument |
| Stdout | the same first line as `status` (`identity: testhome` [76], `identity: pelicanmoor` [77]); sections `Identity:` (the resolved identity file: the home's `gitconfig` [76] or `identities/pelicanmoor` [77], ` (missing)` when absent), `Journal:`, `Tasks:`, `Projects:` (`workspace` row: `+chessgrid` [43] `(from ` `directory name` [43] `)`, `+chessgrid +jobsearch` [43] in file order, `+jobsearch` [42] `(from ` `.daisy/project` [42] `)`, or `(none)` (`none` [44])), `Feedback:`, `Plans:` (with `current plan` when `PLAN.md` is a link), `Prompts:`; each row `  %-12s <real path>`; last `AGENTS.md:    <real path>` (`%-14s`) |
| Stderr | empty when the convention default matches nothing (asserted `""`, case 44); a declared project with no file is named (`typoed-name` [44]); drift warnings |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 42, 43, 44 |

### daisy list

| Field | Value |
|---|---|
| Status | ported (Step 20): Go |
| Legacy | `daisy list` → the bash `list` script, deleted in Step 20 |
| Usage | `daisy list [--full]` (`--full` prints each skill's `description` instead of `short_description`; not in the agreed help line, kept as an unasserted flag) |
| Exit codes | 0; 1 outside a workspace, for an `include.txt` that cannot be read or a line of it with an unmatched quote (where the bash died mid-listing), or a prompt or SKILL.md that cannot be read; 2 with any other argument |
| Stdout | `Prompts (<home>):` then one `  %-20s <trigger summary or (always loaded)>` per `include.txt` entry (the width in bytes, as the bash's `printf`), `(missing: <path>)` for an absent prompt, `(no include.txt found)`; the summary is the trigger's first line that is not `Read the full…` and not empty once a leading `- ` is dropped, cut to 97 characters and `...` past 100, else `(lazy, no trigger summary)`; blank; `Skills:` then one `  %-20s <text>` per installed skill, user-level (`~/.claude/skills`) then workspace (`.claude/skills`), workspace overriding by name, in byte order of name (the bash's `sort` collated by locale); `<text>` is `short_description`, else `description`, one surrounding `"` dropped; or `(none installed — run: daisy install, then daisy init <home>)` |
| Stderr | the refusals above |
| Ops | ReadOnly |
| Writes | none |
| Commits | none |
| Cases | 85 |

### daisy help

| Field | Value |
|---|---|
| Status | ported (Step 20): Go. The tree (`daisy`, `daisy help`, `--help`, `-h`) is rendered from the `cli` table under the names this build answers to (legacy spellings until each rename's step) |
| Legacy | `daisy help [<command>\|<skill>]` → built in to `daisy.sh` (`show_help`, `show_command_help` and the skill lookup, the last removed in Step 20); since Step 20 `daisy.sh` keeps `show_help` lines and `show_command_help` only for the commands still in bash, which answer their own `--help` |
| Usage | `daisy help [<command>\|<skill>]`; `daisy`, `daisy --help`, `daisy -h` are `daisy help` |
| Exit codes | 0; 1 for a name that is neither (`Unknown command or skill: <name>` and `Run 'daisy help' for commands, 'daisy list' for skills.` on stderr; the bash died silently there) or outside a workspace for a skill; 2 for surplus words |
| Stdout | no argument: the tree. A command name (its words: `help prompt build`): exactly what `daisy <command> --help` prints — `Usage: daisy <usage>`, blank, the description, from the table for a Go command; the script header through `daisy.sh` for a command still in bash — then, when a skill of that name exists (installed or source; none looked up outside a workspace), a note — the command's help, not the skill's (not `Collide politely` [52]), and the word `skill` [52]: blank, `Note: a skill named '<name>' also exists; the command name takes precedence.`, `  Read it with: daisy list, or open its SKILL.md directly.` A noun (`help prompt`): the usage of each of its commands. A skill name: `<name>  (<SKILL.md path>)`, blank, the full `description:` value (`when to stop making them` [51]; the line after `description: `, `: ` inside it kept), blank, the `## Rules` section up to the next `#` or `##` heading, `optimize`'s boundary (`Measure before cutting` [51]; until Step 23 an `# ` heading and what followed it to the next `## ` were printed as rules), `Other sections: <H2 titles>` joined by `, ` (`Worked example` [51]). The installed copy (workspace over user) is preferred over the source (home over root) |
| Ops | ReadOnly (the skill lookup and the note); a bash command's help is the fall-through's |
| Writes | none |
| Commits | none |
| Cases | 51, 52 |

## Day

### daisy new-day

| Field | Value |
|---|---|
| Status | as built in Go (Step 19); the `optimize --list` nudge removed with `--list` (D30, built in Step 23) |
| Legacy | `daisy new-day` → the bash `new-day` script, deleted in Step 19 |
| Usage | `daisy new-day` (no argument; any is usage on stderr, exit 2 — the bash ignored them) |
| Exit codes | 0; 1 when `healthcheck` fails first (`Error: System health check failed` on stderr), when `templates/journal-day.md` or `tasks/todo.txt` is missing, when the journal cannot be rotated (`rotate`'s refusal), or when the commit fails. A refusal writes nothing: every write is decided before the first is made (the bash had appended `today.md` to `journal.md` before a rotate refusal, so the next run archived it twice) |
| Stdout | `📦 Archived yesterday to journal.md` (when `today.md` existed) [81], `rotate`'s line [81], `🗑️  Deleted N cancelled task(s)` (when any) [81], `✅ New day started: <YYYY-MM-DD> <Day>` [81], `   High priority tasks: N` [81], `   Next priority tasks: N`, `   Inbox tasks: N`, `   GitHub tasks: N` [81], then `commit`'s `📝 Committed: …` line (git's own commit summary, which the bash let through, is not printed). Nothing follows the commit line: the pending-feedback nudge went with `optimize --list` at Step 23 — the New Week workflow runs `daisy optimize` itself |
| Ops | Real; the health check is the bash `healthcheck.sh`, through `Ops.Exec` |
| Writes | the rotation window files and `journal/journal.md` (see `rotate`; `today.md` is appended after a `---` separator when the journal is non-empty, and rotated with it); `tasks/todo.txt` without its `z ` lines [81], every line newline-terminated; `journal/today.md` from `templates/journal-day.md`: `{DATE}`, `{DAY}`, `{TIME}`, then `{HIGH_PRIORITY_TASKS}` = `(A)` [81], `{NEXT_PRIORITY_TASKS}` = `(B)` [81], `{INBOX_TASKS}` = dated without priority [81], `{GITHUB_TASKS}` = any open line containing `@git`/`@github`, whatever its priority, decided before the priority branches [81]; `(C)`/`(D)` lines without such a tag are not carried [81]; each carried line loses its priority and creation date when it has a date, and is carried whole otherwise. A task's text is written as it is (bash 5.2's pattern substitution replaced an `&` in it with the placeholder's name) |
| Commits | home, once: `New day: <date> <day>` [72, 81] |
| Cases | 81; case 72 (`commit`'s) runs it |

### daisy new-week

| Field | Value |
|---|---|
| Status | as built in Go (Step 19); nudge removed as for `new-day` (D30, built in Step 23) |
| Legacy | `daisy new-week` → the bash `new-week` script, deleted in Step 19 |
| Usage | `daisy new-week` (no argument) |
| Exit codes | as `new-day`; the template is `templates/journal-week.md` |
| Stdout | as `new-day`, plus `📦 Archived N completed task(s) to done.txt` (when any) [82] after the cancelled line, and `✅ New week started: <date> <day>` [82] |
| Ops | Real |
| Writes | as `new-day`, plus `tasks/done.txt`: the `x ` lines of `todo.txt` appended [82] (after a newline when `done.txt` lacked a final one and something is archived — the bash glued the first onto its last line; with nothing archived the file is left as it was) and removed from `todo.txt` [82]; `today.md` from `templates/journal-week.md` [82] |
| Commits | home, once: `New week: <date> <day>` [82] |
| Cases | 82 |

### daisy log

| Field | Value |
|---|---|
| Status | as built in Go (Step 18) |
| Legacy | `daisy log <message...>` → the bash `log` script, deleted in Step 18 (the dispatcher joined the arguments with single spaces: `"$*"`; the binary does the same) |
| Usage | `daisy log <message...>` |
| Exit codes | 0; 2 with no message — no argument, or only empty ones (usage on stderr; the bash exited 1 with `Error: Log message required`); 1 when `healthcheck` fails first (`Error: System health check failed`, `  Run: daisy healthcheck`), when `today.md` is missing (`Error: today.md not found`) or has no `#### Log` section (`Error: Log section not found in today.md`) |
| Stdout | `✅ Logged: <HHMM> <message>` then `commit`'s line (`done` suppresses both: not `Logged:` [15]) |
| Ops | Real; the health check is the bash `healthcheck.sh` run through `Ops.Exec` until Step 24 |
| Writes | `journal/today.md`: appends `- HHMM message` to the `#### Log` section — the lines from the heading to the next heading of any level or the end of the file — after its last non-blank line, so blank lines before the next heading stay; a section with no entries is written in the template's shape (heading, blank line, entry, blank line before the next heading), which healthcheck Check 5 requires [80]. *(decided, D33, Step 18: the bash inserted at the first blank line after the section's first entry without stopping at the next heading, so a Log section abutting the next heading sent the entry into a later section)* |
| Commits | home, once: `Log: <HHMM> - <first 60 characters>` (case 72; case 71 pins that with the home still inside the root repository the commit holds only the home's file) |
| Cases | 80; cases 71/72 (`commit`'s) and 76/78 (`init`'s) run it |

### daisy done

| Field | Value |
|---|---|
| Status | as built in Go (Step 18) |
| Legacy | `daisy done <pattern>` → the bash `done` script, deleted in Step 18 |
| Usage | `daisy done <pattern>` (one argument) |
| Arguments | `<pattern>`: literal, case-insensitive substring (`.` is not a wildcard [08]; `/` is safe [07]; a leading `-` is a pattern, not a flag — except the sole-argument `--help`/`-h` and `--healthcheck`, which the cli takes first) matched against the active lines of `todo.txt` only (`x `/`z ` lines excluded before matching, cases 05, 06). Missing — no argument, or an empty one — or more than one: usage on stderr, exit 2 (the bash exited 1 with `Error: Task pattern required` and ignored extra arguments) |
| Exit codes | 0 exactly one match completed [01, 02, 05–08, 10–15]; 1 no match [04, 16], several matches [03], an Inbox-only item that is not in `todo.txt` [09], `healthcheck` failing first, or the journal entry failing after the edits [17] |
| Stdout | success: exactly one non-blank line [15], `✅ Done: <description> (commit <hash>)` — names the task (`Delta target task` [15]) and suppresses the sub-calls' chatter (not `Logged:` [15], `Committed:` [15], `No changes to commit` [15]); nothing with `✅` [16] when nothing completed; no `✅ Done` [17] when journaling failed |
| Stderr | `No task found matching:` [04] `<pattern>`; `Multiple tasks match "<pattern>":` and each candidate on its own `  N) <line>` (`Shared token Alpha` [03], `Shared token Beta` [03]); `⚠️  Task not found in today.md (may not be in active tasks)` when `todo.txt` is updated but no `- [ ] <description>` line exists (`today.md` [11]; exit still 0); on a journal failure `Error: journal entry failed — …` naming whether the edits were committed, then the sub-calls' output (`Log` [17]). Cases 03, 04, 11, 17 assert on stdout and stderr combined, so the stream is not pinned |
| Ops | Real; the health check is the bash `healthcheck.sh` run through `Ops.Exec` until Step 24 |
| Writes | `tasks/todo.txt`: the one matched line removed and `x <today> <creation date> <description>` appended last, no priority [10] — the line read by `todo.Parse`, so an undated task completes as `x <today> <description>` with one space and its priority dropped (the bash left the priority in the description and wrote two spaces); `journal/today.md`: the first line equal to `- [ ] <description>` becomes `- [x] <description>`, every other byte unchanged [01, 02, 12]; then the log entry `Done: <description>` in-process (no `log` sub-call) |
| Commits | home, exactly once for the whole completion, holding both files [13, 14], subject `Completed: <description>` *(decided, D33, Step 18: the bash's one commit was `log`'s, `Log: HHMM - Done: …`, and its own `Completed:` commit always found nothing)*; a failed journal entry still leaves the edits committed [17]; a failed commit after a successful completion is an error, exit 1 (the bash printed `(commit not committed)` and exited 0) |
| Cases | 01-18 |

### daisy tasks

| Field | Value |
|---|---|
| Status | as built in Go (Step 18) |
| Legacy | `daisy tasks [--all] [--todo\|--done] [pattern]` → the bash `tasks` script, deleted in Step 18 |
| Usage | `daisy tasks [--all] [--todo\|--done] [pattern]` |
| Arguments | scope: this workspace's projects by default — a line is in scope when one of its `+tag` words (whitespace-delimited, compared without case) is a workspace project — `--all` every project [19, 46, 47]; files: both by default [22], `--todo` = `todo.txt` [20], `--done` = `done.txt` [21], the last of the two given wins; `[pattern]`: literal, case-insensitive substring [24] (an empty pattern is none), a leading `-` is a pattern (`-1 point deduction task` [25]) once the three flags above have been consumed — except the sole-argument `--help`/`-h` and `--healthcheck`; flags before the pattern; a second positional is a usage error, exit 2 |
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
| Status | as built in Go (Step 18) |
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
| Status | as built in Go (Step 19); byte-identical to the bash on a copy of a real journal over 33 `new-day`/`new-week` runs across a month and a year boundary, and on 270 synthetic journals (D34) |
| Legacy | `daisy rotate` → the bash `rotate` script, deleted in Step 19 (also run by `new-day` and `new-week`, in-process in Go) |
| Usage | `daisy rotate` (no argument) |
| Exit codes | 0, also when `journal.md` is missing, empty or holds five day-blocks or fewer (nothing written, nothing printed) [83]; 1 when `journal.md` has non-blank content before its first `### YYYY-MM-DD` heading (`Error: rotate: journal.md has non-blank content before its first '### YYYY-MM-DD' heading — aborting, nothing written` on stderr [83]), when the month-to-date or year-to-date window holds no day-block (`Error: rotate: <file> has no '### YYYY-MM-DD' day-block …`; the bash died part-way there, with the last-14 window already written), or when a block would move and a rolling window is not a regular file (`Error: rotate: <file> is not a regular file …`; the bash appended through a link, then replaced it with a file). Nothing is written on either refusal [83] |
| Stdout | `✅ Rotated N day-block(s); journal.md retains its last 5` when anything moved [83]; nothing otherwise [83] |
| Ops | Real |
| Writes | `journal/journal.md` keeps its last five `### YYYY-MM-DD` blocks [81, 83]; a block runs to the next such heading, less its trailing blank lines and one trailing `---`, so a preamble before a heading (new-week's Weekly Retrospective) stays with the block before it. Each older block, oldest first, is appended to all three rolling files — `journal-<YYYYMMDD>-last14.md` (which then drops its oldest block beyond 14 and takes the date of its new first block), `journal-<YYYYMMDD>-mtd.md` and `journal-<YYYYMMDD>-ytd.md`, each named by the full date of its first block [83] — and a month or year window whose first block is of another period than the incoming one closes first into `journal-YYYYMM.md` / `journal-YYYY.md` [83] (replacing a file of that name). So every rotated block is in exactly one month file and one year file, and in the last-14 window while it is among the fourteen most recently rotated (D34 restates Step 19's "exactly one window file" so) *(decided, D34, Step 19)*. Each window file carries `# <label>`, `## Table of Contents` (one date per block), `## Summary` (`N entries, <first> to <last>.` [83]), `---`, then its blocks separated by `---` |
| Commits | none of its own [83] (`new-day`/`new-week` commit) |
| Cases | 83 |

## Plans

### daisy plan new

| Field | Value |
|---|---|
| Status | ported (Step 22): Go (`internal/plan`); byte-identical to the bash's output and plan file but where the bash was wrong (D37) |
| Legacy | `plan` and `new` as one hyphenated word → its bash script, deleted in Step 22 |
| Usage | `daisy plan new [--spec] <description> [+project]` |
| Arguments | `<description>`: required — none, or only empty words, is usage on stderr, exit 2 (the bash exited 1 with `Error: Description required`); slugged (every run of bytes other than ASCII letters and digits → one `-`, trimmed, cut to at most 100 bytes and trimmed again, `fix-four-defects` [55] stays readable; the bash's `sed` kept the locale's letters, so `café` stayed `café` and 100 characters could be 200 bytes, and slugged each line of a multi-line description apart — Go's slug is ASCII whatever the locale, and a newline is one more hyphen); taken literally into the H1 (the bash's `awk -v` turned `\t` and `\\` into a tab and a backslash); `[+project]`: leading `+` optional, stripped; a third word is usage, exit 2 (the bash ignored it) |
| Exit codes | 0 [55]; 1 when `PLAN.md` already exists — as a dangling link too (`Error: PLAN.md already exists in this workspace`; the bash wrote the plan file, then failed at `ln`) —, or with `--spec` when `<slug>_PLAN.md` does, when the slug is empty (`Error: description produced an empty slug`), when the project holds `/` or a NUL (`cannot be part of a file name`; the bash failed at the write) — with `--spec` too, where it names no file yet but `plan pickup` would refuse it, when the plan file already exists (two plans named in the same second; the bash overwrote it), when `templates/plan.md` is missing, when `.gitignore` cannot be read, or when the filename would reach 255 bytes: `the limit is 255` [55] and `Shorten the project tag` [55] on stderr, never the shell's `File name too long` [55]. Every refusal is decided before the first write; a failed write takes back the plan file and link already made (`Nothing was created.`) |
| Stdout | `  ✓ Added PLAN.md to .gitignore` / `  ✓ Created .gitignore with PLAN.md` when it was not ignored, then `✅ Plan created: <filename>`, `   Linked as: <ws>/PLAN.md`; `--spec`: the `*_PLAN.md` gitignore line, then `✅ Spec created: <slug>_PLAN.md`, `   Location: <path>`, `   Not registered with Daisy — promote later with: daisy plan pickup <path>` |
| Ops | Real |
| Writes | `$DAISY_HOME/plans/<yymmddHHMMSS>.<project>.<slug>.plan.md` (no project: `<ts>.<slug>.plan.md`) from `templates/plan.md`, line by line: the `{Description}{project_suffix}` line becomes `# <description>` (` — +<project>` added when there is one), the `**Project:** +{project}` line names the project or goes, the first `{YYYY-MM-DD HH:MM}` on a line becomes the time, and on any other line every `$DAISY_ROOT` the root; the relative symlink `<ws>/PLAN.md` → `.daisy/plans/<filename>`; `<ws>/.gitignore` gains a `PLAN.md` line (on a line of its own: the bash appended it to a last line without a newline), written through a link. `--spec`: `<ws>/<slug>_PLAN.md` only, `.gitignore` gains `*_PLAN.md`, nothing under the home |
| Commits | none (decided, D37: the plan file is committed by the next home commit, `plan pickup`'s and `plan archive`'s included) |
| Cases | 55 |

### daisy plan pickup

| Field | Value |
|---|---|
| Status | ported (Step 22): Go (`internal/plan`) |
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
| Status | ported (Step 22): Go (`internal/plan`) |
| Legacy | `plan` and `archive` as one hyphenated word → its bash script, deleted in Step 22 |
| Usage | `daisy plan archive` (any argument: usage, exit 2; the bash ignored them) |
| Exit codes | 0 [54]; 1 when `PLAN.md` does not exist, is not a symlink (`not a Daisy plan`), its target is not in `plans/` (`The symlink may be broken…`), `plans/archive/<filename>` already exists (the bash's `mv` replaced it), a project file cannot be read, or the home is not in its own git repository — each decided before the first write; 1 when a write fails, with the move, the link and every project file put back (`Nothing was archived.`, or which undo failed); a failed commit leaves the plan archived and says so |
| Stdout | `✅ Archived: <filename>`; `  ✓ Repointed links in N project file(s)` when any (`project file` [54]); then `commit`'s line |
| Ops | Real |
| Writes | moves `plans/<filename>` to `plans/archive/<filename>`, removes `<ws>/PLAN.md`; in every regular `projects/*.md` and `projects/_archive/*.md` (not dotfiles, as the glob) rewrites each literal `../plans/<filename>` to `../plans/archive/` [54] `<filename>`, written through a link (the bash's `sed -i` replaced a linked project file with a copy), leaving unrelated project files byte-identical [54] |
| Commits | home, once: `Archive plan: <filename>` |
| Cases | 54 |

## Homes and identities

### daisy home

| Field | Value |
|---|---|
| Status | ported (Step 21): Go, four rows of the `cli` table (`home` is a noun, D35) |
| Legacy | `daisy home [list\|add\|remove\|new]` → the bash `home` script, deleted in Step 21 |
| Usage | `daisy home list`, `daisy home add <name> <path>`, `daisy home remove <name>`, `daisy home new <name> [path]`; bare `daisy home` or another verb: exit 2 listing the four usages (bare `daisy home` was `home list` in bash) |
| Exit codes | `list`: 0 [68] (69 and 70 run it and assert stdout only). `add`: 0 registered [68]; 1 when the name is `already registered` [68], invalid (`bad=name`, case 68), the path is not a directory [68], has a path problem (`Error: <path> <problem>`: the root itself — `cannot be a home` [68] — or a directory containing it, `contains the Daisy root; …`, which case 68 pins through a registry line and `status`, not through `home add`; the path is already resolved here, so the other two problems cannot arise), has no `include.txt` [68], is `not in its own git repository` [70] (or the root's), or nothing under it can be staged. `remove`: 0 [68]; 1 for an unregistered name [68]. `new`: 0 created [69]; 1 when the name is registered (nothing written, case 69) or the location `already exists` [69]; 1 when any step after the first write fails — the copy is removed: `was removed and nothing is registered` [70] on stderr; 1 when a prompt gets EOF |
| Stdout | `list`: `%-14s %s` name and path per home (`testhome` [68] and its path [68, 69]; ` (missing)` when the directory is gone); no homes: `No homes registered in <registry>` on stderr, exit 0. `add`: `✓ Registered home '<name>' at <path>`. `remove`: `✓ Unregistered home '<name>' (its files at <path> are untouched)`. `new`: the prompts `  Location [<default>]: ` (only without `[path]`), `Git identity for home '<name>':`, `  Name: `, `  Email: ` through `Ops.Prompt` (the questions on stderr); `  ✓ Saved git identity to <path>/gitconfig` or `  ⚠ Skipped (both name and email required)…`; `  ✓ Initialized git repository with first commit`; `✅ Created home: <name>`, `   Location: <path>`, `📋 Next steps:` … `daisy init <name>` |
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
| Status | built (Step 21) |
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
| Status | ported (Step 20): Go, renamed from `build` (no alias) |
| Legacy | `daisy build [home]` → the bash `build-prompt` script, deleted in Step 20 |
| Usage | `daisy prompt build [<home>]`: with `<home>` builds `<home>/AGENTS.md` (no workspace needed); without, builds the current workspace's `.daisy/AGENTS.md` for its bound home. (`--output`, which the bash `init` passed, went with it in Step 21; `home new` builds in-process and `optimize` passes the home's name.) `daisy prompt` alone or with another verb is a usage error listing the `prompt` commands |
| Exit codes | 0; 1 for an unregistered or unusable home (the Home resolution messages), outside a workspace without `<home>`, a missing `include.txt` (`Error: <home>/include.txt not found` and `Create include.txt in your home directory listing prompts to include`), an `include.txt` line with an unmatched quote, a prompt that cannot be read, or an output that cannot be written; nothing is written then; 2 for a surplus argument |
| Stdout | `✅ Built AGENTS.md for home: <home>`, `   Full: <names>`, `   Lazy: <names>` (each only when not empty), `   Output: <file>`, `   Size: N lines` |
| Stderr | `Warning: Prompt not found: <path>` (the root's path, the last looked in; an error block is written into the output and the build continues); `Warning: No ## Trigger section found in <path>` and `  Include as full prompt instead, or add a ## Trigger section` (included in full, counted as Full) |
| Ops | Real |
| Writes | the output file only (mode 0644, replaced atomically): a header — `# Daisy Prompt - Auto-generated`, `# Home: <home>`, `# Built: <YYYY-MM-DD HH:MM TZ>`, `# Source: <home>/include.txt`, `#`, `# This file is generated by daisy prompt build`, `# Output: <file>`, `# Do not edit directly - edit source prompts in prompts/ directory`, `# To rebuild: <command>` (`daisy prompt build <home>` for a home's file; `daisy prompt build`, run in the workspace, for any other), a blank line — then per `include.txt` entry a banner and the prompt: whole for `name`, the `## Trigger` section (the lines up to the next line starting `#`, trailing blank lines dropped) for `~name`, resolved in `<home>/prompts/` then `$DAISY_ROOT/prompts/`, every literal `$DAISY_ROOT` replaced by the root. `include.txt`: blank lines and lines whose first non-space is `#` skipped; each other line's words read as `xargs` reads them (blanks collapsed, quotes and backslashes honoured) and a leading `~` making it lazy. Byte for byte the bash's output except the `Built:`, `This file is generated by` and `To rebuild:` lines (Step 20, D35) |
| Commits | none |
| Cases | 84; every `init` case (32, 33, 76–79) and `home new` (69) build through it |

### daisy install

| Field | Value |
|---|---|
| Status | ported (Step 21): Go; non-interactive in both forms (the bash asked for a default home and wrote `DAISY_DEFAULT_HOME`, which nothing reachable through `daisy` reads) |
| Legacy | `daisy install [--update]` → built in to `daisy.sh`, deleted in Step 21 |
| Usage | `daisy install [--update]`: `--update` re-applies the symlink, root skills and permissions; plain `install` also writes the shell rc line and refuses when already installed (`Error: Daisy is already installed.`, exit 1). The root is `$DAISY_ROOT`, else the root whose `bin/daisy` is running (`Ops.Executable()`, links resolved), so `<root>/bin/daisy install` bootstraps a shell with nothing set. It does not build: `make build` writes the binary, and `install` warns when it is not built yet |
| Exit codes | 0 [31]; 1 already installed, `~/bin/daisy` exists and is not a symlink, `~/.claude/settings.json` is not a JSON object (decided before any write), `HOME` unset, no `DAISY_ROOT` and the binary not under a root's `bin/` |
| Stdout | `Installing Daisy...` / `Updating Daisy...`, `  DAISY_ROOT: <root>`, ✓ lines per step, `  ✓ Installed N skill(s) into <dir>`, `  ✓ Pruned N stale skill(s) from <dir>` (case 31: a `$HOME/.claude/skills/<name>` with no source under `skills/` is removed and current skills are installed), `  Shell config: <rc>` and `  ✓ Added DAISY_ROOT to <rc>` / `  ✓ DAISY_ROOT already set correctly` / `  ✓ Updated DAISY_ROOT (was: <old>)`, `Done.` / `Done. To activate, run:` … `daisy init <home>` (or `daisy home new <name>` first when none is registered) |
| Ops | Real |
| Writes | `$HOME/bin/daisy` symlink; `$HOME/.claude/skills/<name>/` for every `SKILL.md` found under `skills/` (copies, named by the containing directory; stale ones pruned); `$HOME/.claude/settings.json` allow rules (`Read(/$DAISY_ROOT/**)`, `Edit(/$DAISY_ROOT/**)` — the leading slash plus the absolute root gives the `//`-anchored form — and `Bash(daisy:*)`; the legacy `Bash(…/daisy/scripts/*)`, single-slash and triple-slash rules are removed); without `--update`: the rc file by `$SHELL` (`~/.zshenv`, `~/.bashrc`, `~/.config/fish/config.fish`, else `~/.profile`): the `DAISY_ROOT` line replaced in place, or `# daisy` and the line appended; a file kept as a symlink is written through. Applying twice equals once |
| Commits | none |
| Cases | 31; `TestInstall`, `TestInstallBootstrapsFromTheBinary` |

### daisy feedback

| Field | Value |
|---|---|
| Status | as built in Go (Step 23); the bash's bytes but where D38 says |
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
| Status | as built in Go (Step 23): the redesign agreed in D12 — no API key; the briefing and `apply` |
| Legacy | `daisy optimize [--workflow <n>] [--dry-run] [--force] [--list]` → `optimize.sh`, deleted in Step 23 with the 2-entry floor, `--force`, `--dry-run`, `--list`, `EDITOR` and the `y/N/e` prompt |
| Usage | `daisy optimize [--workflow <n>]` prints the briefing; `daisy optimize apply --workflow <n> (--rules\|--rewrite) <file> [--yes]` applies text the agent proposed (`-w` is `--workflow`; `<file>` is the working directory's when relative) |
| Targets | entries group by the file their `**Workflow:**` tag resolves to (as `feedback` resolves it), in order of first appearance; two tags naming one file are one target (`[prompts/plan] 2 entries` [38]); an unheadered entry is `prompts/daisy`'s (`[prompts/daisy] 1 entry` [41]); `--workflow <n>` scopes to the target `<n>` names (`[prompts/plan] 1 entry` [40]; one naming nothing is refused, exit 1 [40]); a skill resolves to its source `SKILL.md` (`Target: <home>/skills/widgetcraft/SKILL.md` [49]); a tag naming no file is a group of its own, briefed with the reason (`[daisy-write] 1 entry` [67], `names no prompt or skill` [67]) and never defaulted to daisy.md |
| Rules | the `## Rules` section runs from its heading (`## Rules`, or `## Rules` and more after a space — `skills/tdd`'s `## Rules of the loop` — as the bash's `/^## Rules/` read it, bounded at the word; closing `#`s allowed) to the next `#` or `##` heading; an `###` is inside it [53]; Daisy reads ATX headings, LF or CRLF line ends (up to three spaces, one to six `#`, then a space, a tab or the line's end) and fenced code (three or more backticks or tildes, a backtick fence's info string holding no backtick — "```x``` …" is inline code — closed only by a run of the same character at least as long, then only spaces or tabs); a line in a fence is no heading. Other markdown (setext headings, containers, HTML blocks) is not read: where a section, the target's or a proposal's, holds a line of only `=` or `-` straight after a non-blank line other than a heading (to another reader, perhaps a setext underline; a list item above is no exception), `apply --rules` refuses, naming the line and the file, rather than guess where the section ends; a blank line above it makes it a thematic break. So does a fence opened in the target's section and never closed (the section would run to the end of the file), or, without a section, a fence left open at the end of the file when no `## ` heading other than the Trigger precedes it (the rules would be appended inside it); the briefing prints the refusal under the target (`--rules refused: …`) (D38). A rule is a list item at the left margin (`N. `, `- `, `* `) outside a fence (`Rules: 2` [53]). `daisy help <skill>` prints the same section |
| Briefing | under `ReadOnly` [57]. `Pending feedback: N entries for N targets (<feedback.md>)` [38], or `Nothing pending in <feedback.md>` [56]; per target `[<label>] N entries` [38], `Target: <path>` [38], `Rules: N` [39, 56] (`Rules: N — more than 10: rewrite flagged` [56]; `Rules: none …`), `--- entries ---` and each entry as written, `--- current ## Rules ---` and the section [39, 53], `--- end ---`; a vendored skill (carries `LICENSE`/`NOTICE.md`) is listed with the reason (`vendored` [50]) and its entries stay pending; then `Rewrite flagged (more than 10 rules, nothing pending):` and `[<label>] rewrite flagged: N rules` [56] for every active target over 10 rules (the prompts `include.txt` names, in its order, then every skill source by name; 10 is not flagged [56]); then how to apply (`daisy optimize apply --workflow <label> --rules` [39], `--rewrite` [56]). No floor: one entry is briefed (`Skipped` [39] and `floor` [39] absent) |
| Apply | `--rules <file>`: the file must be one `## Rules` section — its first non-blank line the heading (`must begin with` [61]), no other `#`/`##` heading outside a fence (named with its line, `## Another Section` [61]), no fence left open; and the result must read back as the section written, so nothing after it is swallowed; it replaces the target's section, keeping the blank lines between the section and the next heading as they were, so replacing the rules with themselves changes nothing, byte for byte [53]; `# Document Title` [53, 58], `## Next Section` [53] and `Body text` [53, 58] survive; a target without a section gets one before its first `## ` heading other than the Trigger, or at the end. `--rewrite <file>`: the file, byte for byte [60]; an empty one is refused. Both print the unified diff (`-1. Measure before cutting.` [58], `+2. Cut once.` [58]) and ask `Apply these changes to <label>? [y/N] ` [58] on stderr through `Ops.Prompt`, unless `--yes` (never asks [60]). `y`/`yes` (any case) applies; anything else is `Cancelled` [59]; EOF is `Cancelled — no answer on stdin (EOF); nothing written.` (`EOF` [64]); any other failure to read the answer is `Error: cannot read the answer: …`, exit 1, nothing written. The writes are computed from what was read before the question: if the target, `feedback.md` or `archive.md` changed while it waited (an entry recorded meanwhile), the apply is refused, exit 1, nothing written. Applied: the target (`  ✓ Updated ` [58] `<path>`), the target's pending entries — every tag naming it [58] — moved from `feedback.md` to `archive.md`, each followed one blank line down by `**Addressed by:** `` `daisy optimize` `` <YYYY-MM-DD> — `` `<target>` `` (rules\|rewrite)` [63], `<target>` relative to the home or `$DAISY_ROOT/…` [62]; `feedback.md` keeps its header and the other entries, one blank line apart, byte for byte [66]; `feedback/.last-optimized` gets `<YYYY-MM-DD HHMM> <label>` [63]; for a prompt, the home's `AGENTS.md` and the workspace's `.daisy/AGENTS.md` are rebuilt from the prompts as written (for a skill, the run says `daisy install --update` [49] and `daisy init` propagate it); the home is committed once [58, 60, 62]. A root target is written and left uncommitted [62], and the run says to run `daisy healthcheck --force` [62] and then `daisy commit --not-home` [62] (`--root` after Step 24). Applying text already there with nothing pending: `Nothing to apply: …`, exit 0 |
| Exit codes | 0 (nothing pending, a briefing, an applied, declined or EOF apply, nothing to apply); 1 when `feedback/feedback.md` does not exist (`Error: No feedback file found at <path>` and the `mkdir -p … && touch …` that makes it), a target the briefing cannot read (printed in its place, the errors after the briefing, `The briefing above is incomplete.`), a file changed during the question, `archive.md` absent with no template to start it from when entries are to move, a workflow naming no file [40, 61] (`names no prompt or skill`), a vendored target on `apply` [50], a proposal that cannot be read (`cannot read` [61]) or is not one rules section [61], the home not a repository, a write or the AGENTS.md build failing (every write taken back: `Nothing was applied.`), the commit failing (the files written, the `daisy commit --home` to run printed), a target `--rules` cannot write (the open-fence and setext refusals of the Rules row, naming the target); 2 for a usage error [57, 61]: `--dry-run`, `--force`, `--list` [57] or any argument to the briefing, bare `optimize apply` [61], no `--workflow`, neither or both of `--rules`/`--rewrite`, a value flag without a value (an empty word, or one of the command's own flags, is none) [61], a stray word [61] |
| Ops | briefing `ReadOnly` [57]; `apply` Real |
| Writes | `apply` only, every refusal before the first write and a failed write taking back the ones before it: the target prompt or `SKILL.md`, `feedback/feedback.md`, `feedback/archive.md` (started from `templates/home/feedback/archive.md` when absent; no template is an error when entries are to move), `feedback/.last-optimized` (`healthcheck` Check 7 reads it), the home's `AGENTS.md` and the workspace's `.daisy/AGENTS.md`. The suite's fixture copies `prompts/` and `skills/` so an applying run writes the fixture's copy, never the repository's (case 65) |
| Commits | `apply`: the home, once, `Optimize <label> (rules\|rewrite)` [58]; never the root [58, 62] |
| Cases | 38, 39, 40, 41, 49, 50, 53, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66; `TestOptimizeApplyApproval`, `TestOptimizeApplyArchivesAndCommitsOnce`, `TestOptimizeApplyRollsBack`, `TestOptimizeUsage`, `TestOptimizeBriefing` |

### daisy eval

| Field | Value |
|---|---|
| Status | as built in Go (Step 23); the bash's bytes but where D38 says |
| Legacy | `daisy eval [<case>] [--record]` → `eval.sh`, deleted in Step 23 |
| Usage | `daisy eval` lists the cases in `$DAISY_ROOT/prompts/evals/*.md`; `daisy eval <case>` prints one; `daisy eval --record <case> pass\|fail ["notes"]` records a result (the agreed help line `eval [<case>] [--record]` abbreviates this form; `--record` leads). With no notes and a terminal, asks `Notes (optional): ` through `Ops.Prompt` |
| Exit codes | 0; 1 for a missing evals directory, an unknown case (`Error: Eval case not found: <case>`; a name holding `/` is none), a result other than `pass`/`fail`, `--record` without a case (its usage line); 2 for a second word without `--record` or a fourth with it |
| Stdout | list: `No eval cases found in <dir>` (exit 0) or `Available eval cases:`, `  %-20s  <H1 title>` per regular `*.md` file in byte order, the two usage lines; one case: the file, then `  Record result: daisy eval --record <case> pass\|fail "notes"`; record: `  ✓ Recorded <result> for eval '<case>' (<YYYY-MM-DD HH:MM>)`, `  File: <path>` |
| Ops | Real (decided, D30: one wrapper per command; `--record` writes) |
| Writes | `--record`: `$DAISY_HOME/feedback/eval/results.md` (created with a header), appending `## <YYYY-MM-DD HHMM>`, `**Case:**`, `**Result:**`, the notes |
| Commits | none (decided, D38, as `feedback`) |
| Cases | none; `TestEval` |

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
