# 0109. `init` creates and seeds the swarm repository

Moved verbatim from `docs/swarm.md` section 14, item 109, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`init` creates and seeds the swarm repository (Step 6 of
revision 6's plan, as built, 2026-11-10; the maintainer's D7).**
`init` run where no clone holding `swarm.json` is found, and no old
layout either, takes the working folder (`declare.Folder`,
`Place.Outside`), in a Daisy workspace or not: detection reads the
clones directly under it, never its own `origin`, and validation
leaves out the clone's `origin` until `init` has made the clone; the
old layout's `init` runs only where `<ws>/swarm/swarm.json` is, so a
workspace no longer gets a new swarm in the old layout. The forge
seam gains `Repository`, `CreateRepository` and `AddCollaborator`,
on Gitea `GET /repos/{o}/{r}`, `POST /orgs/{o}/repos` (`{"name",
"private": true}`) and `PUT /repos/{o}/{r}/collaborators/{login}`
(`{"permission": "write"}`), recorded from the Swagger; the GitHub
adapter maps none, as for the milestones. **Spec amended (sections 10
and 11):** the three calls take no repository: each acts on
`forge.repo`, as the adapter is built for it (section 11 had written
`Repository(<owner>/<repo>)`, `CreateRepository(<owner>, <repo>)` and
`AddCollaborator(<owner>/<repo>, <login>)`), and `CreateRepository`
answers the created repository's `ssh_url` and `default_branch`, so
no second read follows it. **Choices the spec left open:**
`<dir>/<repo>` is checked after the answers and before any forge
call: a clone of the swarm repository holding no `swarm.json` is the
clone (a maintainer's own clone of a repository made by hand), one
holding `swarm.json` refused (`init` from the folder answers from
detection, which would overwrite the file's values; it runs in the
clone), anything else refused as section 11 says; `init`'s two
messages for the maintainer's token are the maintainer's verbs',
reworded for `init` (`the maintainer's writes act with your own`, the
swarm's account named by `$<VAR>`'s login, as `trusted` is not yet
written); `<state>/env`'s values are single-quoted, so that no `$` or
backquote in a token is read as shell syntax; a clone of an empty
repository gets the default branch as its first branch before the
commit; files written but equal to the clone's `HEAD` commit nothing,
and a file of section 2's table the clone holds and its `HEAD` lacks is
committed with them, so a re-run after a failed commit of new files
completes it (a rewritten `swarm.json` whose commit failed stays
staged for the maintainer to commit);
a failed commit is `Error: swarm: commit in <swarm>: <git's error>`;
the labels line names the swarm's token (`✓ Token $<VAR>: labels
…`) in revision 6 and stays `✓ Labels: …` in the old layout and in
`apply`; the seeded READMEs' text is `declare.Seeds`. Question 32's
default is `briefs/` and `docs/` whether the clone holds them or not
(Step 4's interim of those the clone held is gone). The secret names
`<state>/env` assigns and `check-secrets` reports are one list,
`Config.SecretNames`. Tests: the clone fixture runs `init` in a
folder holding the products' clones, the forge holding no swarm
repository and its upstream empty, with both tokens in the
environment and the maintainer's git identity global; `init` creates,
clones, commits and pushes, and every revision 6 test builds on what
it leaves; the old layout's fixture runs its first `init` at its
place; the cli's old-layout tests seed `<ws>/swarm/swarm.json`, and a
cli test runs the new `init` through the gitea adapter over an
in-memory forge and real git. The instance smoke (a scratch
`swarm-<name>` created on the Gitea instance and deleted) needs the
maintainer's say-so on their token and is not run by the step.
