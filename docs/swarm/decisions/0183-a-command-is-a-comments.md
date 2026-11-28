# 0183. A command is a comment's

From Step S10 of the plan implementing the 2026-11-19 swarm roadmap (the
backlog item L365). Built in `docs/swarm.md` sections 6 (Trust), 7.3
(`# Recently closed`) and 11 (`swarm status`'s `maintainer comments`).

**Why.** Daisy reads commands from the comments of an issue and of its
open pull request alone (section 6, The comment commands), yet the
trust rule left out of the count every entry holding a command line: a
review or a line comment with a `/swarm …` line, which runs nothing,
was not counted either, so a role took the entry before it as the
newest. Two readers had the opposite gap: the batch prompt's closed
issue quoted a command as the maintainer's last word, and status
counted command comments among the maintainer's entries on a pull
request.

**The rule.** A thread's entry records whether it is a comment (an
issue's or a pull request's); only a comment holding a command line is
a command comment, left out of the count. A body, a review or a line
comment holding one is counted as any entry is. The closed issue's
quoted comment and the pull request's maintainer comments leave out
command comments.

**Alternatives.** Dropping command comments from the threads would
keep the entry type as it was, but the prompts show a command comment
as any trusted comment (section 6), so the threads must keep them. A
kind beside each entry (comment, review, line comment, body) would say
more than the one rule needs; the entry carries a single flag, set by
constructors from the forge's types.
