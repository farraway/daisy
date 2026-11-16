# 0006. Additions to `swarm.json`

Moved verbatim from `docs/swarm.md` section 14, item 6, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Additions to `swarm.json`:** `base` (default `main`), `interval`
(default `5m`; the plan names an interval but no field), `keys` (the
host variable per provider; the plan says the names are declared but
its example has none; from revision 3 an `env` variable or a slot
count, item 40); from revision 2, `check`, `protected`,
`trusted`, `gates` and `budget` (items 3–5, 12, 21, 23). Effort is
checked as one lowercase word (the plan's "bad effort" and "passes
through" reconciled); a model may not be `""` (until Step 1 it meant the CLI's
default, and was the adversary's; item 52); a
model with whitespace is refused (the plan validates only adapters; one
argv word is the reason). A secret is resolved from `.env.sh`, then the
process environment (the plan names `.env.sh` only; the fallback serves
a timer's `Environment=`). The default sympathetic reviewer is
`anthropic` (the plan's example has `xai`, which had no adapter then;
`xai`'s, Step 22 of the swarm build, left the default as it was).
