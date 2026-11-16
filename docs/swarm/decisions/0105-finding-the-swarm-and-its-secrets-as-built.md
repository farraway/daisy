# 0105. Finding the swarm and its secrets, as built

Moved verbatim from `docs/swarm.md` section 14, item 105, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Finding the swarm and its secrets, as built (Step 2 of revision
6's plan, 2026-11-10).** One function, `declare.Find`, decides the
layout (the plan's risk R4): the clone first, wherever a workspace
lies, then the old layout's (section 11, Moving a swarm into its
repository); the cli's
`swarm` rows no longer resolve a workspace, each verb reading
`$DAISY_ROOT` and `$DAISY_LOCAL` and then the lookup. The old
layout is found by the binding's folder alone, no home or identity,
and a workspace whose `swarm/` holds no `swarm.json` is no swarm for any verb
but `init`, which writes it there (and, outside a workspace, keeps
the workspace lookup's refusal) until Step 6 makes a clone; a
broken binding is passed over, not reported, as no swarm. Until
Steps 3 and 4 replace them, a clone's `repos[].path` and
`resources` are read relative to the clone, the base the old layout
gave the workspace, so the smallest seam that runs every verb from
a clone is a product's clone inside it; their messages still say
"workspace". `<state>/env` is refused when its mode has a group's
or other's read bit (`0040` or `0004`), before it is read, by every
verb that reads it (`init`, `status`, `apply`, the maintainer's;
`pause` and `resume` read none), its mode written as four octal
digits; a line it cannot be read past is named by its path. The
`.env.sh` in a clone is warned of once per `status` and once at
`apply`'s start, before the lock. `daisy check-secrets` learns the
swarm's variables: run in a swarm (the same lookup), after the
workspace's report when it runs in one, it adds `Swarm
<owner>/<repo> (<secrets file>):` and one row per variable,
`forge.token_env`'s, each `keys` `env`'s in provider order, then
`DAISY_MAINTAINER_TOKEN`, each `✅ <VAR>: <first 8 characters>...`
or `⚠️  <VAR>: Not configured`, resolved as the verbs resolve them;
`swarm.json` is read for its forge and keys alone (one that cannot
be read is `⚠ swarm.json: …; the swarm's variables are not checked`
on stderr, not a failure); `<state>/env` refused is a `❌` row and
exit 1; outside both a workspace and a swarm it refuses as before.
The unit's description names its root as what it is, `for the swarm
repository %f` in a clone and `for the workspace %f` in the old
layout.
