# 0097. The swarm repository is the swarm's root

Moved verbatim from `docs/swarm.md` section 14, item 97, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The swarm repository is the swarm's root (revision 6, Step 1 of
its plan, 2026-11-10; the maintainer's D1).** The workspace held
the configuration, the briefs' clone and the secrets by accident: a
swarm needs one place its definition lives and is versioned, and the
swarm's own repository (item 83) is that place. `daisy swarm` finds
the swarm as the nearest folder upward holding `swarm.json` and
`.git`; the plan said the nearest `swarm.json`, and the `.git` keeps
the old layout's configuration folder (no repository's top) from being
taken for a root while both layouts run. `forge.repo` is that
repository, validated by the clone's `origin` (the checks
`repos[].path`'s `origin` had), so one swarm has one repository and
no field names it twice; `wiki`, which keeps a declared issues
repository's notes, is then left to the old layout, the notes going
to `notes/`. No Daisy workspace, home or identity is read. The old
layout runs until Step 7, which the maintainer confirms.
