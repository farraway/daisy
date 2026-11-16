# 0114. A path gone from a member's `.git` mid-check is skipped, and a member's clone has no automatic maintenance

Moved verbatim from `docs/swarm.md` section 14, item 114, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**A path gone from a member's `.git` mid-check is skipped, and a
member's clone has no automatic maintenance (Step 3 of the plan that
clears the Deferred list, 2026-11-11).** The full suite failed now
and then with *the clone reaches outside itself: a file in .git
cannot be read*. A current git's `git commit` ends by starting `git
maintenance run --auto --quiet --detach` (`maintenance.autoDetach`
defaults to true), which holds `.git/objects/maintenance.lock` past
the commit's exit and then removes it; the check (7.6 step 1)
listed the lock, found it gone at its `lstat`, and read the vanished
path as one that cannot be read. A path gone (`ENOENT`) between its directory's listing and the
check's look at it, file or directory, is now skipped: it is no way
out of the clone, since what host git reads is what is there when it
fetches, which the check cannot pin for a clone a process still
writes to in any case. A path that is there and cannot be read
still refuses, as does a link anywhere after a skipped path, and
`.git` itself gone is the failure. The clone is made with
`maintenance.auto` `false` (7.1 step 3), so no such writer starts
from a member's commit at all; a member may set it back, which the
skip covers. Tests: `TestFetchRunSkipsAVanishedPath` (a file or a
directory removed between the check's calls, through the FS it
reads, is skipped and the branch fetched; one denied refuses; a link
after a skipped file refuses), `TestCloneAndTheCommitMsgHook`
(`maintenance.auto` is `false`).
