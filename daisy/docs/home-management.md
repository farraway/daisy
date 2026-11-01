# Home Management - Architecture

## Root, Homes and the Registry

Daisy is two kinds of repository:

- **The root** (`$DAISY_ROOT`): the code — `daisy.sh`, `daisy/scripts/`,
  `prompts/`, `skills/`, `daisy/templates/`, `daisy/docs/`. One git
  repository, published as it is, so it never holds personal data.
- **A home**: one context's data — `journal/`, `tasks/`, `projects/`,
  `plans/`, `feedback/` — plus the configuration that shapes the agent for
  that context: `include.txt`, `skills/`, an optional `prompts/` whose files
  shadow the root's by name, and its identities (`gitconfig`, `identities/`). A home is its own git repository and lives
  wherever you put it; every Daisy mutation (`log`, `done`, `new-day`, plans)
  ends in one commit there (`daisy commit --home`).

The root knows homes only by name, through the **registry**:
`$DAISY_CONFIG/homes` (default `~/.config/daisy/homes`), one
`name=/absolute/path` per line; `#` comments and blank lines are skipped.
Nothing derives a home's path from `$DAISY_ROOT`.

Two per-machine directories, both overridable by environment variable:

| Variable | Default | Holds |
|---|---|---|
| `DAISY_CONFIG` | `~/.config/daisy` | Configuration: the `homes` registry |
| `DAISY_LOCAL` | `~/.local/daisy` | Data: `homes/<name>/` is the location `daisy home new` offers by default |

## The Registry: `daisy home`

