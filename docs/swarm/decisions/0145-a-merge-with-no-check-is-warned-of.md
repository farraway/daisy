# 0145. A merge with no check is warned of

From Step 35 of the plan remediating the 2026-11-15 design review (talk A6, 2026-11-16; wording approved by the maintainer 2026-11-16).

**With the merge gate off, a repository with no check is warned of and
listed as waiting, never refused (Step 35 of the plan remediating the
2026-11-15 design review, 2026-11-16; talk A6).** With `gates.merge`
false the merge step merges each approved pull request itself (section
12), and a repository whose `check` is `[]` has no check of Daisy's run
on the head first: two model reviews, and whatever checks the forge
reports, decide the merge. The maintainer should see that, not be
stopped by it: a new stop is the maintainer's to make, and none was
asked for. **`apply`** prints `⚠ swarm.json: repos[<i>] (<name>) has no
check and gates.merge is false: no check of Daisy's runs on its pull
requests before the merge step merges them`, one line per such
repository (`declare`'s `Config.Unchecked` and `UncheckedWarning`), at
the process's first pass and again only when the set of lines changes,
as Drift's lines are, so the loop the unit runs does not repeat it every
interval and an edit of `swarm.json` while it runs is warned of at the
next pass. **`status`** lists each under Waiting for you as `no check
<name>` (`waiting.no_check` under `--json`), after `notes`, and
`connect` shows that line and key, read from the same report. It is **a
waiting item, not a note** beside the line: Waiting for you already
holds an item no verb answers (`notes`, a branch merged by hand), and
every item there is told by notify by the function that names it, so
this one is too (`<owner>/<repo> waits for you: no check <name>`, known
by the repository's name), once, and again only after it left and came
back. A note shown beside the line but told by no one would have been
the first exception to that rule. The cost: a swarm the maintainer keeps
that way on purpose never reads `Waiting for you: nothing`; the item
leaves when the repository declares a `check` or the gate is turned on.
Alternatives: a refusal by validation (a stop the maintainer did not ask
for); a warning at `apply`'s start only (an edit while the loop runs
would go unseen until a restart); a warning at every pass (the unit's
log repeats it every interval); a `StatusWait` entry (an item names an
issue, which a repository is not); a note beside the line that notify
does not tell. Test: `TestSwarmMergeWithoutACheck`.
