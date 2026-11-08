# Daisy — the root, for agents

This file is for work on the root itself: the Go binary, the prompts, the
skills, the templates and the documentation. Cursor and Claude Code apply
it when a file in this directory is edited. The user-facing workflows
(tasks, journal, projects, plans) are `prompts/daisy.md`, loaded through
the generated `AGENTS.md` of each workspace; `README.md` is the account of
what Daisy is for and why it is shaped as it is.

Four words have fixed senses here. The **root** is this repository,
installed at `$DAISY_ROOT`, public as it is. A **home** is one context's
data and configuration, its own git repository, found by name through the
registry `$DAISY_CONFIG/homes` (default `~/.config/daisy/homes`). A
**workspace** is a directory bound to a home by `daisy init`, through its
`.daisy/` directory. An **identity** is the git author a workspace's own
commits carry, defined in a home (`gitconfig`, or `identities/<id>`), and
naming one at `init` chooses the home as well. `docs/home-management.md`
has the rules; `env.Resolve` in `internal/env` is the code.

## Layout and the dependency rule

`daisy` is a Go binary (`go 1.27`, standard library only); no bash under
the root runs but the acceptance suite (`tests/`), and no Go file sits at
the root. `cmd/daisy/main.go` is `os.Exit(cli.Run(ops.Real(), …))` and
nothing else; `cmd/daisy` also holds the functional tests, which call the
same `cli.Run` with an `ops.Fake` built around a temp machine. Every
package's tests run under `internal/testenv`, which pins what the git they
start reads: `HOME`, `USER`, no global git config.

| Package | Holds |
|---|---|
| `internal/ops` | every side effect in one value: `Real`, `ReadOnly` (refuses every write; since the swarm build it lets a forge `GET`/`HEAD` and `docker ps` through for the read-only commands), `Fake` |
| `internal/env` | root, registry, workspace, home, identity and projects resolution; `.env.sh` read without being run; the homes a root commit leaves out (`RootExclusions`) |
| `internal/cli` | `Run` and the command table (`internal/cli/table.go`), from which the help, `docs/cli.md`'s tree and the Command Reference below are rendered |
| `internal/commands` | one function per command |
| `internal/todo`, `internal/journal`, `internal/gitx`, `internal/prompt`, `internal/skill`, `internal/identity`, `internal/settings`, `internal/workspace`, `internal/plan`, `internal/feedback` | the domain packages behind them; `internal/gitx` makes the one commit every command that changes an existing home ends in (`plan new`, `feedback`, `eval --record` and a hand-run `rotate` leave their writes to the next commit) |
| `internal/health` | `daisy healthcheck`: the checks, the TTL stamp and the publication scan |
| `internal/swarm/declare` | `swarm.json`, its validation and `init`'s wizard |
| `internal/swarm/forge` | the forge seam: `Adapter`, `gitea`, `Fake` |
| `internal/swarm/provider` | each agent CLI's argv and what its output reports |
| `internal/swarm/runner` | the containers: `Docker`, the login slots, `Fake` |
| `internal/swarm/reconcile` | the trigger rule, the prompt files, host-side git and `apply`'s pass |
| `internal/swarm/brief` | the briefs: `Status:` lines and milestone headings |

**Dependency rule:** `cli → commands → {domain packages} → ops`; nothing
above `ops` calls `os`, `os/exec` or `time.Now` itself. A read-only command
runs under `ReadOnly`, so it cannot write by accident; `docs/cli.md` names
each command's wrapper. Each command's `--healthcheck` hook is a `Health`
function in the table; `daisy healthcheck` runs them in-process as its
`Component:` lines, and `log`, `done`, `new-day` and `new-week` run the
whole check in-process before they write.

