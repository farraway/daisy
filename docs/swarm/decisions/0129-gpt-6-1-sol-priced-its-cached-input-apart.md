# 0129. `gpt-6.1-sol` priced, its cached input apart

Moved verbatim from `docs/swarm.md` section 14, item 129, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`gpt-6.1-sol` priced, its cached input apart (Step 11b of the
plan for the inbox requests F27–F35, 2026-11-15; the maintainer's
request at its Step 11 checkpoint).** The live swarm's `codex` runs
had no estimate, `gpt-6.1-sol` having no price. Its rates are $2
input, $0.10 cached input and $10 output per million tokens (section
9, List prices). `codex` reports `input_tokens` holding
`cached_input_tokens`: in the 474 `turn.completed` events of the
live state when this step read it, the cached part is never more
than the input and is most of it (95% in all, 81% at least;
93-worker-1: 3390208 of 3476833), so pricing the input whole at
$2 would overstate a run more than tenfold ($7.08 for 93-worker-1, $0.64
priced apart). **The count's place**: the result's `cost` line gains
` cached <c>`, which `status`'s reader of the line ignores and the
record reads; the row's `tokens_in` stays the CLI's count, so a
`codex` row's `tokens_in` holds its cached input where a `claude` or
`grok` row's leaves it out, and the cached count goes to the notes,
`<c> cached input`, as the turns do, since a column would make every
existing CSV's header refuse (item 118). **The count unknown**: a
`codex` line without it (every run recorded before this step, and a
run collected by this Daisy whose `cost` line an earlier one wrote,
across a restart) has no estimate rather than one priced whole; the
provider says so (`InputHoldsCached`), since `claude`'s line without
a count is complete. A model with a price and no cached rate (the
Claude models) is not estimated for cached input it reports, so no
cached rate is guessed; `claude` reports none on the line (its cache
reads and writes lie outside `input_tokens`). (Amended by item 152, which gives the Claude models their cached rates and prices `claude-fable-5-2`, `gpt-6-luna`, `gpt-6.2-astra` and `grok-4.7` from their providers' pages.) **No reprice**: a
price added reprices no row already written, and `swarm metrics
--backfill` writes none for a run the CSV holds. A `--reprice`
appending a superseding row (`metrics` keeps the CSV append-only and
reads a later row of a key over an earlier one) would need the
cached count of runs whose `cost` line lacks it, read again from
`<run>/stdout` through the adapter, an `Append` that writes past a
held key, and the held row's cells carried over (the live row's
commits and time): not taken here. Tests:
`TestSwarmApplyPricesCachedInputApart` (an architect on `codex`
whose output is 93-worker-1's usage: `cost - tokens 3476833/12969
turns 1 cached 3390208`, `tokens_in` 3476833, the notes'
`3390208 cached input`, the journal's `(3390208 cached)` and the
estimate $0.641961), `TestSwarmMetricsBackfill` (an old `codex`
line without the count: no estimate), `TestEstimate` and
`TestReport` (a turn without the count leaves it unknown).
