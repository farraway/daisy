# 0107. Members read pushed content, as built

Moved verbatim from `docs/swarm.md` section 14, item 107, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Members read pushed content, as built (Step 4 of revision 6's
plan, 2026-11-10).** In a swarm repository's clone, `apply`'s pass
fetches the swarm repository's mirror, `<state>/git/<repo>`, first,
reached at its clone's `origin` with the clone's `core.sshCommand`
(recorded in the mirror's config as a declared repository's is), its
`HEAD` pointed at the default branch `ls-remote --symref` reads; then
each declared repository's; then each reference's pin when its
mirror lacks it. The pass's commits are then read from the mirrors
(the swarm repository's `HEAD`, each declared repository's `<base>`,
each pin), once, before the first launch or before the owner's batch
trigger first reads the briefs (7.1, step 1), so a pass that tests
the trigger fetches though it launches nothing, and the trigger and
the batch prompt read the same pushed briefs. A pin the fetch does
not bring is recorded for the pass and is each launch's fault,
`fault: swarm.json: references[<i>]: pin <pin> not found at <url>:
<git's error>`, not the pass's error; a reference is fetched with
`--no-tags`, so a commit pin brings no tag with it. Each launch
lists each path at its commit (`ls-tree -r -t`): nothing there is
`does not exist at <short sha>` (a reference's `at <pin>`; a
repository with no commit fetched at all, an empty swarm repository
say, `does not exist: <repository> has no commit fetched`), and a
path element `.env.sh` or `.daisy` anywhere in it `contains <name>`;
then `git archive --format=tar <commit> -- <path>` is extracted in
Go under `<run>/resources/<repository>/` (directories, regular files
with their executable bit, links as links, nothing outside it; a
path two entries both name written once), and `resource <repository>
<sha>` appended after the start lines (and `notes-base`), one per
repository, in `resources`' then `references`' order. One read-only
mount, `<run>/resources` at `/work/.resources`, its mount point made
in `<run>/work`, given to a CLI that needs leave for it as one
directory; none when nothing is checked out. `<run>/resources` is
removed when the run is collected or abandoned. The prompt's
`Resources (read-only)` line names `/work/.resources/<repository>/<path>`
per resource and per reference path, and `# Briefs` mounts each brief
at `/work/.resources/<repository>/<its path>`. The briefs are read
in the mirrors (`ls-tree`, `cat-file blob`): every regular `.md`
blob under a `resources` entry, never a link, never from a
reference; a repository with no commit fetched yet holds none, so
`status` before the first pass shows none, and `status`, which
fetches nothing, reads them as the last pass left the mirrors
(`ReadOnly` lets `ls-tree` through, and `hash-object` without `-w`
for Step 5's Drift). A `Brief:` or `Met:` line whose path begins
`<repo>/`, the swarm repository's name, names the brief at the rest
(`BriefName`), in the milestone step, the batch's closed list, the
triage note and `status`'s met mark; the old layout reads every path
as it is. Validation in a clone checks the names alone (section 3's
rows: a path neither the clone nor the pushed head holds is valid
until a launch reads it); a `<name>:` prefix is read only when the
text before the first `:` is a repository name, so a
swarm-repository path holding a `:` after a `/` stays one.
`references` in the old layout is `references: unknown field`, as
the clone's other fields are there. The wizard's question 32 in a
clone reads `Resources (comma-separated: paths in the swarm
repository, or <name>:<path> in a declared one; - for none)`, its
default, until Step 6 seeds them, those of `briefs/` and `docs/`
the clone holds as directories, since a resource the pushed head
lacks refuses every launch. `init` in a clone fetches the swarm
repository's mirror before the declared ones'. The old layout keeps
its live bind mounts, one per resource, and its workspace paths.
The test fixture of a clone is cloned from an upstream of the swarm
repository, and what a test writes in the clone for the members is
committed and pushed (`publish`); `runner.Fake` gains a `Resources`
hook, the directory mounted at `/work/.resources`.
