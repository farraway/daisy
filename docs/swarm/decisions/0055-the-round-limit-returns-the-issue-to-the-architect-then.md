# 0055. The round limit returns the issue to the architect, then blocks it

Moved verbatim from `docs/swarm.md` section 14, item 55, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The round limit returns the issue to the architect, then blocks
it** (revision 5; the maintainer's plan-workflow rule of 2026-11-05,
one cap and a hand-back rather than a stop, carried to the swarm;
confirmed as built, with the plan gate off too, by the maintainer
on 2026-11-12). `max_rounds` defaults
to 5, range 1 to 10. The pair completing with a request for changes
at the limit moves `in-review` to the architect's stage with a
reconciler note (T11): `planning` with G2 on, so the maintainer
accepts the revision (T7), and `ready` with G2 off, so the architect's
revision is accepted as it is written (T6) and the swarm keeps the
decision the gate gives it; either way, while the pull request is open, the architect's prompt holds
the earlier pull requests, the current pull request's thread and the
check's result, and its Task text names the reviews as the input and
keeps slice `<s>`'s number, repository and branch, so that the worker
continues on them (section 5; 7.1 step 2 takes a continued pull
request's repository from the pull request, a revised plan naming
another being *bad slices*); rounds are counted from the accepted
plan, so the revised plan has `max_rounds` of its own; the second
time the same pull request reaches the limit, told by a round-limit
note for the limit on it from an earlier run (a retried collection
finding its own note is still the first time), the issue is `blocked`
(G4). No new state and no new forge call: the count and the
first-or-second are read from the threads. Step 1 built the default
as 3, and Step 4 of the swarm build changed it to 5 with the wizard's
test. Alternative: `blocked` at the first limit (revisions 1–4),
under which every hard slice was the maintainer's to resume.
