# 0003. Several pull requests per issue, one per slice

Moved verbatim from `docs/swarm.md` section 14, item 3, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Several pull requests per issue, one per slice.** Branch
`swarm/<n>-<s>`, body `Refs #<n>` (from revision 4 its `Refs` line, item 49), never `Closes`; the current pull
request is the newest slice's. The step text wrote `swarm/<n>-<k>`;
`<k>` is already the run's number here, so the slice is `<s>`. With
nothing closing on a merge, `base` no longer has to be the forge's
default branch, and revision 1's `ls-remote --symref` check and its
error row are gone.
