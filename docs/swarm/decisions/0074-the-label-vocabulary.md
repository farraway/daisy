# 0074. The label vocabulary

Moved verbatim from `docs/swarm.md` section 14, item 74, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The label vocabulary (Step 15 of the swarm build, 2026-11-07).**
The maintainer's request: the forge should read in stage groups,
each stage numbered by how far it has got. **Nine stages**
(`swarm:proposal-posted`, `swarm:proposal-accepted`,
`swarm:plan-0-posted`, `swarm:plan-1-accepted`,
`swarm:plan-2-in-progress`, `swarm:review-0-ready`,
`swarm:review-1-in-progress`, `swarm:review-2-approved`,
`swarm:review-3-changes-requested`), **four flags** (`swarm:gated`,
`swarm:blocked`, `swarm:paused`, `swarm:blocking`) and the **two
terminals**: fifteen labels, where the plan's text said sixteen,
counting `swarm:blocked` both among the eight old stages and among
the flags; the vocabulary it names is fifteen. **The rename is made
in place** by `EnsureLabels` (Gitea's `PATCH /orgs/{org}/labels/{id}`
with the new name), so every issue, open or closed, keeps its label,
its id and its history and reads the new name; alternatives: new
labels and a relabel of every issue (the history split across two
labels, one write per issue), or the old names kept (two
vocabularies). A rename is an owner's right on Gitea as a create is
(item 50): the swarm's token is refused, and the error names every
rename and create left for an owner, who renames in place, never
deleting. An old name beside its new one is left as it is (a rename
would collide). A run recorded under an old name is read under the
new one (`forge.Current`), so a run the old binary launched and the
new one collects still moves its issue. **The in-progress stages are
written once the container has started**, so a refused or failed
launch never writes one (it moves only the pair's second reviewer's
issue, back home to review-0-ready), and the collection moves it on from
whichever of the two stages it carries, so a refused write costs
nothing; a run that does not move it on takes it back home (T22):
a worker's to the stage it was launched on, which `<run>/result`
already records, and a reviewer's to review-0-ready, as the plan's
text says, the pair's second included (launched on
review-1-in-progress, so its label alone would leave it there): the
role rule launches it again from review-0-ready, the first's review
standing on the head.
**`swarm:blocked` is a flag** that keeps the stage, so the note need
not name where the issue was and `unblock` no longer reads it (the
note says `(at swarm:<stage>)`); the round limit's second time keeps
`review-1-in-progress`, the stage as it was, where no role runs, so
`unblock` still refuses there and names the relabels to make first
(alternative: `review-3-changes-requested`, which would make
`unblock`'s default one more fix round, a change to Step 10's
refusal the request did not ask for). A verb's relabel leaves
`swarm:blocked` and `swarm:paused` as they are, only `unblock` and
`resume <n>` clearing them; a close clears every flag. The merge
step skips a blocked issue as it skips a paused one, or it would try
a refused merge again, and post its note, at every pass.
**`swarm:paused`** is the maintainer's hold on one issue, beside the
whole swarm's `<state>/paused`, which the bare verbs keep.
**`swarm block <n> --by <m>`** holds a job by the maintainer's
comment, `Blocked by #<m>`, while it is the newest maintainer
comment on #n and #m is open, `Issue(m)` read only for such a
comment; `swarm:blocking` on #m says so on the forge and no rule
reads it. #m may be any issue of `forge.repo`, the swarm's or not.
A later maintainer comment lifts the hold as #m's close does, and
the comment is then the newest counted entry, which the role of an
issue-thread stage answers. The cli takes a value flag only before
the positionals, so `block` reads `--by <m>` after `<n>`, as its
usage writes it, as well as before. `status` lists the holds on a
`Held:` line, and a blocked issue under `blocked` in "Waiting for
you" at whatever stage. An open issue blocked before the change
carries `swarm:blocked` alone, a flag and no stage, and is no swarm
issue until the maintainer adds the stage its note names. **An empty
`co_author` is no trailer** (section 3): the maintainer's choice to
leave the swarm's commits without a co-author line, as the
maintainer's own commits carry none. `checkCoAuthor` accepts `""`,
the clone gets no `commit-msg` hook (a hook that adds nothing would
be dead weight in every clone), and the trailer scan reports nothing
(it would call every commit unstamped). The wizard's empty answer
already means the default, so none is `-`, as the list questions
take it, and a file's empty value is offered as `-` while the role's
provider and model are the file's.
