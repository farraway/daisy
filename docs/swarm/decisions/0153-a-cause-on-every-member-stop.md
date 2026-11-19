# 0153. A cause on every member stop

From Step 4 of the plan implementing the 2026-11-19 swarm roadmap (R0, measure first, 2026-11-19).

**A member that stops names why, in one line `Cause: <word>` under its
`BLOCKED:` or `QUESTION:` line, and the run's metrics row records the
word in a new last column, `cause` (sections 7.4, 7.5 and 7.6).** The
roadmap classified 89 member stops by hand to find that the plan was
the largest cause (52 of them); nothing in the runs said so, and every
later item of the roadmap measures itself by stops of a cause. The
words are the roadmap's fixed vocabulary, in its order: `plan`,
`prerequisite`, `main-red`, `environment`, `forge-fact`, `kept-work`,
`ruling`, `scope` and `capacity`, each defined in one line in 7.5; no
word is added here.

**The member names the cause; Daisy does not infer it.** The member
alone knows why it stopped, and the collection reads only the line
under its stop's line: the stop's line is the output's first that is
neither blank nor a `Met:` line (the owner's batch writes its `Met:`
lines first), the cause's line the next one that is not blank. The
alternative, a model classifying each stop's text after the run, would
put a judgement Daisy cannot check into a record meant to be counted;
it was not taken. The line is asked for once, in the closing paragraph
every Task text ends with, so every role and every text that asks for
a `BLOCKED:` or a `QUESTION:` line (the owner's and the QA's "as the
only line" among them) asks for it in the same words; the persona
prompts, which state a stance and not what to write (section 4), are
unchanged. The words and their glosses are one list in the code
(`reconcile.causes`), from which the sentence is built.

**Only a member's stop has a cause, and an unknown word is recorded
empty with a warning.** The cell is read for a run whose outcome is
`blocked` or `question`, the outcomes of a member's stop alone; any
other run, and a stop without the line, records it empty, so a run
recorded before this step and one whose member ignored the line read
alike, as unknown. A word outside the vocabulary is recorded empty,
never as written, so the column holds the vocabulary alone and a count
by cause needs no cleaning; the warning, `⚠ #<n> <role> run <k>: cause
left empty: "<word>" is not one of …`, names it for the maintainer.
The line changes nothing the collection does: the stop is posted and
moved as before. Daisy's own stops (a fault, a failure, a conflict)
and the maintainer's retagging on `unblock` are not this step's: their
cells stay empty until they are. The column is added at the end of
the header, as `internal/metrics`' rule for a column has it: a CSV
under the 29-column header is read with `cause` empty and its header
line rewritten at the next append. Tests: `TestSwarmApplyRecordsTheStopsCause`,
`TestStopCause`, `TestStep30HeaderReadAndExtended`, `TestRenderArchitectReady`.
