# 0126. The pull requests on the issue

Moved verbatim from `docs/swarm.md` section 14, item 126, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The pull requests on the issue (Step 7 of the plan for the
inbox requests F27–F35, 2026-11-14; the maintainer's request of
2026-11-12).** Gitea has no panel of an issue's pull requests across
repositories: the `Refs` line puts a cross-reference among the
thread's events, and the maintainer scrolled for it; the gate and
blocked notes named the stage, not the pull request. Daisy keeps a
block in the issue body (section 5) and links the pull request in
the first line of every note about one (section 12). **The place**:
after the marker lines when the body's first line is the marker,
since an entry is the swarm's by its first line (section 6) and an
owner's proposal with nothing after it must stay the owner's word,
so the owner at `proposal-posted` has nothing to answer; at the top
of any other body, a maintainer's, where it reads first. The request offered two rules, the block placed
so that the readers skip it or the readers strip it; both are taken.
The readers strip it (`withoutPulls`: the thread's first entry and
every reader of the body), which is what each relies on: the
owner's `Brief:` and `Class:` lines open the text as before, and the
issue body a member is given leaves it out, its `# Earlier pull
requests` and `# Pull request` sections giving the same. The
placement keeps the raw body's first line the marker as well, so the
body is the swarm's by section 6's rule to a reader that does not
strip it (an earlier Daisy, a person reading the source). **The
block**, between `<!-- daisy-swarm kind=pulls -->` and `<!--
/daisy-swarm kind=pulls -->`: a block is an opening line and the
nearest closing line after it, so the rests of a damaged one never
take in the text between them; anything else of the body is never
rewritten, and a fresh block goes at the top when none is whole.
**The round** a pull request is in, one more than its rounds
(section 6, over all its reviews), as the request's `open, review
round 2` reads while the second is reviewed or fixed; `approved`
once both newest reviews approve its head. **The writes**: step 8
lists every open swarm issue, each read with the flag's lists and
again only when the block would change, right before the edit, so a
pass that changes nothing edits nothing and one edits an issue at
most once; the merge step lists the issue its T15 closed, which no
later pass reads, and `swarm merge` lists the issue after its merge,
under the maintainer's token, for the same reason; nothing migrates,
each open issue gaining the block at the first pass that lists it.
The cost is a `Pulls(n)` per open issue and two reads per open pull
request each pass, as step 5 makes them. **The trigger rule** reads
the body at its `created_at`, which an edit does not move, and with
its marker first, so no edit wakes a role (section 6); the spec
counts no forge writes, so the edits are counted nowhere. A failure
is a warning: the block is a view. **The links** are full Markdown
links to `Pull.URL` (`html_url`), the pull requests living in other
repositories than the issue, the text `<owner>/<name>#<m>`, which
reads the same in a terminal; the readers of a note's text (a
direction's beginning, a round limit's pull request) take the form
before this step too, so an issue sent back or at the limit before
the upgrade is read as before. Tests: `TestSwarmPullsBlock` (two
slices, one merged and one open, the block and the owner's text
beneath it, edits once per change, the owner's `Brief:` and `Class:`
read, the merge gate's first line linking the pull request, both
merged in the pass that closes the issue, and #2's owner idle),
`TestWithPulls`, `TestPullsBlockKeepsTheReaders`, `TestPullState`,
`TestGiteaEditIssue`, `TestGiteaPulls` (`html_url`) and
`TestFakeEditIssueAndPullURL`.
