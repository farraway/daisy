# 0005. `budget`

Moved verbatim from `docs/swarm.md` section 14, item 5, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`budget`** (`{per_day, total}` in USD, from the per-run cost lines of
item 28) pauses the swarm when reached and is required when any gate is
off. Read literally: reaching `per_day` writes `paused` like `total`,
and the maintainer resumes. Alternative: a `per_day` hold that releases
itself at the next UTC day, which an autonomous swarm returning daily
might prefer. A provider that reports no cost counts as zero, so the
budget is a lower bound for its runs (Step 39's run records confirm
that `codex` reports tokens and no cost). From revision 3, under a
subscription login the figure is notional and the account's quota is
the binding limit, met as a provider fault (section 3). Not built:
`swarm.json` validates it and no pass reads the spend against it
(section 3, The budget).

**Amended by decision [170](0170-capacity.md)** (R4, specified ahead of its build, Step 12 of the plan implementing the 2026-11-19 swarm roadmap): the money `budget` stays validated, shown and not enforced; a provider's credential may be paced by a token budget per day or week declared in `keys.<provider>.tokens`, no default (`docs/swarm.md` section 7.10).
