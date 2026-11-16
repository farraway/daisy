# 0033. Paused, `apply` still collects

Moved verbatim from `docs/swarm.md` section 14, item 33, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Paused, `apply` still collects.** The plan's "stop if `paused`"
is read as "stop launching" (and, from revision 2, merging): members
already running are collected and posted, so a pause loses no work.
