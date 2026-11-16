# 0001. Why build, and the one merging configuration

Moved verbatim from `docs/swarm.md` section 14, item 1, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Why build, and the one merging configuration.** The swarm is built
rather than adopted for the five reasons of section 1 (*Orchestras, Not
Factories*: build only where one holds knowledge the defaults lack).
Rejected: members as CI jobs triggered by label events, the execution
plane the forge's own runner offers. It inverts the credential model
(the provider key would live in CI secrets, readable by any workflow a
branch carries), ties the swarm to one forge's CI dialect, and needs
the webhooks version 1 does without. Rejected too (revision 3): hosted
agent machines as the execution plane, since the clone and the
provider credential would sit on a third party's machine, against
section 1's "no hosted service holds the code", and such hosts take
API keys, not subscription logins. `gates.merge: false` is the one
configuration in which the swarm merges (section 12), reversing the
plan's "no merging swarm" non-goal at the maintainer's choice, never by
default.
