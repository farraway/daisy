# 0123. The default persona prompts take the general rules

Moved verbatim from `docs/swarm.md` section 14, item 123, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The default persona prompts take the general rules (Step 3 of
the plan for the inbox requests F27–F35, 2026-11-14; request F34).**
A live swarm's maintainer added rules to its five persona prompts
from 2026-11-10 to 11-12, each after a run went wrong in the way it
names; an edited prompt is kept out of every later default (`prompt
build` keeps it, item 117), so a swarm that learnt them by hand
stopped receiving Daisy's own fixes. The rules that state no swarm's
own fact join the defaults (section 4). The owner proposes a job
only once every job it depends on is closed (a proposal can be
accepted and started at once: a routine job, or any with the triage
gate off), prefers next a job whose files no open job is changing,
and names as a question, rather than proposes, a job whose outcome
is a change to a protected path. The architect says, for each test a
slice names, whether it shows new behaviour or guards kept behaviour
and how a guard is shown to bite; states a performance claim as a
scaling or a ratio, never a time measured on one machine; writes
`BLOCKED:` when the job builds on work not yet on `base` that none
of its slices builds; and holds back a slice whose check can pass
only after the maintainer changes a protected path. The worker opens
no pull request on a failing check (it fixes it, or writes
`BLOCKED:` when only the maintainer can make it pass), reports a
claim done only when a test committed on the head shows it, and
changes every text that describes the behaviour it changes, folded
into item 113's documentation duty rather than a second bullet. Both
reviewers, where the request's swarm had some only in the
sympathetic's prompt, try for each new test the change to its
subject that should break it, request changes on a head whose check
fails, naming whose move would make it pass, and write every
finding, however small, as its own marked line. **Protected paths**
are named generally, "a path the repository protects", whose list
the Task texts name (7.4); the request's "CI workflows, `AGENTS.md`,
the ADRs" were one repository's. The owner's batch text's class rule
(a job that changes a protected path is architecture) stands beside
it: that rule classes a job that touches one, the persona keeps out
a job whose outcome is one. **No Task text is repeated** (item
113's rule): the adversary's rollback (7.4) counts a build failure
as failing, so a test of a new function passes it however little it
asserts, which the probe on the test's subject catches; the
reviewers' Task text makes each finding a marked line, and the
persona adds that the prose holds none. What stays a swarm's own is
its reading list (F35) and its rules about its own practice pages,
styles and browser code. **`prompt build`** now moves a swarm whose
prompts are an earlier default to these (each role's new hash
appended to `defaultHistory`, item 117); a swarm whose prompts carry
the rules by hand, as the request's swarm's do, is edited and stays
as it is until its maintainer rebuilds it. Tests:
`TestDefaultPromptsAreSectionFours` and
`TestDefaultHistoryHoldsEachCurrentDefault`.