**Build and gates.** `make build` writes `bin/daisy` (gitignored); `daisy
install` links `~/bin/daisy` to it and does not build. `daisy test` builds,
runs `go test ./...`, then drives the acceptance suite through the binary
(`make test` is `daisy test`); the suite refuses a `daisy` on `PATH` that
is not this root's `bin/daisy`, so a stale link cannot run another
checkout's code against its cases. `make check` (gofmt, vet, `go mod tidy
-diff`, `go test -race`) runs before every root commit, and so does `daisy
healthcheck --force` against this root (`PATH="$PWD/bin:$PATH"
DAISY_ROOT="$PWD" daisy healthcheck --force` from the root). No test runs
through the real environment: a test builds its own machine (temp root,
home, registry, workspace) and pins every variable the program reads; no
step, live run, fixture or probe writes a real home (`prompts/praxis.md`
rules 14–15).

## Publication Hygiene

The root is public: pushing it to its remote publishes it. Every file git
would keep in it, tracked or untracked but not ignored, is published
verbatim; there is no mirror, no copying step and no sanitising pass.
Treat every system file (code, prompts, docs, templates, skills, tests,
dotfiles) as already public. Personal data lives in the homes, each its
own repository, never the root's, which Daisy refuses; untracked files
under a registered home placed inside the root's directory are left out
of root commits and of the scan alike.

**The hygiene scan is the gate.** `daisy healthcheck` scans the
publishable surface — `git ls-files --cached --others --exclude-standard`
in `$DAISY_ROOT`, an untracked file under a registered home inside the
root excepted (`env.RootExclusions`, the list a root commit leaves out); a
file is its content, a symlink its link text (what git publishes), every
path its name, and a file staged differently from the worktree its staged
blob — for identifying terms derived from the machine at run time, never
from a hardcoded list, which would itself leak: every registered home's
name, `$USER`, the git author (`user.name`, `user.email`) configured in
the root's repository and in each usable home's, and literal `$HOME/`
paths. A term
is matched without case, as a whole word at each of its edges that is a
letter, digit or `_` (so `$HOME/` is found inside a longer path). Run
`daisy healthcheck --force` before every root commit: it is the only gate
between the tree and the public remote, and a cached pass reflects the
tree as it was. The scan is a net, not the standard: it cannot catch
personal *content* that contains no known term, and it says nothing about
a file it considers unpublishable. Write system files as if they were
already public.

**`.gitignore` is the publication boundary.** It is the single declaration
of what does not ship, honoured by the scan and by git alike. Every line
in it names its cause, what writes that path under the root: the binary
`make build` writes (`bin/`), OS artifacts, secrets (`.env.sh`, which
`check-secrets` reads in a workspace), and the block `daisy init` writes
when the root is itself bound as a workspace (`.daisy/`, its stage and
previous names, `.cursorignore`, `.claude/settings.local.json`,
`.claude/commands/daisy.md`, `.agents/skills/daisy/SKILL.md`,
`.cursor/rules/daisy*.mdc`, `PLAN.md`, and the bound home's skills): files
that sit inside `$DAISY_ROOT` and would otherwise publish. A line without
a cause goes. Two consequences:

- Keep ignore rules in `.gitignore`. A global `core.excludesFile` or a
  `.git/info/exclude` entry hides a file from the scan on this machine
  only, and a clone elsewhere would commit it: the one divergence that
  turns the check into a false negative.
- Adding a per-machine artifact to `$DAISY_ROOT` means adding it to
  `.gitignore` in the same change. Until then it is public.

**Never write into system files:** real home names, the OS username, real
people's names or emails, employers, forge hosts, the names of the
products a swarm works on, or absolute paths under `$HOME`.

**Use the standard substitutes:**
- `$DAISY_ROOT`-relative paths, `~` or `$HOME`; never `/home/{user}/...`
- Placeholder home names: `work`, `personal`, `example` in docs;
  `testhome` in test fixtures (the scan exempts these four)
- `Jane Doe <jane@example.net>` for identities in examples
- `<forge-host>`, `<owner>/<repo>` and `<ws>` for a forge, a repository
  and a workspace, as `docs/swarm.md` uses them
- `~alias` notation for people

Real names belong only in a home (`gitconfig`, `identities/`, the data),
in `~/.config/daisy/` and in git config: never in the root's own content,
which is everything a root commit takes.

## Data formats

Daisy's data files have exact formats that the binary parses and the
prompts edit. When modifying any of them, preserve the format exactly.

**Todo.txt** (`docs/todotxt.md`, `docs/task-format.md`):
- Active: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Completed: `x YYYY-MM-DD YYYY-MM-DD Description +Project @context` (no priority)
- Cancelled: `z YYYY-MM-DD YYYY-MM-DD Description +Project @context` (no priority)
- Never change the date format (always `YYYY-MM-DD`); never add or remove
  the priority parentheses on an active task; never keep a priority on a
  completed or cancelled task; never reorder fields; preserve every space.

**Today.md** (`docs/templates.md`):
- Preserve the heading levels (`####`) and the checkbox forms `- [ ]`,
  `- [x]`, `- [z]` (note the spaces); never add or remove blank lines
  between sections.
