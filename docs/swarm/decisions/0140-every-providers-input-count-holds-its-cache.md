# 0140. Every provider's input count holds its cache

From Step 28 of the plan remediating the 2026-11-15 design review (P22, 2026-11-16). It amends decision [129](0129-gpt-6-1-sol-priced-its-cached-input-apart.md) where the two differ.

**Every provider's input count holds its cache, and a reprice
supersedes the rows written before (Step 28 of the plan remediating
the 2026-11-15 design review, 2026-11-16; P22).** The metrics could not
compare providers: a `codex` row's `tokens_in` held its cached input
(1–2.8M a run), a `claude` row's left it out (30–90 a run; the review
read the architect's 44 as a defect of the `codex` adapter, but the
architect runs on `claude`, whose result reports `input_tokens`, the
uncached part alone, beside `cache_creation_input_tokens` and
`cache_read_input_tokens`, which the adapter did not read), and 484
`gpt-6.1-sol` rows written before decision 129 had no estimate.
**The adapter normalizes**: `Report.InTokens` is every input token for
every provider (`claude`'s and `grok`'s three counts added, `codex`'s
`input_tokens`), `CachedTokens` the part read from the cache, and
`InputHoldsCached`, the flag a reader had to know, is gone. The `cost`
line keeps its form, ` cached <c>` now written for every provider
that reports it. **The CSV gains `tokens_cached`**, at the end of its
header: a file under the older header is read with the column empty,
and the next append rewrites its header line alone, its rows kept as
they are, a cell short, so the reason items 118 and 129 kept counts
in the notes, that a new column would make every existing CSV's
header refuse, no longer holds; a header that is not this one or this
one cut short is still refused. The row's
`tokens_total` is `<in>` and `<out>` added, and the notes no longer
carry `<c> cached input`. **The estimate needs the cached count**: a
line without it (a `claude` or `grok` run recorded before this step,
whose `<in>` leaves the cache out) has no estimate, where it had a
lower bound; a model with no cached rate, every Anthropic model in
the table, has none for a run that read its cache, so a `claude` run's
estimate is now mostly empty beside the cost it reports, rather than a
figure far below it. No cached or cache-write rate is added for the
Anthropic models: none is in the root or its sources. (Amended by item 152, which gives the Claude models their cached rates and prices `claude-fable-5-2`, `gpt-6-luna`, `gpt-6.2-astra` and `grok-4.7` from their providers' pages; cache writes stay at the input rate.) **The rows
already written are superseded, not rewritten**: the CSV stays
append-only, and normalizing at read time cannot work, since a row
does not hold its cache's tokens; `daisy swarm metrics --reprice`
reads each collected run's `<run>/stdout` through the adapter of its
row's provider and appends a row superseding the held one when its
usage cells differ, every other cell carried over, its notes ending
`, repriced`, and a second reprice writes nothing (section 11). The
alternatives: a reprice of the `codex` rows alone, leaving the
`claude` rows' `tokens_in` uncomparable; the cached count from the
notes, which only rows since decision 129 carry; or rewriting the
rows in place, which would end the CSV's one guarantee. Tests:
`TestReport`, `TestSwarmMetricsReprice`, `TestSwarmApplyRecordsEachRun`,
`TestSwarmApplyPricesCachedInputApart`, `TestSwarmMetricsBackfill`,
`TestFirstHeaderReadAndExtended`, `TestSupersede`.

**As built: a cached count over the input (Step T5a–d of the plan
implementing the 2026-11-19 swarm roadmap, 2026-11-21).** Every input
count holds its cache, so a `cost` line whose `cached <c>` is over its
`<in>` reports a count no run can have: its row now has no estimate,
where the rest of the input was priced as a negative count. A line of
eight words whose seventh is not `cached`, or whose count is not a
number, has none as before (`TestRunRowCostLineForms`).
