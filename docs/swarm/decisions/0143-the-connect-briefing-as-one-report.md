# 0143. The connect briefing as one report

From Step 33 of the plan remediating the 2026-11-15 design review (P21, 2026-11-16).

**`daisy swarm connect` prints the briefing of `/daisy swarm connect`
as one read-only report (Step 33 of the plan remediating the
2026-11-15 design review, 2026-11-16; P21).** The briefing of
`prompts/swarm-connect.md` rule 2 was eight calls of the agent's
(`status`, `journal`, a read of `AUTHORITIES.md`, a `git log`, two
`daisy metrics`, then the mark), each a permission decision and a
chance to skip one, over data Daisy already held, and the agent
regrouped a run-ordered journal by issue by hand. **The command
supplies the data, the prompt keeps the judgment:** it prints
`status`'s State, Running, Waiting for you, Held and runs lines, the
journal since the mark grouped by issue with `journal`'s Mark line,
each waiting issue's gate note (the reasons and the decision's verbs
the flag step already wrote, so the `Decide:` lines come with them;
`status --waiting` is unchanged), the open questions the swarm
answers (`swarm:question`, which `status` does not list: they wait for
the swarm, not the maintainer), `AUTHORITIES.md`, and the swarm's
metrics since the clone's newest commit to `swarm.json` or `prompts/`
and in all; the summary per issue, each item's decision and evidence,
what an authority grants and the comparison stay the agent's.
**`AUTHORITIES.md` is quoted, byte for byte, and acted on in no way**:
decision [139](0139-written-authorities-for-a-connected-agent.md)'s
"Daisy's code does not read it" becomes "reads it only to quote it":
no line of it changes what the command reads or prints, and an
unreadable file fails the command rather than reading as nothing
granted. **The metrics are the swarm's home's**: `<state>/home` names
the home its runs' rows are written to, so the report reads that
home's CSV, where decision [133](0133-the-connects-read-helpers.md)
had the agent run `daisy metrics` in its own workspace and leave the
metrics out when the swarm's home was another. **A gate note is shown
only while it is the current gating's** (the issue carries
`swarm:gated`, and the note is the blocked gate's for a blocked issue,
else one `apply`'s regating test finds no gating since), so a note of
an earlier gate is never read as this one's. A failure of `status`'s
reads, the journal's or `AUTHORITIES.md`'s fails the whole report, as
`status` does; the metrics' failure is a line of it. Alternatives: a
`--waiting` widened with the gate notes and Held (Step 14's open
question, left to the maintainer), which would still leave the
journal, the questions, the authorities and the metrics to the agent;
the whole `status` document inside the connect's (the facts the text
does not print, against the `--json` rule); a section per part that
fails on its own (`status` refuses a partial report because one
without the forge's facts reads as nothing waiting, and the
authorities likewise).