- Time is `HHMM` (24-hour, no colon); a log entry is `- HHMM message`
  (one dash, the time, one space, the message; no second dash).

**Journal.md** (`docs/logging.md`): append daily entries; abridge only
during the weekly review; it may be modified only to consolidate quiet
days, update `~alias` forms or fix ordering errors; never delete
stakeholder interactions, task progress or retrospective content.

## Command Reference

Generated from the `cli` table: edit the table in `internal/cli/table.go`,
then replace the block below with the output of `daisy help --markdown`.
`daisy healthcheck` warns when the two differ, and `go test` fails.

<!-- BEGIN command reference: generated by `daisy help --markdown`; edit the cli table, not this block -->
| Group | Command | What it does |
|---|---|---|
| Workspace | `daisy init <identity> [path]` | Bind a workspace to an identity and its home |
| Workspace | `daisy clean [-f]` | Remove Daisy from this workspace |
| Workspace | `daisy status` | Home, identity, projects, today's counts, active plan |
| Workspace | `daisy files` | Resolved real paths for every home file |
| Workspace | `daisy list` | Active prompts and installed skills |
| Workspace | `daisy help [<command>\|<skill>\|--markdown]` | This help, detail for one command or skill, or the Command Reference |
| Day | `daisy new-day` | Start a new day |
| Day | `daisy new-week` | Start a new week |
| Day | `daisy log <message...>` | Add a log entry to today.md |
| Day | `daisy done <pattern>` | Mark a task complete |
| Day | `daisy tasks [--all] [--todo\|--done] [pattern]` | Search tasks (read-only) |
| Day | `daisy projects [--archived]` | List active or archived projects |
| Day | `daisy rotate` | Rotate journal.md into window files |
| Plans | `daisy plan new [--spec] <description> [+project]` | Create a plan (or an unregistered spec draft) |
| Plans | `daisy plan pickup <path>` | Promote a spec draft into a tracked plan |
| Plans | `daisy plan archive` | Archive the active plan |
| Homes and identities | `daisy home new <name> [path]` | Create a home; prompts for identity and location |
| Homes and identities | `daisy home add <name> <path>` | Register an existing home |
| Homes and identities | `daisy home list` | Registered homes and where they live |
| Homes and identities | `daisy home remove <name>` | Unregister a home; its files stay where they are |
| Homes and identities | `daisy identity list` | Identities across registered homes |
| Prompts and skills | `daisy prompt build [<home>]` | Rebuild AGENTS.md |
| Prompts and skills | `daisy install [--update]` | ~/bin/daisy, root skills, permissions |
| Prompts and skills | `daisy feedback [--workflow <n>] <text>` | Record a prompt failure for optimization |
| Prompts and skills | `daisy optimize [--workflow <n>]` | Briefing: pending entries per target, current rules, rewrite flags |
| Prompts and skills | `daisy optimize apply --workflow <n> (--rules\|--rewrite) <file> [--yes]` | Apply a proposal: diff, approve, archive, commit |
| Prompts and skills | `daisy eval [<case>] [--record]` | List, display, or record an eval case result |
| Swarm | `daisy swarm init [--defaults]` | Declare the swarm: swarm.json, prompts, labels |
| Swarm | `daisy swarm apply [--once]` | Reconcile the forge: launch members, post their output, move labels |
| Swarm | `daisy swarm status` | Stages, gates, what waits for you, running members (read-only) |
| Swarm | `daisy swarm pause` | Launch no new member until resumed; running ones are collected |
| Swarm | `daisy swarm resume` | Launch members again and release the owner's held batch |
| Swarm | `daisy swarm triage <n>` | As the maintainer: triage a proposed job to ready |
| Swarm | `daisy swarm accept <n>` | As the maintainer: accept the plan, planning to planned |
| Swarm | `daisy swarm approve <n>` | As the maintainer: approve the issue's open pull request |
| Swarm | `daisy swarm merge <n>` | As the maintainer: merge the approved pull request, then next slice or close |
| Swarm | `daisy swarm unblock <n> <comment...>` | As the maintainer: comment, then return a blocked issue to its prior stage |
| Swarm | `daisy swarm close <n> [comment...]` | As the maintainer: close the issue, after the comment when given |
| System | `daisy commit [--root] <message>` | Commit the home repo (or the root repo) |
| System | `daisy healthcheck [--force]` | Run the system health check |
| System | `daisy check-secrets` | Check which secrets/tokens are configured |
| System | `daisy test [filter]` | Run go test, then the hermetic acceptance suite |
<!-- END command reference -->

