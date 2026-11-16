# 0065. The spec read against the code built in Steps 1–7

Moved verbatim from `docs/swarm.md` section 14, item 65, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The spec read against the code built in Steps 1–7 (Step 8 of the
swarm build).** Sections 7, 11 and 13 were read against the code;
each difference is resolved here. Toward the code: section 7 says a
member has the network and only the check runs without one; a
worker starting a slice refuses on any slice line of the plan that
no longer resolves, naming each (7.1 step 2, as 7.7 already said);
the clone's `exclude` line is written before its hook (7.1 step 3);
a resource's mount source is resolved again at the launch (7.2); an
earlier pull request may read `open` (7.3); the batch's `BLOCKED:`
reason is held as `blocked: <question>` (7.5); a failed wait on a
container is a fault with its own row, and the pause after two
faults counts collected runs, the newer collected by this process
(7.7, item 62); a pass reads the running set before it collects, a
`docker ps` failure ending it before either, and a check's timeout
has its stdout line (section 11); `Adapter` holds fifteen methods,
`Board` waiting (section 13); the secret check's As built names the
cases its tests cover. Toward the spec, the code fixed: the empty
field check of a prompt read every line, so a quoted body's
`- Clone ` bullet or a fenced `# Issue` line was a *prompt refused*
fault, and two such runs would pause the swarm; it now reads
Daisy's own lines (7.1 step 2), and a quoted text that leaves a
fence open has it closed at its end (7.3), so that no fence hides
a heading of Daisy's from the check or from the member. A provider with no
`credentials/<provider>/` directory, where no login was ever made,
was a plain error that ended an `--once` pass; it is now *slot
refused* with the login's hint, as a slot without `auth.json` is
(7.1 step 4). Section 13 gains what the first real run established
and what it did not.
