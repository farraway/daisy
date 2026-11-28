# 0181. Three gaps in the maintainer's verbs

From Step S8 of the plan implementing the 2026-11-19 swarm roadmap (the
backlog items L356, L363 and L380). Built in `docs/swarm.md` section 11
(The maintainer's verbs, daisy swarm replan, and daisy swarm init's
Readiness warnings).

**Why.** Three verbs said less than the maintainer needed. `close`
rejected the job and left its pull requests open with no word of them,
and no pass reads a closed issue, so nothing named them again. A verb's
comment holding a command line (`/swarm …` on a line of its own) was
posted by `close` and `unblock`, though section 6 reads such a comment
as the maintainer's command to Daisy, never an entry a role answers;
only `replan` refused it. And `init`, which writes the `swarm.json`
that sets `gates.merge` and each repository's `check`, never warned of
a repository the merge step would merge with no check run, which
`apply` warns of and status lists under Waiting for you.

**The rule.** `close` reads the issue's pull requests before any write
and names each still open after its terminal, `#<n> pull <name>#<m>
left open (<url>): close it on the forge`; it closes none, the
rejection being of the job, not of the work on the forge. Every verb's
comment is refused when it holds a command line, naming the first one,
nothing posted (the check moved from `replan` into the comment the
three verbs share, its message now naming the comment). `init`'s
readiness warnings carry `apply`'s warning for each repository
`Unchecked` returns, after that repository's empty check.

**Alternatives.** `close` closing the pull requests itself would be a
write section 11 does not give the verb, and a pull request may hold
work the maintainer keeps; naming them leaves the choice where it was.
Refusing only a comment whose posted text is a command would accept
`replan`'s single-line `/swarm …` direction, posted as `Re-plan: /swarm
…`: the simpler rule refuses the words as typed, whatever the verb adds
to them.
