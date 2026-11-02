# Daisy - System Architecture & Internal Specifications

This AGENTS.md is for **Mode 4: meta work** — modifying the Daisy system itself (the Go binary, prompts, templates). It is auto-applied by Cursor and Claude Code when editing files in this directory.

**For user-facing workflows (Modes 1–3), see `prompts/daisy.md` (loaded via per-home AGENTS.md).**

---

## CRITICAL: Format Preservation Rules

When modifying any Daisy data files, preserve EXACT formatting.

### Todo.txt Format

- Active: `(PRIORITY) YYYY-MM-DD Description +Project @context`
- Completed: `x YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- Cancelled: `z YYYY-MM-DD YYYY-MM-DD Description +Project @context` (NO priority)
- NEVER change date format (always `YYYY-MM-DD`)
- NEVER add/remove priority parentheses on active tasks
- NEVER preserve priority on completed/cancelled tasks
- NEVER reorder fields
- Preserve ALL spaces exactly

### Today.md Format

- Preserve markdown heading levels (`####`)
- Preserve checkbox format: `- [ ]`, `- [x]`, `- [z]` (note spaces)
- NEVER add/remove blank lines between sections
- Time format: `HHMM` (24-hour, no colons)
- Log entry format: `- HHMM - message` (note dashes and spaces)

### Journal.md Archive Rules

- Append daily entries; abridging ONLY during weekly review
- MAY be modified ONLY to: consolidate quiet days, update ~alias format, fix ordering errors
- NEVER delete stakeholder interactions, task progress, or retrospective content

---

## Publication Hygiene

The root repository is public: pushing it to its remote publishes it.
Every file git would keep in it — tracked, or untracked but not ignored —
is published verbatim: there is no mirror, no copying step and no
sanitization pass. Treat every system file — code, prompts, docs,
templates, skills, tests, dotfiles — as already public. Personal data lives
in the homes, each its own repository — never the root's, which Daisy
refuses; untracked files under a registered home placed inside the root's
directory are left out of root commits and of the scan alike.

**The hygiene scan is the gate.** `daisy healthcheck` scans the publishable
surface — `git ls-files --cached --others --exclude-standard` in
`$DAISY_ROOT`, an untracked file under a registered home placed inside the
root excepted (`env.RootExclusions`, the list a root commit leaves out); a file is its
content, a symlink its link text (what git publishes), every path its
name, and a file staged differently from the worktree its staged blob — for
identifying terms derived from the machine at runtime, never from a
hardcoded list, which would itself leak: every registered home's name,
`$USER`, the git identity (`user.name`, `user.email`) of the root's
repository and of each usable home's, and literal `$HOME/` paths. A term
is matched without case, as a whole word at each of its edges that is a
letter, digit or `_` (so `$HOME/` is found inside a longer path). Run `daisy healthcheck --force` before every root commit;
it is the only gate between the tree and the public remote, and a cached
pass reflects the tree as it was. The scan is a net, not the standard: it
cannot catch personal *content* that contains no known term, and it says
nothing about a file it considers unpublishable. Write system files as if
they were already public.

