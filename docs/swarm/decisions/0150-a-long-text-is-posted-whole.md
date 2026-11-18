# 0150. A long text is posted whole

From Step 38d of the plan remediating the 2026-11-15 design review (request F40, 2026-11-17).

**A text Daisy posts on the forge reaches it whole: one longer than
60 000 bytes is posted cut, its whole text attached to the issue as a
Markdown file the cut's note links, or, where the forge refuses the
file, its rest posted in numbered continuations after it (Step 38d of
the plan remediating the 2026-11-15 design review, 2026-11-17; request
F40).** The maintainer chose it: "I actually think we should either
split large output over multiple comments or post large content to a
file and link to that file from the comment." Until now Daisy cut the
text and named `runs/<run>`, a path only the host can read, so a
reader of the forge lost the rest (items 148 and 149 gave it back to
the members and to Daisy, not to the forge's readers). **One rule for
every text that carries a member's words:** a member's text as a
comment, a review or a pull request's description, and Daisy's own
notes that quote one, the reconciler's notes on a run (whose head may
name a member's paths or its question) and the gate note (whose Open
Questions and findings item 149's reads made unbounded). Not yet the
merge step's notes and the rulings note, which name the forge's answer
and the base's files, and whose retries read the thread's last comment,
which a continuation would change; nor a proposal's body, which opens
its issue and holds its `Brief:` and `Class:` lines. What Daisy writes after
a text is never cut, which widens the old rule for a worker's check
block to a note's last sentences and the gate note's decision and
mention: the actionable part and the maintainer's mention stay in the
comment, and a reader of a note's end (`blocks`'s tails) still finds
it. **The file first:** attaching needs no new seam beyond widening
`Attach` from a pull request to an issue (Gitea's asset endpoint is the
issue's, a pull request numbered as one), and the issue is the one
place that exists before every post, a pull request's description
included; the file is named `<n>-<role>-<k>.md` for a member's text
(`<n>-<role>-<k>-note.md` for the reconciler's note on that run,
`<n>-gate.md` for a gate note). The project's Gitea allows `.md`
(`GET /settings/attachment`, 2026-11-17: attachments on, `max_size` 100
MB). **Continuations second:** where the forge refuses the file
(`ErrNotAttached`: attachments off, size or type, or GitHub, which has
no upload), the rest follows in comments on the same thread, each cut
within the limit at a line break, or inside a longer line before a
character, the note counting them. A continuation is marked
`role=reconciler kind=continued` and its text begins `Continued from`:
a kind, so every rule that reads a role's entries, counts an entry or
shows one in a prompt passes it by as it passes a gate note, and a
heading line, so no reader of a text's first line (`QUESTION:`,
`ANSWER:`, a direction's head) takes a part of a member's text for one.
Daisy and the members read the text whole from the run's `out.md`
(items 148 and 149), so neither needs the continuations, and no prompt
shows a text twice. **Retries:** the collection records `attached
<file> <url>` or `not attached <file>: <error>` and each `continued
<file> <i> of <c>`, so a retried collection attaches once and builds
the same post; a continuation found on the thread by its body is not
posted again either. The gate note keeps its own retry (none on a first gating, the next
pass on a gating again): its file goes with it, attached again when the
note is tried again, and a continuation refused after the note is
posted is a warning, not tried again. The cut's note keeps
naming its run, `runs/<n>-<role>-<k>`, so items 148 and 149 read it as
before, and a note of the earlier form is still read. Alternatives:
continuations alone (comments the maintainer scrolls through on every
forge, though Gitea takes the file); the file alone (GitHub has no
upload, so a GitHub swarm would lose the rest); a continuation marked
with the member's role and run (every rule that reads a role's
entries, a run's verdict or the newest entry would need to tell it
apart); reassembling the text from the continuations for Daisy's reads
(the out.md holds it whole, and items 148 and 149 already read it);
a cap on a note's length (a limit the maintainer did not set). Tests:
`TestSwarmLongTextIsAttachedWhole`,
`TestSwarmLongTextContinuedWhenRefused`, `TestSwarmLongGateNote`,
`TestSwarmShortTextIsPostedAsBefore`,
`TestSwarmCutTextReachesTheMemberWhole` (both forms),
`TestLongPosted`, `TestLongNote`, `TestCutRun` and
`TestRenderCutNoteForms`.
