# 0015. Severity per finding

Moved verbatim from `docs/swarm.md` section 14, item 15, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Severity per finding.** Each finding is `[blocking]` or
`[non-blocking]`, `request-changes` only on a blocking one, and a
verdict its findings contradict is *bad verdict*; from round 2 a
reviewer first says whether its earlier blocking findings are resolved
and raises new ones only on code changed since. Without a bar, a
reviewer told to find fault always does, and most jobs would reach
`max_rounds` (*Why Software Factories Fail*: adversarial review raises
the floor and does not converge by itself).
