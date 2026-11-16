# 0057. The brief

Moved verbatim from `docs/swarm.md` section 14, item 57, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The brief** (revision 5; the synthesis's G6, and the maintainer's
decision of 2026-11-06 that the swarm is perpetual and fed by
briefs). The four levels of section 1; the brief's form in section 4,
`<ws>/docs/briefs/` the convention with the roadmap as brief 0; the
parsed surface is the `Status:` line and the `## ` headings, which
the forge milestones of Step 9 mirror, the priority being stated in
the header's words for the owner to read; the owner reads every
`active` brief in its batch prompt, proposes toward the
highest-priority brief's first unmet milestone with each body opening
`Brief: <path> — <milestone>`, and reports `Met: <path>` in its
output, which stays in the run's `out.md` for `status` to show from
Step 8. The briefs are read from the resolved resource paths on the
host as the batch prompt is rendered: the one addition to what
`apply` reads, no forge call and no new record. Alternatives:
a `brief` verb, or the brief on the forge (a wiki page, a pinned
issue), both rejected since the brief is the maintainer's file and the
product repository is the shared surface.
