# 0152. Every model in use priced from its provider's page

From Step 3 of the plan implementing the 2026-11-19 swarm roadmap (R0, measure first, 2026-11-19).

**The four models the live swarm ran without a price (the 2026-11-19
roadmap's R0) have a list price, and every model in the table its
cached input apart, each from its provider's pricing page as read on
2026-11-19 (section 9, List prices).** Until now `Prices` held
`claude-fable-5-1`, `claude-opus-5-5` and `claude-sonnet-5-5` without a
cached rate, and `gpt-6.1-sol` with one, so the runs of
`claude-fable-5-2` (the architect), `grok-4.7`, `gpt-6.2-astra` and
`gpt-6-luna` carried no estimate, and a Claude run that read its cache
none either (decision [140](0140-every-providers-input-count-holds-its-cache.md)),
as the live architect's do (73-architect-1: 3,200,374 of its input
read from the cache). The table now holds
`claude-fable-5-2` ($8 input, $0.20 cached, $40 output per million
tokens), `gpt-6.2-astra` ($8, $0.40, $40), `gpt-6-luna` ($0.10, $0.01,
$0.50) and `grok-4.7` ($2, $0.50, $6), and the cached rates of
`claude-fable-5-1` ($0.25), `claude-opus-5-5` ($0.20) and
`claude-sonnet-5-5` ($0.10): Anthropic's "cache hits and refreshes",
OpenAI's Standard table and xAI's model page. A subscription's runs
are priced at these rates, billed-equivalent, as before.

**One rate per model, below the bill where a provider charges more.**
xAI doubles `grok-4.7`'s rates for a request whose prompt reaches
200,000 tokens, and Anthropic and OpenAI price a cache write above the
input rate. A result reports a run's sums, not each request's prompt or
which write lasted how long, so the estimate prices every request at
the table's rate, the cache's writes at the input rate: a lower bound
for such runs, as it already was for a cache write. The alternative,
leaving `grok-4.7` unpriced until an adapter reports each request's
prompt, would keep the reviewers' runs without any estimate, where a
cited lower bound shows most of their cost; it was not taken.
**No reprice here**: the rows already written keep their cells until
`daisy swarm metrics --reprice` supersedes them (section 11). Tests:
`TestEstimate` (each priced model, its input and output and its cached
input at their rates; a model with no cached rate and one with no
price).
