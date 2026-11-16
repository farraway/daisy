# 0083. The swarm's own repository: issues, briefs and notes in `<owner>/swarm-<name>`

Moved verbatim from `docs/swarm.md` section 14, item 83, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The swarm's own repository: issues, briefs and notes in
`<owner>/swarm-<name>` (Step 23 of the swarm build, 2026-11-09).** A
Gitea wiki is a flat directory with constrained page names, and a
wiki shared across swarms would put one swarm's notes in the next
one's prompts; a repository of the swarm's own, holding no product
code, holds the issues, the briefs under `briefs/` and the research
notes under `notes/` in one tree, versioned on the forge, one per
swarm. `forge.repo` names it, none of `repos` (validation already
allowed that, item 7 keeping one owner): no mirror, no worker, no
check and no image are its. Its briefs are the `resources` entry
`<clone>/briefs/`, its clone in the workspace the maintainer's
(Daisy reads the briefs there and writes nothing; a `resources`
entry inside a repository was already allowed, its `.git` excluded,
so validation needed no change for it). `notes: true` puts the notes
in its `notes/`: Daisy's host clone is the repository itself, at
`<state>/notes`, reached as `<owner>/<repo>.git` beside the first
declared repository's remote as the wiki is; each run's copy is that
directory alone, so the briefs and the rest never reach `/work/.notes`;
the push goes to the branch the clone checks out, the forge's
default, by Step 19's pick, conflict branch and secret check. Daisy
never writes the briefs: the copy is copied back over `notes/` only.
`notes` refuses a declared issues repository (Daisy writes no
declared repository's tree, section 2) and `wiki` beside it (the
swarm's own repository's wiki is no better than its tree); `wiki`
stays for a product issues repository. Where the step left a choice
the smaller was taken: `notes` a second top-level boolean beside
`wiki`, not a value of one field, so no existing file changes; the
swarm's own repository is told by `forge.repo` being none of `repos`,
with no field of its own, and the wizard asks about it as question
4's follow-up only for such a repository, numbering nothing anew;
question 32's default adds the clone's `briefs/` when the answer is
yes and the clone holds one; question 35 asks for the notes in place
of the wiki then; one mount, `/work/.notes`, one run copy,
`<run>/notes`, and one result line, `notes-base <sha>`, for both
forms (the wiki's `/work/.wiki` renamed: no run had used it, the
wiki never declared on a real swarm), while the host clones keep
their own directories, `<state>/wiki` and `<state>/notes`, so a
switch from one form to the other never reuses the wrong clone; a
`notes/` absent from the repository gives an empty copy, the first
note's push making it, and one that is a link gives an empty copy
too, never followed, and the run's notes are kept (`notes kept:
notes/ is not a directory`; `notes unchanged` for a run that wrote
none, item 88), never committed over the maintainer's
link. The state directory keys on `forge.repo`, as
built, so a move to the swarm's own repository starts the swarm
clean there, its issues, labels and milestones new.
