# 0079. The research wiki

Moved verbatim from `docs/swarm.md` section 14, item 79, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The research wiki (Step 19 of the swarm build, 2026-11-08).** On
the first two days the architects were 46% of the bill, and their
cost was research: four architects on one evening each read the same
upstream source again. A swarm declaring `wiki` keeps what its
members research as notes in the issues repository's wiki, each
pinned to the commit or tag it describes, so the next owner or
architect reads the note, judges it by its pin, and checks what
changed instead of researching again (7.3, 7.4). **The member never
holds a `.git` Daisy reads**: it is given a copy of the pages
(7.1), and host git commits in its own clone, `<state>/wiki`, after
copying the changed pages across without links, so a hook or a
config the member writes is never run on the host (section 8, the
rule 7.1's step 3 keeps for the clones). **The maintainer edits the
wiki at will**, so Daisy's push merges rather than overwrites: the
run's commit is made on the commit it was given, then picked onto
the wiki's head as fetched; a conflict goes to its own branch with a
warning. Where the step left a choice the smaller was taken: `wiki`
is a top-level boolean, absent read as `false` and written only when
`true`, so no existing file changes; the wizard asks it last,
question 35, rather than renumbering 33 and 34; the wiki's URL is
the first declared repository's remote with its last element
replaced, as Gitea and GitHub place a wiki, rather than a forge call;
a run adds and changes pages and deletes none; the conflict branch
is `swarm/notes-<n>-<role>-<k>`, not the plan's `swarm/notes-<n>-<k>`,
since the owner and the architect number their runs on one issue
separately and the two would collide; the `Written:` line adds `on
#<n>` for the same reason; a note is a page at the wiki's top whose
header, by a brief's rule (`brief.Field`), holds `Subject:` and
`Source:`; the staleness count is `rev-list --count
<pin>..refs/heads/<base>` in the mirror, a pin that is not a plain
revision (an option, a range) counted as not found; an external
source gets its pin and its `Checked:` age, no count; the worker and
the reviewers are given the copy read-only and the `# Repository`
line, not the index, which the step asks of the owner and the
architect; the notes step reads the changed pages for the member's
secrets as `out.md` is read, and neither a hit, a conflict nor a
refused push changes the run's outcome (notes, not code: no review),
the copy kept for the maintainer when they were not pushed. On
Gitea the maintainer enables the repository's wiki and creates its
first page once, which makes its git repository. Step 23 (item 83)
moved the mount to `/work/.notes`, the run's copy to `<run>/notes`
and the result line to `notes-base <sha>`, for the wiki and the
notes alike.
