# 0148. A text cut on the forge reaches the member whole

From Step 38b of the plan remediating the 2026-11-15 design review (request F40, 2026-11-17).

**A member's output that Daisy cut on the forge reaches each later
member that acts on it whole, as a copy of the run's `out.md` in the
member's own `/work/.swarm/outputs/` (Step 38b of the plan remediating
the 2026-11-15 design review, 2026-11-17; request F40).** Daisy posts at
most 60 000 bytes of a text and ends a longer one with
`[truncated by Daisy; the whole output is in runs/<n>-<role>-<k>]`
(section 7.5), a path on the host. Every member reads the earlier
members' texts from the forge's threads, so on swarm-gridapp#143 the
worker read a 70 333-byte plan without its last third, its own
`## For the builder` section among it, and blocked asking for a
directory it cannot reach. **The principle, not the case:** any swarm
entry a prompt shows, in any role and section (the plan under `# Plan`
or in `# Thread`, a review in full or as its `[blocking]` lines, a pull
request's description or comment, an answer under `# Questions`), whose
text ends cut in the note naming its own run, has the note rewritten to
name the member's copy, and the launch copies that run's `out.md` there
(section 7.3, A text cut on the forge). The Task text names each file,
the plan by that name (section 7.4), since a note at the end of a long
text is easily passed over. **Which run:** the one the entry's marker
names on the issue, `<n>-<role>-<k>`, never one a note's text names, so
a text that quotes another run's note, a trusted login's comment and a
member's output cannot have Daisy copy another run's output; the plan's
run is the accepted plan's (the architect's newest plan entry, as the
prompt finds it), whatever revision that is. **What is copied** is the
run's `out.md`, read as a regular file (`Lstat`, as the collection reads
it), from a run whose result holds no `failed: secret in output`: the
text the collection searched for the member's secrets and posted, so the
copy gives the later member nothing the forge would not have held but
for its limit. Alternatives: posting the remainder as further comments
(the forge's limit holds for each, the thread grows by as many comments,
every reader of the thread, the maintainer included, pays for them, and
each role's compaction, which keeps one entry per role whole, would have
to learn to join them); raising the limit (a forge caps a comment
too, GitHub at 65 536 characters, so a long enough text is cut
whatever Daisy's limit); mounting the earlier run's directory (a new mount,
section 7.2, holding its prompts and streams, for one file a copy
delivers through the mount the run has, the pattern of decision
[147](0147-screenshots-reach-the-pull-request-and-the-reviewers.md));
a fixed `/work/.swarm/plan.md` (the request's example: one name per
role-and-run serves the plan, a review and a report alike, and names
the run the prompt's heading names). **Not changed:** Daisy's own reads
of a posted plan (its slices for the worker's branch and the merge
step's count, the gate note's summary) read the forge's text, so a plan
whose `## Steps` the cut reaches is read with fewer slices; that is
recorded as Deferred, since reading it from the run asks for a file
system where those reads have none. (Amended by item 149: Daisy's own
reads of a text it cut read it whole from the same run, the plan's
slices and the gate notes among them, and a review shown as its
`[blocking]` lines alone has them listed whole, with no note.) Tests:
`TestSwarmCutTextReachesTheMemberWhole`, `TestSwarmUncutTextIsNotCopied`,
`TestRenderCutNote` and `TestOutputs`.