`docs/cli.md` is the contract behind each row: arguments, exit codes,
output, the `Ops` wrapper, writes and commits. A Go test
(`cmd/daisy/docs_test.go`) checks that every `daisy …` the documents name
is a row of the table with flags the table gives it, that every
repository path they name exists, and that `docs/cli.md`'s tree is `daisy
help` byte for byte.

Commands do not yet implement: priority floor rules, task preservation
from yesterday, quiet day consolidation, advanced sync validation, JIRA
sync, or log audit during retrospective. The agent does these by following
the prompts. After modifying commands or workflow logic, validate against
[`docs/test-cases.md`](docs/test-cases.md).

## Development Heuristics

**Prototype manually before automating.** When adding a new workflow or
modifying a complex command, do the operation by hand first: no commands,
no automation. The manual run reveals constraints and invariants that are
invisible in the abstract. Only after understanding what a correct manual
execution looks like should you codify it into a command or a prompt rule.
This holds most for multi-file operations (`new-day`, `new-week`) and
sync workflows.

**Read `prompts/praxis.md` before substantive work.** It carries the
rules this root is built by: read before inventing, scope as relatedness,
a claim per step that can fail (`skills/testing/SKILL.md`, with
`docs/testing.md` as its long form), a hostile review of each diff to a
stated gate, no silent failure, no dead code. Under a plan, the review
gate and the cap of five passes are `prompts/plan.md` rule 22; a step not
clean at the cap stops and hands back with its findings.

## The swarm, from an agent session

The swarm's code is `internal/swarm/*` and `internal/commands/swarm.go`;
its contract is `docs/swarm.md`, and `images/member/Dockerfile` with
`images/member/entrypoint.sh` is the member image. Three rules bind an
agent working near it:

- **`init` and `apply` are the maintainer's, from a terminal.** An agent
  session never runs them: the sandbox has no Docker socket, and the swarm
  must not depend on a chat being open. The `swarm` skill
  (`skills/swarm/SKILL.md`) is the agent's role: read `daisy swarm
  status`, the briefs and the runs; explain; draft; run a maintainer's
  verb only after the maintainer has confirmed that write in words.
- **Two tokens, two code paths.** The swarm's token (`forge.token_env` in
  `swarm.json`) is read by `init`, `apply` and `status`; the maintainer's
  (`DAISY_MAINTAINER_TOKEN`) by the six verbs only, which refuse when it
  equals the swarm's or is the swarm's account. Neither value is ever
  printed, written to a file Daisy makes, or passed into a container (a
  member gets exactly one credential, its provider's).
- **Daisy writes nowhere but the places `docs/swarm.md` section 2
  lists:** `<ws>/swarm/` at `init`, the state directory under
  `$DAISY_LOCAL/swarm/`, and the forge. Never a Daisy home, never the
  workspace outside `swarm/`, never a declared repository's tree.

The products a swarm works on, their repositories and the forge's host
are named only in the workspace that holds the swarm, never in the root
(Publication Hygiene).

## Prompts: the build system

`daisy prompt build <home>` generates a home's `AGENTS.md` from its
`include.txt`, resolving each prompt name against the home's `prompts/`
and then the root's `prompts/` (`internal/prompt`). Without a home it
builds the workspace's `.daisy/AGENTS.md` for the bound home, which `daisy
init` also writes.

`include.txt` is prompts-only. Skills (`skills/`) have no manifest: every
skill in the merged root and home set installs at `daisy init`; see
[`docs/prompts-vs-skills.md`](docs/prompts-vs-skills.md) for why these are
different systems.

### Lazy loading

**Full inclusion** (default): the entire prompt is embedded in
`AGENTS.md`.

**Lazy inclusion** (`~` prefix in `include.txt`): only the `## Trigger`
section is embedded, and the model reads the full file on demand when the
trigger condition is met. The build takes everything between `## Trigger`
and the next line starting with `#`; a prompt without a `## Trigger`
section is included in full, with a warning.

### Home overrides

Homes consume the root. A home file at a root prompt's path
(`<home>/prompts/<name>.md`, or `<home>/skills/<name>/SKILL.md`) overrides
it, in one of two forms (`internal/prompt/override.go`, D49):

- **Layered**: the file is the line `<!-- daisy: upstream body -->` and
  one `## Rules` section holding only the home's rules. The prompt is the
  root's file with those rules appended to its `## Rules` section,
  renumbered after the root's (a root without the section gets one), so
  upstream changes flow through. A home rule may refine a root rule, never
  delete one. `daisy optimize apply --rules` on a root target writes this
  form.
- **Full**: the file holds the whole prompt. `daisy optimize apply
  --rewrite` on a root target writes it and records the root file's hash
  in `<home>/feedback/.overrides`; `daisy healthcheck` warns when the root
  has changed since, and `daisy list` marks it `full, behind`.

One merge rule serves every reader: `prompt build` and `init` write every
layered prompt, merged, to the workspace's `.daisy/prompts/<name>.md`,
list each override at the top of `AGENTS.md`, and point every reference to
an overridden root prompt there (a full override's at the home's file);
`init` installs a layered skill as the root's directory with the merged
`SKILL.md`, in `.claude/skills/<name>/` and its twin
`.agents/skills/<name>/` (Codex's; one list of trees,
`skill.WorkspaceTrees`, beside the generated `.agents/skills/daisy/SKILL.md`,
Codex's way into Daisy); `help` and the `optimize` briefing show the
merged text. A root target is never written by `apply`.

### include.txt

```
# Full inclusion
personal

# Lazy inclusion (trigger only)
~daisy
~retrospective
~git
~plan
```

### Creating a new prompt

1. Create `prompts/{name}.md` with a `## Trigger` section as the first
   heading.
2. Add `~{name}` (lazy) or `{name}` (full) to the home's `include.txt`.
3. Rebuild: `daisy prompt build {home-name}`.

## Skills

Before creating either a prompt or a skill, apply the coupling criterion
in [`docs/prompts-vs-skills.md`](docs/prompts-vs-skills.md): does this
capability need to know the user's situation (projects, journal, tasks,
plan state), or only the artifact in front of it? Situated → prompt;
artifact-only → skill. Where a capability is a hybrid, find the seam at
which situational knowledge stops being needed and split there.

Placement does not decide ownership. A Daisy-authored skill stays
feedback-owned (the craft layer is where it runs, not who refines it), but
homes consume the root: the feedback loop writes only to the home, as an
override (see Home overrides above), and a root prompt or skill changes
only by the developer's own hand through `daisy commit --root`. Only
skills carrying `LICENSE`/`NOTICE.md` track an upstream source and must
stay outside the feedback loop's write path, overridden or not.

### Creating a new skill

1. Create `skills/{name}/SKILL.md` with `name`, `description` and
   `short_description` frontmatter (plus `disable-model-invocation: true`
   if it should only ever be invoked by name). `short_description` is a
   separate, ≤50-character, standalone summary that `daisy list` prints
   by default; `description` is never shortened, since Claude Code
   surfaces it verbatim for model-invocation matching.
2. No manifest entry is needed. Root skills (`skills/`) install to
   `~/.claude/skills/` via `daisy install`; home skills (the home's
   `skills/`) install to the workspace's `.claude/skills/` and its twin
   `.agents/skills/` (Codex's) via `daisy init`: separate targets, no
   install-time merge. `daisy list` consolidates both for display, with
   workspace and home entries overriding root and user entries of the
   same name.
3. Re-run `daisy install` (root skill) or `daisy init` (home skill) in any
   workspace that should pick it up: skills are copied, not symlinked, so
   this is the only way to refresh one.

### Adopting a skill from reference

Reference material staged at `docs/resources/mattpo-skills/{name}/` (or
any future reference source) is not a skill until adopted:

1. Copy the whole directory, `cp -r docs/resources/mattpo-skills/{name}
   skills/{name}`; this carries the source's `NOTICE.md` along.
2. Copy `LICENSE` alongside: `cp docs/resources/mattpo-skills/LICENSE
   skills/{name}/LICENSE`.
3. Fix the copied `NOTICE.md`'s license line from `../LICENSE` to
   `./LICENSE`: the original pointed at one shared file one level up, and
   once `LICENSE` is copied in as a sibling the old relative path dangles.
   `NOTICE.md` needs no other edits; it is the sole provenance record for
   an adopted skill, with no separate `attribution:` frontmatter.
4. Add `short_description` frontmatter to `SKILL.md` (above).
5. Re-run `daisy install` to deliver it.

The reference source itself is never modified or deleted by adoption: copy
*from* it, always.

## Reference Documentation

- [`docs/cli.md`](docs/cli.md) — The `daisy` CLI contract: the command tree as built, each command's arguments, flags, exit codes, asserted stdout/stderr, `Ops` wrapper, writes and commits; the renames table
- [`docs/swarm.md`](docs/swarm.md) — The swarm's contract (the eleven verbs built; the board, the GitHub adapter and the budget specified ahead of their build): `swarm.json`, the brief, the label lifecycle and trigger rule, the marker, the member container and its image, the credential rule, the provider and forge seams, the verbs (the maintainer's six among them, with the `swarm` skill as their guide), the merge gate
- [`docs/home-management.md`](docs/home-management.md) — Root and homes as separate repositories, the registry, identities, per-workspace resolution, the health check
- [`docs/prompts-vs-skills.md`](docs/prompts-vs-skills.md) — The situated/craft distinction, the coupling criterion, the ownership axis, worked examples
- [`docs/task-format.md`](docs/task-format.md) — Task format regex, conversion rules, file organization
- [`docs/task-sync.md`](docs/task-sync.md) — Bidirectional sync rules, validation algorithm
- [`docs/templates.md`](docs/templates.md) — Template placeholders and formatting rules
- [`docs/workflows.md`](docs/workflows.md) — All workflow algorithms (new-day, new-week, done, cancel, add, priority, sync, etc.)
- [`docs/logging.md`](docs/logging.md) — Logging design rationale, triggers, abridged archival
- [`docs/projects.md`](docs/projects.md) — Project management architecture and algorithms
- [`docs/praxis.md`](docs/praxis.md) — Implementation quality reference: how we work, anti-patterns, extended guidance
- [`docs/testing.md`](docs/testing.md) — The `testing` skill's long form: vocabulary, the literature, where the method departs, the design at two scales, one step worked
- [`docs/retrospective.md`](docs/retrospective.md) — Retrospective guide and integration workflows
- [`docs/plan.md`](docs/plan.md) — Plan workflow reference
- [`docs/github.md`](docs/github.md) — GitHub operations reference
- [`docs/examples.md`](docs/examples.md) — Complete interaction walkthroughs
- [`docs/todotxt.md`](docs/todotxt.md) — Todo.txt format specification
- [`docs/test-cases.md`](docs/test-cases.md) — Validation test cases
- [`docs/resources/prompt-learning-loop.md`](docs/resources/prompt-learning-loop.md) — Arize talk: feedback loops, rules-section optimization, eval co-evolution
- [`docs/resources/infinite-software-crisis.md`](docs/resources/infinite-software-crisis.md) — Netflix/Nations: simple vs easy, essential vs accidental complexity, three-phase approach
- [`docs/resources/progressive-disclosure.md`](docs/resources/progressive-disclosure.md) — Developers Digest: lazy-load context on demand, tools as files, memory as markdown; validates Daisy's lazy loading
- [`docs/resources/agents-in-complex-codebases.md`](docs/resources/agents-in-complex-codebases.md) — HumanLayer/Horthy: context quality over quantity, RPI calibration, step granularity, intentional compaction on repeated corrections

## See Also

- **`prompts/daisy.md`** — the user-facing workflows, loaded through each workspace's generated `AGENTS.md`
- **`README.md`** — goals, features, design decisions, usage, prompts vs. skills, recommendations
