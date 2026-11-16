# 0067. The maintainer's verbs

Moved verbatim from `docs/swarm.md` section 14, item 67, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The maintainer's verbs (Step 10 of the swarm build; the
maintainer, 2026-11-07).** The maintainer's writes, made by hand with
the maintainer's token during the first real run (triage, accept,
the merge with `do: merge` and the approved head, T13's relabel and
the last slice's close), are six verbs in the binary, `triage`,
`accept`, `approve`, `merge`, `unblock` and `close` (section 11),
with the `swarm` skill as the agent session's guide over them, not a
skill that calls the forge: the verbs are tested on the Fakes, and
the skill holds no token and no endpoint. **Two tokens, two code
paths.** The verbs read `DAISY_MAINTAINER_TOKEN` and use no other;
`init`, `apply` and `status` never read it. This amends section 8,
rule 5, whose "the maintainer's token stays off the swarm's host"
(item 47) the maintainer's own choice to keep the token in
`<ws>/.env.sh` reverses: the boundary is the code paths and the two
refusals, not the host. **The refusals.** The token equal to the
swarm's is refused before the forge is reached; a token whose
`Login()` is the swarm's (a second token of the swarm's account) is
refused before any write. **The swarm's login is `trusted`'s last**,
the login `init` appends after the maintainer's (section 3), which
also tells the verbs the swarm's entries apart (the plan, the
reviews, the reconciler's notes). The alternative, `Login()` under
the swarm's token, was rejected: it is a forge call with the swarm's
token on the maintainer's path, which the plan's "each reading only
`DAISY_MAINTAINER_TOKEN`" forbids; a `trusted` reordered by hand
makes the verbs refuse the maintainer or find no approved head,
which fails closed. **`Approve` is a twentieth method**, not
`Review` with an approving verdict: `Review` falls back to a
comment-type review on a `422`, which for the maintainer would leave
no approval and say nothing; `Approve` has no fallback, and pins
the review to the head read (`commit_id`). **`merge` is the merge
step's merge, the maintainer's way**: the same approved head and
slice count, an already merged pull request going straight to the
relabel or the close (so a verb interrupted after the merge, or a
merge made on the forge, completes), and a refused merge changing
nothing (no note, no `blocked`), since the maintainer is there to
read the refusal. **`unblock` returns to the stage the newest
reconciler note that blocked the issue names** (every note but the
round limit's first, which blocks nothing), posting the maintainer's
comment before the relabel, so the role the stage runs reads it;
without such a note (a block made by hand) it refuses, and the
maintainer relabels on the forge. The tests assert authorship on the
Fake's state: `forge.Fake.As` is the same forge under another
account, and `Events` its timeline of label changes, closes and
merges by account (section 10).
