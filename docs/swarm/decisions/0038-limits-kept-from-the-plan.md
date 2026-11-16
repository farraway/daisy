# 0038. Limits kept from the plan

Moved verbatim from `docs/swarm.md` section 14, item 38, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Limits kept from the plan:** the container label and name carry
`<owner>/<repo>` without the forge host, so two forges holding the
same `<owner>/<repo>` on one machine would share a running set.
