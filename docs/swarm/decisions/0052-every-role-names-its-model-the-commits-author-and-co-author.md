# 0052. Every role names its model; the commit's author and co-author are declared

Moved verbatim from `docs/swarm.md` section 14, item 52, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Every role names its model; the commit's author and co-author are
declared** (the maintainer's decision of 2026-11-05, built at Step 1).
`personas.<role>.model` is required (`""` and `-` refused), so neither
`apply` nor any later verb starts until each role has one, and the
example's adversary is a named Codex model; each persona's
`co_author` is the whole `Co-Authored-By:` value, set at `init` and
stamped verbatim with no logic at run time; `forge.commit_email` is
the swarm's commit address, defaulting to the account's email.
Alternatives: the model in the author name (no co-author shown by the
forge), the maintainer's identity, a fixed `daisy-swarm.invalid`
address (no link to the swarm's account on the forge).
