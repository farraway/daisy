# 0012. Trusted logins

Moved verbatim from `docs/swarm.md` section 14, item 12, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Trusted logins.** `trusted` (from revision 4 the maintainer's logins
and the swarm's, item 47) decides whose words reach a prompt and wake a
member; any other login's entry is listed with its content omitted.
Because the protected paths and the system-prompt split showed what an
injected comment could reach, revision 1's "filtering by author is
deferred" no longer held. The marker alone was the swarm's sign; it
now needs the token's own login too, so a commenter cannot forge it.