**`.gitignore` is the publication boundary.** It is the single declaration
of what does not ship, honoured by the scan and by git itself. Every line
in it names its cause — what writes that path under the root: the binary
`make build` writes (`bin/`), OS artifacts, secrets (`.env.sh`, which
`check-secrets` reads in a workspace), and the block `daisy init` writes
when the root is itself bound as a workspace (`.daisy/`, its stage and
previous names, `.cursorignore`, `.claude/settings.local.json`,
`.claude/commands/daisy.md`, `.cursor/rules/daisy*.mdc`, `PLAN.md`, and the
bound home's skills) — files that sit inside `$DAISY_ROOT` and would
otherwise publish. A line without a cause goes. Two consequences:

- Keep ignore rules in `.gitignore`. A global `core.excludesFile` or a
  `.git/info/exclude` entry hides a file from the scan on this machine only,
  and a clone elsewhere would commit it — the one divergence that turns the
  check into a false negative.
- Adding a per-machine artifact to `$DAISY_ROOT` means adding it to
  `.gitignore` in the same change. Until then it is public.

**Never write into system files:** real home names, the OS username, real
people's names or emails, employers, or absolute paths under `$HOME`.

**Use the standard substitutes:**
- `$DAISY_ROOT`-relative paths, `~` or `$HOME`; never `/home/{user}/...`
- Placeholder home names: `work`, `personal`, `example` in docs; `testhome`
  in test fixtures (the scan exempts these four)
- `Jane Doe <jane@example.net>` for identities in examples
- `~alias` notation for people

Real names belong only in a home — `gitconfig`, `identities/`, the data —
or in `~/.config/daisy/` and git config: never in the root's own content,
which is everything a root commit takes.

---

## The Go port

`daisy` is a Go binary; no bash under the root runs but the acceptance
suite (`tests/`). `cmd/daisy/main.go` is `os.Exit(daisy.Run(ops.Real(),
…))` and nothing else; `daisy.go` is `Run`; `internal/ops` is every side
effect in one value (`Real`, `ReadOnly`, `Fake`); `internal/env` resolves
root, registry, workspace, home, identity and projects, and reads
`.env.sh` without running it; `internal/cli` is the command table, from
which the help and the Command Reference below are rendered;
`internal/commands` is one function per command; the domain packages
behind them are `internal/todo`, `internal/journal`, `internal/gitx`,
`internal/prompt`, `internal/skill`, `internal/identity`,
`internal/settings`, `internal/workspace`, `internal/plan`,
`internal/feedback` and `internal/health` (the health check, its TTL
stamp and the publication scan). Dependency rule: `cli → commands →
{domain packages} → ops`; nothing above `ops` calls `os`, `os/exec` or
`time.Now` itself. Each command's `--healthcheck` hook is a `Health`
function in the table; `daisy healthcheck` runs them in-process as its
`Component:` lines, and `log`, `done`, `new-day` and `new-week` run the
whole check in-process before they write. `make build` writes `bin/daisy`
(gitignored); `daisy test` builds it, runs `go test ./...`, then drives
the suite through it (`make test` is `daisy test`); `make check` (gofmt,
vet, `go mod tidy -diff`, `go test -race`) runs before every root commit.

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
| System | `daisy commit [--root] <message>` | Commit the home repo (or the root repo) |
| System | `daisy healthcheck [--force]` | Run the system health check |
| System | `daisy check-secrets` | Check which secrets/tokens are configured |
| System | `daisy test [filter]` | Run go test, then the hermetic acceptance suite |
<!-- END command reference -->

`docs/cli.md` is the contract behind each row: arguments, exit codes,
output, the `Ops` wrapper, writes and commits.

Commands do NOT yet implement: priority floor rules, task preservation from yesterday, quiet day consolidation, advanced sync validation, JIRA sync, or log audit during retrospective. These are handled by the agent following prompt instructions.

After modifying commands or workflow logic, validate against [`docs/test-cases.md`](docs/test-cases.md).

---

## Development Heuristics

**Prototype manually before automating.** When adding a new workflow or modifying a complex command, do the operation by hand first — no commands, no automation. The manual run reveals hidden constraints and invariants that are invisible in the abstract. Only after understanding what a correct manual execution looks like should you codify it into a command or prompt rule. This is especially true for multi-file operations (new-day, new-week) and sync workflows.

---

## AGENTS.md Build System

`daisy prompt build <home>` generates a home's `AGENTS.md` from its `include.txt`, resolving each prompt name against the home's `prompts/` and then the root's `prompts/` (`internal/prompt`). Without a home it builds the workspace's `.daisy/AGENTS.md` for the bound home, which `daisy init` also writes.

Usage: `daisy prompt build [<home>]`

`include.txt` is prompts-only. Skills (`skills/`) have no manifest — every
skill in the merged root+home set always installs at `daisy init`; see
[`docs/prompts-vs-skills.md`](docs/prompts-vs-skills.md) for why
these are different systems.

### Lazy Loading Architecture

**Full inclusion** (default): entire prompt embedded in AGENTS.md.

**Lazy inclusion** (`~` prefix in include.txt): only the `## Trigger` section is embedded. The model reads the full file on demand when the trigger condition is met.

### include.txt Format

```
# Full inclusion
personal

# Lazy inclusion (trigger only)
~daisy
~retrospective
~git
~plan
```

### Creating a New Prompt

1. Create `prompts/{name}.md` with a `## Trigger` section as the first heading
2. Add `~{name}` (lazy) or `{name}` (full) to the home's `include.txt`
3. Rebuild: `daisy prompt build {home-name}`

The build extracts everything between `## Trigger` and the next line starting with `#` for lazy stubs. If no `## Trigger` is found, falls back to full inclusion with a warning.

### Creating a New Skill

Before creating either a prompt or a skill, apply the coupling criterion in
[`docs/prompts-vs-skills.md`](docs/prompts-vs-skills.md): does
this capability need to know the user's situation (projects, journal, tasks,
plan state), or only the artifact in front of it? Situated → prompt;
artifact-only → skill. Where a capability is a hybrid, find the seam at
which situational knowledge stops being needed and split there.

Placement does not decide ownership. A Daisy-authored skill stays
feedback-owned — the craft layer is where it runs, not who refines it. Only
skills carrying `LICENSE`/`NOTICE.md` track an upstream source and must stay
outside the feedback loop's write path.

