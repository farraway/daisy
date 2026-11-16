# 0010. The fast lane

Moved verbatim from `docs/swarm.md` section 14, item 10, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The fast lane.** A job relabelled straight to `planned` has no plan;
the worker's Task text then takes the issue body and the maintainer's
comments as the job, keeps it small, and asks for a plan with
`BLOCKED:` when it needs one (*Software Engineering Is Becoming Factory
Engineering* and *Why Software Factories Fail*: small, unambiguous jobs
go straight to implementation). Revision 1 told that worker to follow
"the latest plan in the thread", which did not exist.
