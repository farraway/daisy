# 0139. Written authorities for a connected agent

Moved verbatim from `docs/swarm.md` section 14, item 139, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Written authorities for a connected agent (Step 21 of the plan
for the inbox requests F27–F35, 2026-11-15; S5).** Notify (item
136) wakes an agent, which connects (item 133) and asked for every
write, so a write the maintainer would approve the same way each
time still waited for them. **The authorities are a file in the
swarm repository**, `AUTHORITIES.md` at its top, each granted by
the maintainer and labelled (`A1`, …): the connect prompt reads it
in its briefing, runs a write that lies wholly within one on that
standing yes, and cites the label in the log line and in any
comment the write posts; everything else is asked as before, one
write at a time, and with no file nothing is granted. The rule that
a write waits for the maintainer's yes is widened (a yes in words,
or a standing one in the file) rather than given an exception per
verb, and the `swarm` skill's rule 3 points at the file under the
connect alone. Alternatives: a field of `swarm.json` (configuration
Daisy validates and acts on, while no code acts on an authority:
the agent does), or the authorities in the connect prompt (the
root's, the same for every swarm, while each maintainer grants
their own). **Daisy's code does not read it**, and `init` does not
seed it: an authority is the maintainer's word, and a seeded
default would grant what no maintainer approved. An authority
changes who said yes, not how a write is made: the skill's other
rules (the verbs as the only writes, no forge call of the agent's
own) stand.
