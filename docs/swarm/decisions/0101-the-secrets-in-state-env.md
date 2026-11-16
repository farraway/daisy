# 0101. The secrets in `<state>/env`

Moved verbatim from `docs/swarm.md` section 14, item 101, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The secrets in `<state>/env` (the maintainer's D5).** Outside
every repository and every workspace, beside the state it serves,
mode `0600`, refused when a group or other may read it. "Written by
`init`" was read as written with the values `init` finds in its
environment (the maintainer exports them for the run, as `init`
needs both tokens then), each unset one written empty and warned of,
and never rewritten: the one file Daisy writes that holds a token,
which sections 3 and 8 now name as the exception. The format and
the reader are `.env.sh`'s, and the process environment stays the
fallback.
