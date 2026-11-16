# 0047. The swarm's own forge account

Moved verbatim from `docs/swarm.md` section 14, item 47, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The swarm's own forge account** (from a reading of an outside
project that runs its forge automation as a bot with its own token).
Chosen: `forge.token_env` names a swarm account's token (a Gitea bot
user in an organization team with Write), one account for all five
roles; `trusted` is the maintainer's logins, asked at `init` after
`Login()`, then the swarm's, and an answer naming the swarm's login is
refused. The marker rule is unchanged, its author now the swarm's
account. Gains: the maintainer's approval is a formal review, which a
branch protection rule counts (section 12); authorship tells the
swarm's entries apart; the maintainer's token stays off the swarm's
host (reversed by item 67: the maintainer keeps the token in
`<ws>/.env.sh`, and the boundary is the code paths and the two
refusals). `Review` tries the formal event in case a forge accepts it;
the instance refused it on the swarm's own pull request (`422`, Step
5's smoke of the swarm build; section 10). Alternative: the
maintainer's token, as in
revisions 1–3, under which no one could approve a swarm pull request.
