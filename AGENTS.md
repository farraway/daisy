# Daisy - System Architecture & Internal Specifications

This AGENTS.md is for **Mode 4: meta work** — modifying the Daisy system itself (scripts, prompts, templates). It is auto-applied by Cursor and Claude Code when editing files in this directory.

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

Every file git would keep — tracked, or untracked but not ignored — outside
top-level `home/` is published verbatim to public mirrors (see the dz-meta
workspace's `migrate.sh` — an rsync with no sanitization pass). Treat every
system file — scripts, prompts, docs, templates, skills, tests, dotfiles —
as already public.

**`.gitignore` is the publication boundary.** It is the single declaration
of what does not ship, honoured by both the publisher (`migrate.sh` filters
its rsync through it) and the checker (`daisy healthcheck`). That is what
keeps per-machine workspace state (`.daisy/`, `.claude/settings.local.json`,
`.cursor/rules/daisy*.mdc`) and secrets (`.env.sh`) out of the mirror —
files that sit inside `$DAISY_ROOT` and would otherwise publish. Two
consequences:

- Keep ignore rules in `.gitignore`. A global `core.excludesFile` or a
  `.git/info/exclude` entry would hide a file from the scan that `migrate.sh`
  still publishes — the one divergence that turns the check into a false
  negative.
- Adding a per-machine artifact to `$DAISY_ROOT` means adding it to
  `.gitignore` in the same change. Until then it is public.

**Never write into system files:** real home names, the OS username, real
people's names or emails, employers, or absolute paths under `$HOME`.

**Use the standard substitutes:**
- `$DAISY_ROOT`-relative paths, never `/home/{user}/...`
- Placeholder home names: `work`, `personal`, `example` in docs; `testhome`
  in test fixtures
- `Jane Doe <jane@example.net>` for identities in examples
- `~alias` notation for people

Real names belong only in `home/{name}/`, which the sync excludes wholesale.

`daisy healthcheck` enforces this with a scan of the publishable surface —
`git ls-files --cached --others --exclude-standard`, minus top-level `home/`
— for identifying terms derived from the machine at runtime (home directory
names, `$USER`, the repo's git identity, literal `$HOME` paths), never from
a hardcoded list, which would itself leak. The scan is a net, not the
standard: it cannot catch personal *content* that contains no known term,
and it says nothing about a file it considers unpublishable. Write system
files as if they were already public.

---

## Script Reference

| Script | Purpose |
|--------|---------|
| `daisy.sh` (repo root) | CLI entry point — dispatches subcommands |
| `daisy-init.sh` | Initialize Daisy in a workspace with a specific home |
| `new-day.sh` | Archive yesterday, generate new today.md, rotate journal.md |
| `new-week.sh` | Archive completed tasks to done.txt + new day, rotate journal.md |
| `rotate.sh` | Archive journal.md day-blocks into rolling/closed window files |
| `done.sh` | Mark task complete in todo.txt and today.md |
| `log.sh` | Add timestamped log entry to today.md |
| `feedback.sh` | Record a prompt failure with workflow tag and description |
| `optimize.sh` | LLM-driven prompt optimization loop (train/test split, diff, approval, archives used entries) |
| `eval.sh` | List, display, or record eval case results for the optimize loop |
| `create-home.sh` | Create new home from template, optionally activate |
| `build-prompt.sh` | Generate home/{home}/AGENTS.md from include.txt |
| `common.sh` | Shared functions (resolve_home, require_env) |
| `commit.sh` | Git commit helper — stages and commits changes under home/ |
| `check-secrets.sh` | Report which API tokens/secrets are configured in .env.sh |
| `healthcheck.sh` | System validation |
| `files.sh` | Report resolved real paths for every per-home file/directory |
| `list.sh` | List active prompts (with trigger summary) and installed skills (with description) |
| `projects.sh` | List active or archived projects with resolved paths |
| `tasks.sh` | Search todo.txt/done.txt (read-only); `--all\|--done\|--todo [pattern]` |
| `plan-new.sh` | Create a new Daisy plan file, symlink as PLAN.md (or, with `--spec`, an unregistered draft in the working directory) |
| `plan-pickup.sh` | Promote a local spec draft (from `plan-new.sh --spec`) into a tracked plan |
| `plan-archive.sh` | Archive the active Daisy plan, remove the PLAN.md symlink |
| `test.sh` | Run the hermetic `daisy/tests/` suite (`daisy/tests/run.sh`) |

**Documentation policy:** every script in `daisy/scripts/` must have a row in this table, and — if it implements the `--healthcheck` contract — a corresponding entry in `healthcheck.sh`'s `HEALTHCHECK_SCRIPTS` array. `healthcheck.sh` includes an automated drift check for the table half of this; there is no automated check for the array half beyond that same script list (see `daisy/scripts/healthcheck.sh`).

Scripts do NOT yet implement: priority floor rules, task preservation from yesterday, quiet day consolidation, advanced sync validation, JIRA sync, or log audit during retrospective. These are handled by the agent following prompt instructions.

After modifying scripts or workflow logic, validate against [`daisy/docs/test-cases.md`](daisy/docs/test-cases.md).

---

## Development Heuristics

**Prototype manually before automating.** When adding a new workflow or modifying a complex script, do the operation by hand first — no scripts, no automation. The manual run reveals hidden constraints and invariants that are invisible in the abstract. Only after understanding what a correct manual execution looks like should you codify it into a script or prompt rule. This is especially true for multi-file operations (new-day, new-week) and sync workflows.

---

## AGENTS.md Build System

`build-prompt.sh` generates `home/{home}/AGENTS.md` from `home/{home}/include.txt`. Workspaces access it via `.daisy/AGENTS.md` symlink.

Usage: `daisy build [home-name]`

`include.txt` is prompts-only. Skills (`skills/`) have no manifest — every
skill in the merged root+home set always installs at `daisy init`; see
[`daisy/docs/prompts-vs-skills.md`](daisy/docs/prompts-vs-skills.md) for why
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
2. Add `~{name}` (lazy) or `{name}` (full) to `home/{home}/include.txt`
3. Rebuild: `daisy build {home-name}`

The build script extracts everything between `## Trigger` and the next `#` or `##` heading for lazy stubs. If no `## Trigger` is found, falls back to full inclusion with a warning.

### Creating a New Skill

Before creating either a prompt or a skill, apply the coupling criterion in
[`daisy/docs/prompts-vs-skills.md`](daisy/docs/prompts-vs-skills.md): does
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
   `~/.claude/skills/` via `daisy install`; home skills
   (`home/{home}/skills/`) install to the workspace's `.claude/skills/` via
   `daisy init` — separate targets, no install-time merge. `daisy list`
   consolidates both for display, with workspace/home entries overriding
   root/user entries of the same name.
3. Re-run `daisy install` (root skill) or `daisy init` (home skill) in any
   workspace that should pick it up — skills are copied, not symlinked, so
   this is the only way to refresh one

### Adopting a Skill from Reference

Reference material staged at `daisy/docs/resources/mattpo-skills/{name}/`
(or any future reference source) is not a skill until adopted:

1. Copy the whole directory — `cp -r daisy/docs/resources/mattpo-skills/{name}
   skills/{name}` — this carries the source's `NOTICE.md` along
   automatically.
2. Copy `LICENSE` alongside: `cp daisy/docs/resources/mattpo-skills/LICENSE
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

- [`daisy/docs/prompts-vs-skills.md`](daisy/docs/prompts-vs-skills.md) — The hat/skill distinction, the ownership-of-evolution criterion, worked examples
- [`daisy/docs/task-format.md`](daisy/docs/task-format.md) — Task format regex, conversion rules, file organization
- [`daisy/docs/task-sync.md`](daisy/docs/task-sync.md) — Bidirectional sync rules, validation algorithm
- [`daisy/docs/templates.md`](daisy/docs/templates.md) — Template placeholders and formatting rules
- [`daisy/docs/workflows.md`](daisy/docs/workflows.md) — All workflow algorithms (new-day, new-week, done, cancel, add, priority, sync, etc.)
- [`daisy/docs/logging.md`](daisy/docs/logging.md) — Logging design rationale, triggers, abridged archival
- [`daisy/docs/projects.md`](daisy/docs/projects.md) — Project management architecture and algorithms
- [`daisy/docs/praxis.md`](daisy/docs/praxis.md) — Implementation quality reference: how we work, anti-patterns, extended guidance
- [`daisy/docs/retrospective.md`](daisy/docs/retrospective.md) — Retrospective guide and integration workflows
- [`daisy/docs/plan.md`](daisy/docs/plan.md) — Plan workflow reference
- [`daisy/docs/github.md`](daisy/docs/github.md) — GitHub operations reference
- [`daisy/docs/home-management.md`](daisy/docs/home-management.md) — Per-workspace home resolution, health checks
- [`daisy/docs/examples.md`](daisy/docs/examples.md) — Complete interaction walkthroughs
- [`daisy/docs/todotxt.md`](daisy/docs/todotxt.md) — Todo.txt format specification
- [`daisy/docs/test-cases.md`](daisy/docs/test-cases.md) — Validation test cases
- [`daisy/docs/resources/prompt-learning-loop.md`](daisy/docs/resources/prompt-learning-loop.md) — Arize talk: feedback loops, rules-section optimization, eval co-evolution
- [`daisy/docs/resources/infinite-software-crisis.md`](daisy/docs/resources/infinite-software-crisis.md) — Netflix/Nations: simple vs easy, essential vs accidental complexity, three-phase approach
- [`daisy/docs/resources/progressive-disclosure.md`](daisy/docs/resources/progressive-disclosure.md) — Developers Digest: lazy-load context on demand, tools as files, memory as markdown; validates Daisy's existing lazy loading architecture
- [`daisy/docs/resources/agents-in-complex-codebases.md`](daisy/docs/resources/agents-in-complex-codebases.md) — HumanLayer/Horthy: context quality over quantity, RPI calibration, step granularity, intentional compaction on repeated corrections

## See Also

- **`prompts/daisy.md`** — User-focused workflows (loaded via per-home AGENTS.md)
