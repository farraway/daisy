# 0022. Swarm faults are not failures of the work

Moved verbatim from `docs/swarm.md` section 14, item 22, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Swarm faults are not failures of the work.** A launch failure,
Docker's own exits 125–127, a prompt, resource or slot refused, a
clone that failed, a provider's authentication or quota error, a wait
that failed and a check that cannot start (section 7.7's table is the
full list) write nothing to the forge; two in a row
pause the swarm with the reason. In revision 1 each was a T9 per issue,
so one stopped daemon or expired key blocked the whole board (*Software
Engineering Is Becoming Factory Engineering*: humans come in when the
work gets stuck, not the plumbing; *What It Actually Takes to Build a
Software Factory*: provider failure is the case for fallback, which is
not in version 1).
