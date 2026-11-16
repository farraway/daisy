# 0068. Review until a pass finds nothing real

Moved verbatim from `docs/swarm.md` section 14, item 68, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Review until a pass finds nothing real (Step 11 of the swarm
build).** The code changed where a review or an attack showed it
against this document, and the document where it was wrong:
**`forge.token_env` may not name `DAISY_MAINTAINER_TOKEN`** (section
3), or `apply` would act with the maintainer's token; **`apply` takes
its lock before it reads the forge or Docker** (section 11, Start), so
the Lock row's "nothing read from the forge" holds; **a worker run's
secret check comes before its protected-path check** (7.6 step 3),
whose note names paths, and **`--no-renames`** names a protected file
renamed away; **a push the forge does not answer is a git error,
retried**, and only a ref git's porcelain marks `!` is *push rejected*
(7.6 step 5); **the pass acts on an issue as `Issue(n)` reads it**,
not as the label list read before it did, **counts every container it
starts during the pass**, a check started in step (3) included, for
rules 2 and 4, and **the merge step merges only an open issue whose
one `swarm:*` label is `approved`** (section 11, A pass; section 12);
**a pull request's body is an entry by the login that opened it**
(`Pull.Author`, section 6 and 10), so a marked body opened by another
account is not the swarm's; **a proposal's title is 1 to 200
characters**, not bytes (7.5); **a slot's secrets are its strings of
32 bytes or more**, as built (7.6); **a brief's header ends at its
first `## ` heading outside a fence** (section 4).
