# 0119. Reconcile's loose ends

Moved verbatim from `docs/swarm.md` section 14, item 119, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Reconcile's loose ends (Step 19 of the plan that clears the
Deferred list, 2026-11-13).** **A bold `Status:`** (section 4): the
maintainer decided (2026-11-12) that the brief reader takes
`**Status:** <…>`, the form Daisy's own plan and project templates
write, rather than leave such a file silently a resource;
`brief.Field` reads either form of any header field, so a research
note's fields too. **A slice prefix with one repository** (7.5): the
architect may begin a slice with its subject (`1. Parser: …`); with
one repository declared there is nothing to choose, so a prefix
naming no repository is read as the slice's text; with more it
stays *bad slices*, a slice there having to name its repository. A
name alone cannot tell `Parser` from a repository removed from
`repos` since the plan (`1. lib: …`), whose slices must stay *bad
slices* (item 65), so a name the swarm holds a mirror of in
`<state>/git` (every repository declared since it began, and the
swarm repository) is never read as text. **The round's diff** (7.4): `$(git diff --name-only …)`
split a name with a space and, empty, diffed the whole range; the
names now go through `-z` and `xargs -0 -r`. **The round limit's
later notes** (7.6): the note said `a second time` at the third limit
too; it now says `again`, which counts no time, and the old words are
still read, since notes on the forge carry them. **A CRLF end** (7.6):
a worker's text ending `\r\n` kept a `\r` before the check block.
**An abandoned run's fetch** (7.6, 7.7): the maintainer decided
(2026-11-12) that a fetch failing with git's error is retried as a
worker run's is, the first failure the pass's error and the second in
a row keeping nothing, since a transient failure lost the commits for
good; a clone that reaches outside itself, or has no branch, gives up
at once. To tell no branch from git's (localized) message, the
collection reads the clone's `.git` for the ref, loose or packed,
before the fetch, as the check before it reads `.git`; a `.git` in
git's reftable format is left to the fetch. A fetch that succeeded
keeps its commits even when the trailer's log after it fails (the
log's error is the pass's), where before the commits were dropped.
**An unreadable copy of the notes** (7.6, The notes): a directory the
member closed failed the collection at every pass; it is now `notes
kept`, whether `notes/` is a directory or not. **A mount point** (7.1,
7.7): a product repository tracking `.swarm`, `.notes` or
`.resources` as a file left a run directory with no `started` line
and the pass's error at every pass; it is now the fault *clone
failed*, so two in a row pause the swarm with the reason. Tests:
`TestSwarmApplyRoundLimit` (the limit a third time, `again`),
`TestAtTheRoundLimit` (both words), `TestRoundDiffLeavesOutABaseMerge`
(a name with a space; no file changed), `TestSliceRepos`,
`TestSwarmApplyBadSlices` (one repository: a prefix naming no
repository; a repository removed since the plan),
`TestSwarmWorkerPostTakesOffACRLFEnd`,
`TestSwarmAbandonedWorkerFetchRetried`,
`TestSwarmAbandonedWorkerKeepsWhatItFetched`,
`TestSwarmAbandonedWorkerWithoutItsBranch`, `TestFetchRunFindsTheBranch`,
`TestSwarmApplyOwnNotesUnreadable`, `TestSwarmApplyMountPointRefused`
and `TestRead`.
