# 0112. The workspace layout retired

Moved verbatim from `docs/swarm.md` section 14, item 112, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The workspace layout retired (Step 7 of revision 6's plan, as
built, 2026-11-10; the maintainer's go-ahead of that day, every
live swarm moved).** The old layout's every branch is gone: the
lookup of `<ws>/swarm/swarm.json` (`declare.Find` reads the clone
alone, and takes no Daisy root; a workspace's `swarm/` is no
swarm, `init` taking the workspace as the folder it makes a clone
in, section 11, Finding the swarm), `repos[].path` and its
validation, workspace `resources` and their live bind mounts,
`<ws>/.env.sh` for the swarm (`check-secrets` keeps its rows for
the workspace's own variables), the old layout's `init` and wizard
(paths in the workspace, the issues repository and its "own"
follow-up), and the brief reader's match of the old workspace path
(`<repo>/<path>` names no brief, section 4). `wiki` went with it:
since revision 6 `forge.repo` is the swarm repository, never a
declared one, so validation refused `wiki` on every clone, and it
survived only for the old layout; the notes' clone is the swarm
repository's alone (`<state>/notes`, its `notes/`). A file holding
`path` or `wiki` is refused by the parse, `unknown field`, naming
it. `Place` keeps `Root` alone, the old configuration folder
(`Dir`, `Rel`, `Old`) having no use. The kept tests run once, on
the clone: the old fixture, `bothLayouts` and the tests of the old
layout alone (live resource links, the workspace's `.env.sh`, the
old wizard) are removed, and the research wiki's tests now drive
the notes in the swarm repository. With the issues in the swarm
repository and the pull requests in the declared ones, a pull
request is numbered in its own repository, so `forge.Fake` gains
two seeding helpers, `ClosePull` (a pull request closed unmerged,
which `CloseIssue` did when both shared the issues repository) and
`PullInIssues` (a pull request opened in the swarm repository, as a
maintainer may, which the verbs refuse by number); the adapter seam
is unchanged. Section 11, Moving a swarm into its repository, keeps
the old files' description and the steps, for a maintainer moving a
swarm with a Daisy older than this step.