1. Create `skills/{name}/SKILL.md` with `name`/`description`/
   `short_description` frontmatter (plus `disable-model-invocation: true` if
   it should only ever be invoked by name). `short_description` is a
   separate, ≤50 char, standalone-sufficient summary — `daisy list` prints
   it by default; `description` is never shortened, since Claude Code
   surfaces it verbatim for model-invocation matching.
2. No manifest entry needed. Root skills (`skills/`) install to
   `~/.claude/skills/` via `daisy install`; home skills (the home's
   `skills/`) install to the workspace's `.claude/skills/` via
   `daisy init` — separate targets, no install-time merge. `daisy list`
   consolidates both for display, with workspace/home entries overriding
   root/user entries of the same name.
3. Re-run `daisy install` (root skill) or `daisy init` (home skill) in any
   workspace that should pick it up — skills are copied, not symlinked, so
   this is the only way to refresh one

### Adopting a Skill from Reference

Reference material staged at `docs/resources/mattpo-skills/{name}/`
(or any future reference source) is not a skill until adopted:

1. Copy the whole directory — `cp -r docs/resources/mattpo-skills/{name}
   skills/{name}` — this carries the source's `NOTICE.md` along
   automatically.
2. Copy `LICENSE` alongside: `cp docs/resources/mattpo-skills/LICENSE
   skills/{name}/LICENSE`.
3. Fix the copied `NOTICE.md`'s license line from `../LICENSE` to
   `./LICENSE` — the original pointed at one shared file one level up;
   once `LICENSE` is copied in as a same-level sibling, the old relative
   path dangles. `NOTICE.md` needs no other edits — it's the sole
   provenance record for an adopted skill, no separate `attribution:`
   frontmatter.
4. Add `short_description` frontmatter to `SKILL.md` (see above).
5. Re-run `daisy install` to deliver it.

The reference source itself is never modified or deleted by adoption — copy
*from* it, always.

---

## Reference Documentation

- [`docs/cli.md`](docs/cli.md) — The `daisy` CLI contract: the agreed command tree, each command's arguments, flags, exit codes, asserted stdout/stderr, `Ops` wrapper, writes and commits; the renames table
- [`docs/prompts-vs-skills.md`](docs/prompts-vs-skills.md) — The hat/skill distinction, the ownership-of-evolution criterion, worked examples
- [`docs/task-format.md`](docs/task-format.md) — Task format regex, conversion rules, file organization
- [`docs/task-sync.md`](docs/task-sync.md) — Bidirectional sync rules, validation algorithm
- [`docs/templates.md`](docs/templates.md) — Template placeholders and formatting rules
- [`docs/workflows.md`](docs/workflows.md) — All workflow algorithms (new-day, new-week, done, cancel, add, priority, sync, etc.)
- [`docs/logging.md`](docs/logging.md) — Logging design rationale, triggers, abridged archival
- [`docs/projects.md`](docs/projects.md) — Project management architecture and algorithms
- [`docs/praxis.md`](docs/praxis.md) — Implementation quality reference: how we work, anti-patterns, extended guidance
- [`docs/retrospective.md`](docs/retrospective.md) — Retrospective guide and integration workflows
- [`docs/plan.md`](docs/plan.md) — Plan workflow reference
- [`docs/github.md`](docs/github.md) — GitHub operations reference
- [`docs/home-management.md`](docs/home-management.md) — Root and homes as separate repositories, the registry, identities, per-workspace resolution, health checks
- [`docs/examples.md`](docs/examples.md) — Complete interaction walkthroughs
- [`docs/todotxt.md`](docs/todotxt.md) — Todo.txt format specification
- [`docs/test-cases.md`](docs/test-cases.md) — Validation test cases
- [`docs/resources/prompt-learning-loop.md`](docs/resources/prompt-learning-loop.md) — Arize talk: feedback loops, rules-section optimization, eval co-evolution
- [`docs/resources/infinite-software-crisis.md`](docs/resources/infinite-software-crisis.md) — Netflix/Nations: simple vs easy, essential vs accidental complexity, three-phase approach
- [`docs/resources/progressive-disclosure.md`](docs/resources/progressive-disclosure.md) — Developers Digest: lazy-load context on demand, tools as files, memory as markdown; validates Daisy's existing lazy loading architecture
- [`docs/resources/agents-in-complex-codebases.md`](docs/resources/agents-in-complex-codebases.md) — HumanLayer/Horthy: context quality over quantity, RPI calibration, step granularity, intentional compaction on repeated corrections

## See Also

- **`prompts/daisy.md`** — User-focused workflows (loaded via per-home AGENTS.md)
