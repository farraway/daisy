# 0136. Notify, not poll

Moved verbatim from `docs/swarm.md` section 14, item 136, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Notify, not poll (Step 18 of the plan for the inbox requests
F27–F35, 2026-11-15; S2).** A live swarm's watch session polled
every 30 minutes and noticed a provider's fault pause late: what
waited for the maintainer was on the forge and in `status`, and
nothing told him. **The maintainer** is the first login of
`trusted` that is not the swarm's, not a new field
(`forge.maintainer` was the alternative): `init` writes the
maintainer's logins in the order given, so the order already
chooses whom to mention, and a second way to say it would need a
rule tying the two; the token behind `DAISY_MAINTAINER_TOKEN`,
whose login acts on the verbs, is read only at a comment command,
and the mention must not wait for one. **Which notes mention**:
those that put the issue in Waiting for you, the blocking notes and
the gate notes, the blocked gate's excepted, since the note that
blocked the issue precedes it in the same pass and two mentions
would notify twice; the mention is the note's last paragraph,
after the text every reader reads by its start or its first line,
and no note read by its end (the hands-on notes, `handsOn`) carries
one. **Once per item**: an item is known by its issue, its
`status` item and the id of the note that put it there, recorded in
`<state>/notified` (one file, rewritten whole as the items waiting
at each pass, in place of a directory of one file per item, so the
record of what left Waiting for you is dropped by the same write);
the note's id makes a block lifted and set again within one pass,
or a gating event's new gate note, a new item. **A failure is told
once**: the item is recorded whether the command succeeded or not,
so a broken command warns once per item, not every pass, and a
command that delivered and then failed does not deliver again; the
forge's mention and `status` still show the item. **The bound** is
`timeout`, the swarm's only bound on what it runs; host git and the
forge's requests have none, and a new cap was not invented: a
command that may hang is wrapped by the maintainer. A swarm fault's
pause and a notes branch have no issue to mention on, so notify is
their only telling. Tests: `TestSwarmApplyNotifies`,
`TestSwarmApplyMentionsNoOne`, `TestSwarmApplyNotifiesAFaultPause`,
`TestSwarmApplyNotifyTimeout`, `TestFormatNotify`,
`TestValidateNamesTheField` and `TestWizardKeepsTheFilesDeps`.
