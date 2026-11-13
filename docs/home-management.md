# Home Management - Architecture

## Root, Homes and the Registry

Daisy is two kinds of repository:

- **The root** (`$DAISY_ROOT`): the code — the Go binary's source
  (`cmd/`, `internal/`), `prompts/`, `skills/`, `templates/`, `docs/`. One git
  repository, published as it is, so it never holds personal data.
- **A home**: one context's data — `journal/`, `tasks/`, `projects/`,
  `plans/`, `feedback/` — plus the configuration that shapes the agent for
  that context: `include.txt`, `skills/`, an optional `prompts/` whose files
  shadow the root's by name, a `templates/` whose `journal-day.md` and
  `journal-week.md`, when present, `new-day` and `new-week` use in place of
  the root's (see [templates.md](templates.md)), and its identities (`gitconfig`, `identities/`). A home is its own git repository and lives
  wherever you put it; every Daisy mutation (`log`, `done`, `new-day`, plans)
  ends in one commit there, except `plan new`, `feedback`, `eval
  --record` and `rotate` run by hand, whose writes the next commit takes (`daisy commit <message>`
  makes one by hand).

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
| `daisy home add <name> <path>` | Register an existing home; the path is stored resolved and absolute. Refused unless it is an existing directory that is neither the root nor a directory containing it, holds `include.txt`, and is its own git repository's top, where it can be staged |
| `daisy home remove <name>` | Unregister; the files are left in place |
| `daisy home new <name> [path]` | Create a home from `templates/home/` and register it (below) |

`daisy home new` collects every answer before writing anything: the location
(prompted when not given, default `$DAISY_LOCAL/homes/<name>`), then the git
identity's name and email. It then copies the template, writes the identity
to `gitconfig`, runs `git init` with a first commit as that identity,
registers the name, and builds the home's `AGENTS.md`. Leaving name or email
blank is allowed (reported, `gitconfig` is left as the template) but then the
first commit uses the machine's global git identity, and with none the
commit fails. Any failure after the first write, up to the registration,
removes the copy, so nothing half-made is left behind or registered; a
failed `AGENTS.md` build after it is a warning (`daisy prompt build
<name>` makes it later). `daisy init <name>` then binds a
workspace to it.

Home and identity names are ASCII: letters, digits, `.`, `_`, `-`, starting
with a letter or digit.

## Identities

The README defines identity beside root, home and workspace (Goals) and
shows what `daisy init work` writes (Usage, Bind a workspace); this
section is the mechanism. An identity is the git author a workspace's own
repository commits as —
`user.name`, `user.email` and, optionally, the SSH key it pushes with. It
does not change who commits in the home's repository: Daisy's own commits
there (`daisy commit` among them) carry that repository's git config, which
`daisy home new` sets from the default identity; so a home's own repository
is never bound as a workspace (`init` refuses it). An identity is a file of
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

`daisy identity list` prints every identity on the machine and its home.
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
applied in `.daisy/gitconfig`, in the same format, and what the three keys
held just before it applied them — a value set by hand, say — in
`.daisy/gitconfig.before`; `daisy clean` and a later `daisy init` unset
against that record — what was applied, not what the home's file says now —
only where the value in git config is still the one Daisy set, and put back
what the key held before. So `init` then `clean` leaves the keys exactly as
they were. Two exceptions: a workspace bound before records existed has
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
- `gitconfig.before` — what the three keys held before `init` applied it
  (what `clean` puts back), present when any held a value
- `AGENTS.md` — the prompt built for that home
- symlinks into the home: `tasks/`, `today.md`, `journal.md`, `projects/`,
  `plans/`, `feedback.md`
- `project` — optional: the project this workspace is bound to

**Resolution order** (`env.Resolve` in the binary):

1. Walk up from `$PWD` to the nearest workspace: a directory with a `.daisy/`
   that is not the Daisy root itself. Read the name from `.daisy/home`, look
   it up in the registry, and validate the path (absolute, present, neither
   the root nor containing it). A `.daisy/` without a `home` file is an
   incomplete binding to repair with `daisy init <identity>` or `daisy
   clean`, never a level to walk past.
2. With no workspace above, fail: "Not in a Daisy workspace", naming
   `daisy init <identity>` as the fix. There is no default home.

Resolution is stateless, so different workspaces on one machine use
different homes, or the same home under different identities, concurrently.

Two sessions in one home share its repository, and Daisy takes no lock of
its own. Each command that changes the home writes its files whole (a
temporary file renamed over the old) and ends in one commit that stages
every change in the home (`gitx.Repo.Commit`), another session's
uncommitted write included. Git's index lock is what serialises the
commits: a command whose `git add` or `git commit` finds it held fails
with git's message and exits non-zero, its edit left in the home, and the
next commit from either session takes it. Two commands editing the same
file at the same moment are not merged: the later write wins.

## Switching Home or Identity

Re-run `daisy init <identity>` in the workspace. The new binding's `home`,
`identity`, six links and `AGENTS.md` are prepared in a staging directory
beside `.daisy/`; then the previous identity is unset from the repository's
git config (where it still matches the record), the new one is recorded into
the stage's `gitconfig` and applied; then the staged directory is swapped in
by rename. Everything else `init` writes is decided before its first write,
so a refused `init` (an unknown identity, an unreadable settings file, a
directory where `.daisy/` holds a file) writes nothing; a failure in the
final step rolls the git config back and removes the stage. Other workspaces are unaffected; no environment variable changes.

A bare `daisy init`, anywhere in the workspace, does the same with the
binding's own home (`.daisy/home`) and identity (`.daisy/identity`, else
the home's default): the identity is taken back and applied again, so its
record and what the keys held before stay as they were, and everything
else `init` writes is rebuilt from the root and the home as they are now.
It is how a workspace takes up an update of the root.

## Health Check

`daisy healthcheck` verifies, in order: `DAISY_ROOT` is set and valid (1);
the home resolves for this workspace (2); the root is a git repository and
the home is its own repository — never the root's or some larger
repository that merely encloses it (3); required directories (4),
installed-skill drift (4b, 4c), `today.md` format and journal rotation state
(5, 5b), each command's own `--healthcheck` hook, called in-process (6),
publication hygiene (6c, see `AGENTS.md`), whose identifying terms include
every registered home's name and the git author (`user.name`,
`user.email`) configured in each usable home's repository and in the
root's, and that AGENTS.md's Command Reference is the cli table's (6b, a
warning). A home inside the root repository is refused: every commit it
made would land in the published root's history. A passing run is cached in
`<home>/.healthcheck.ttl` for `DAISY_HEALTHCHECK_TTL` seconds (default 300);
`--force` re-checks now.

## Related Code

| Package | Purpose |
|--------|---------|
| `bin/daisy` (`cmd/daisy`, `internal/`) | The CLI (`~/bin/daisy`); `daisy help` lists every command |
| `internal/env` | The registry, workspace and home resolution, `.env.sh` reading, the homes a root commit leaves out |
| `internal/workspace`, `internal/identity` | `init`, `clean`, identities and their drift |
| `internal/gitx` | The one commit every mutation makes, and `daisy commit [--root]` |
| `internal/health` | `daisy healthcheck`: the checks above, the TTL stamp, the publication scan |
