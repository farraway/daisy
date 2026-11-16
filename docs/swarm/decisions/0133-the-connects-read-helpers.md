# 0133. The connect's read helpers

Moved verbatim from `docs/swarm.md` section 14, item 133, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The connect's read helpers (Step 15 of the plan for the inbox
requests F27–F35).** `/daisy swarm connect` (`prompts/swarm-connect.md`,
a lazy prompt of a Daisy workspace, not a verb) briefs from commands
that write nothing. **Finding a swarm from anywhere**: every verb runs
in its clone, and nothing on the machine named the clone, so `apply`
records the clone it runs from in `<state>/clone`, and `swarm list`,
the one verb run anywhere, lists each `<state>` with that clone
(checked: a moved clone says so), its home and its pause, and marks
the swarm whose clone the working folder is in. Alternatives: the
prompt scraping `$DAISY_LOCAL/swarm/` and searching for clones (a
layout every prompt would repeat), or `init` recording the clone too
(a clone `init` made need not be the one the swarm runs from, and
`init`'s files are each a step's contract). Whether `apply` runs is
not shown: a flock is tested only by taking it, and `apply` takes
its own without waiting. **The journal since the last connect**:
`swarm journal` reads the journal from `<state>/notes` and the queue
(the maintainer's clone falls behind, the mirror lags the pass's
push), and the mark is a run, `<state>/connect-seen`, moved only by
`--mark` to a run the journal holds, so the briefing reads with
nothing written and marks only the lines it showed. Alternatives: a
time (`--since`; the lines are timed to the minute, so lines of the
mark's minute would be lost or shown twice), or a mark moved by the
read itself (the briefing no longer read-only, and a line collected
while the agent reads marked unseen). With no mark, the newest day's
lines, not the whole journal. **The metrics since a model or prompt
change**: `daisy metrics` gains `--swarm <owner>/<repo>` and `--since
<time>` (the rows of that swarm's runs, written since); the prompt
takes the time from the clone's newest commit to `swarm.json` or
`prompts/` and compares with the swarm's whole record. A row's
`prompt_hash` cannot say when a prompt changed: every `system.md`
opens with its issue, role and run, so each run's hash differs. The
rows are the home's the workspace is bound to; the prompt leaves the
metrics out when `swarm list` names the swarm's home as another.
**Faults and pauses** need no helper: `status`'s `State:` line and
Throughput, and the journal's `fault` lines. Tests:
`TestSwarmConnectBriefing`, `TestSwarmListStates`,
`TestSummarizeFilter` and `TestTemplateListsSwarmConnect`.
