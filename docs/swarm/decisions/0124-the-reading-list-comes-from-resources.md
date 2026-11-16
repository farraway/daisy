# 0124. The reading list comes from `resources`

Moved verbatim from `docs/swarm.md` section 14, item 124, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The reading list comes from `resources` (Step 4 of the plan for
the inbox requests F27–F35, 2026-11-14; request F35).** The owner's
and the architect's defaults said "the resources under
/work/.resources", so a swarm whose members needed to know which
resource was which wrote its own list into its persona prompts (the
request's swarm: its briefs, a product's architecture wiki whose
ADRs are rulings, a pinned library, its own docs, and the practice
pages its reviewers review against), which went stale (a library's
version written by hand beside a pin that moves) and made the
prompts edited, keeping them out of every later default (item 123).
Now a `resources` entry, and a reference's path, is its path alone
or `{"path", "note"}`, the note one line (section 3), and every
run's `system.md` ends with `# Resources`: each mount in
`swarm.json`'s order, its `README.md` as its index page where the
commit the run mounts holds one as a regular file, and its note
(7.3). The defaults read "the resources listed below" and name no
path (section 4); each role's new hash is appended to
`defaultHistory` (item 117). **Every role gets the list**, since
every role has the mount (7.2): no `roles` field and no `review`
note decides who reads what. A per-role filter would have to decide
mounts too, or list less than is mounted; a note says in words who a
resource is for ("the practice pages the reviewers review against"),
which a reviewer reads as it reads its persona, and a swarm whose
reviewers should hold to them says so in their prompts. **The list
moves** from `prompt.md`'s `- Resources (read-only):` line, which
went (the paths would otherwise be written twice), to `system.md`,
beside the swarm's notes about a repository: a note is the
maintainer's word, and `system.md` holds Daisy's and the
maintainer's. **The index** is named, not included: a wiki's
`README.md` can be long, and it is in the mount. **The note is
kept** by `init`'s question 32 for each path its answer keeps, as
`check` and `deps` are. A swarm whose prompts carry a hand-written
list is edited and stays as it is until its maintainer moves the
list into notes and rebuilds the prompts. Tests:
`TestSwarmResourcesAtTheirCommits` (the architect's `# Resources`:
a note on a resource and on a reference's path, an index, a link
named `README.md` and one pushed after a reference's pin none),
`TestSwarmMembersReadPushedContent`, `TestRenderResources` (every
role, last, `None.`), `TestDefaultPromptsNameNoResourcePath`,
`TestFormatResourceNotes`, `TestParseNamesTheField`,
`TestValidateResourcesAndReferences` and
`TestWizardKeepsTheFilesResourceNotes`.
