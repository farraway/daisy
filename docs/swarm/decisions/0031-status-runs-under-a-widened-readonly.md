# 0031. `status` runs under a widened `ReadOnly`

Moved verbatim from `docs/swarm.md` section 14, item 31, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`status` runs under a widened `ReadOnly`.** Today `ops.ReadOnly`
refuses every HTTP request and every `Exec` but git reads, so the plan's
"`status` runs under `ReadOnly`" cannot read the forge or `docker ps`.
Chosen: Step 27 lets `GET`/`HEAD` requests and `docker ps` through
`ReadOnly`. Alternative: `status` under `Real`.