| Command | Effect |
|---|---|
| `daisy home list` | Each registered name and its path; `(missing)` when the directory is gone |
| `daisy home add <name> <path>` | Register an existing home; the path is stored resolved and absolute. Refused unless it is an existing directory that is neither the root nor a directory containing it, holds `include.txt`, and is in its own git repository (or the root's) where it can be staged |
| `daisy home remove <name>` | Unregister; the files are left in place |
| `daisy home new <name> [path]` | Create a home from `daisy/templates/home/` and register it (below) |

`daisy home new` collects every answer before writing anything: the location
(prompted when not given, default `$DAISY_LOCAL/homes/<name>`), then the git
identity's name and email. It then copies the template, writes the identity
to `gitconfig`, runs `git init` with a first commit as that identity,
registers the name, and builds the home's `AGENTS.md`. Leaving name or email
blank is allowed (reported, `gitconfig` is left as the template) but then the
first commit uses the machine's global git identity, and with none the
commit fails. Any failure after the first write removes the copy, so nothing
half-made is left behind or registered. `daisy init --new <name>` runs `home new` and then binds the
current workspace to it.

Home and identity names are ASCII: letters, digits, `.`, `_`, `-`, starting
with a letter or digit.

## Identities

An identity is the git author a workspace's own repository commits as —
`user.name`, `user.email` and, optionally, the SSH key it pushes with. It
does not change who commits in the home's repository: Daisy's own commits
there (`daisy commit --home`) carry that repository's git config, which
`daisy home new` sets from the default identity. An identity is a file of
`key=value` lines in a home:

```
name=Jane Doe
email=jane@example.net
sshkey=~/.ssh/id_ed25519_jane
```

`sshkey=` is optional and must be one plain path (no spaces or comment on
the line); git expands a leading `~` itself, so the same file serves every
machine.

- `<home>/gitconfig` is the home's **default identity**, named by the home's
  own name. The template ships every line commented out; `daisy home new`
  fills in `name=` and `email=` from its prompts.
- `<home>/identities/<id>` is a further identity of that home, named `<id>`.
  One person can work against the same home from several workspaces whose
  own repositories commit as different authors — a personal and a work
  persona, say — by choosing the identity when binding each workspace.

`daisy init <identity>` resolves the identity to a home:

1. A registered home of that name wins: its default identity (`gitconfig`),
   with no further lookup.
2. Otherwise the registry must be whole first: while any registered home
   cannot be read (missing or invalid path) the lookup is refused, since
   that home might define the name too.
3. Then the identity must be `identities/<identity>` in exactly one
   registered home. Two homes defining it is an error naming both; none is
   an error listing every identity on the machine.

On the workspace repository `init` applies `user.name` and `user.email`
and, when `sshkey=` is set, `core.sshCommand "ssh -i <key> -o
IdentitiesOnly=yes"`. A named identity must be complete (name and email);
the default identity may be absent or incomplete (the template ships it all
commented out), in which case the workspace binds with no git identity
configured and `init` says so. `init` records the triple it
applied in `.daisy/gitconfig`, in the same format; `daisy clean` and a later
`daisy init` unset against that record — what was applied, not what the
home's file says now — and only where the value in git config is still the
one Daisy set. Two exceptions: a workspace bound before records existed has
none, so its bound identity file as it is now stands in (`init` says so when
it does this; `clean` does not); and a `core.sshCommand` in Daisy's exact
form (`ssh -i <key> -o IdentitiesOnly=yes`), which only an init writes, is
cleared whether or not a record names it. `daisy status` and `daisy files` print the identity in their
header line.

## Per-Workspace Home Resolution

Each workspace has a `.daisy/` directory holding:

- `home` — the home's name (looked up in the registry)
- `identity` — the identity's name, present only when it is not the home's
  default
- `gitconfig` — the identity `init` applied (the record `clean` unsets against)
- `AGENTS.md` — the prompt built for that home
- symlinks into the home: `tasks/`, `today.md`, `journal.md`, `projects/`,
  `plans/`, `feedback.md`
- `project` — optional: the project this workspace is bound to

**Resolution order** (`resolve_home` in `daisy/scripts/common.sh`, used by
every script):

1. Walk up from `$PWD` to the nearest workspace: a directory with a `.daisy/`
   that is not the Daisy root itself. Read the name from `.daisy/home`, look
   it up in the registry, and validate the path (absolute, present, neither
   the root nor containing it). A `.daisy/` without a `home` file is an
   incomplete binding to repair with `daisy init <identity>` or `daisy
   clean`, never a level to walk past.
2. With no workspace above, fall back to `$DAISY_DEFAULT_HOME` (set by
   `daisy install`) with a warning on stderr.
3. Otherwise fail: "Cannot resolve home", naming `daisy init <identity>` as
   the fix.

Resolution is stateless, so different workspaces on one machine use
different homes, or the same home under different identities, concurrently.

## Switching Home or Identity

Re-run `daisy init <identity>` in the workspace. The new binding's `home`,
`identity`, six links and `AGENTS.md` are prepared in a staging directory
beside `.daisy/`; then the previous identity is unset from the repository's
git config (where it still matches the record), the new one is recorded into
the stage's `gitconfig` and applied; then the staged directory is swapped in
by rename. A failure before the swap rolls the git config back and removes
the stage. Other workspaces are unaffected; no environment variable changes.

## Health Check

`daisy healthcheck` verifies, in order: `DAISY_ROOT` is set and valid (1);
the home resolves for this workspace (2); the root is a git repository and
the home is in its own repository or the root's — never in some larger
repository that merely encloses it (3); no registered home still lies inside
the root repository without being its own repository (3b, a warning); required directories (4),
installed-skill drift (4b, 4c), `today.md` format and journal rotation state
(5, 5b), each script's own `--healthcheck` (6), and publication hygiene (6c,
see `AGENTS.md`), whose identifying terms include every registered home's name
and the git identity of each home's repository. A passing run is cached in
`<home>/.healthcheck.ttl` for `DAISY_HEALTHCHECK_TTL` seconds (default 300);
`--force` re-checks now.

## Related Scripts

| Script | Purpose |
|--------|---------|
| `daisy.sh` (repo root) | CLI entry point (`~/bin/daisy`) |
| `home.sh` | The registry: `daisy home list\|add\|remove\|new` |
| `daisy-init.sh` | Bind a workspace to an identity and its home; switch by re-running |
| `common.sh` | Registry, identity and resolution functions shared by every script |
| `commit.sh` | `--home` commits in the home's repository, `--not-home` in the root's |
| `healthcheck.sh` | System validation, including the checks above |
