# 0069. Step 11's second pass

Moved verbatim from `docs/swarm.md` section 14, item 69, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**Step 11's second pass.** **`out.md` is read only as a regular
file** (7.5): a member could otherwise leave a link to a host file,
`.env.sh` or another slot's login, and have `apply` post it;
**`forge.token_env` may not name a provider's credential variable**
(section 3), which a member is given; **a `null` where a field is not
optional is the wrong type** (section 3), not a gate silently off;
**the pull request's body in a prompt is its opener's entry** (7.3):
an untrusted opener's is omitted; **`unblock` reads the last
`(was swarm:<label>)` of the note** (section 11), since a failure's
reason may quote a member's path; **the maintainer's verbs refuse a
pull request's number** (`Issue.Pull`, sections 10 and 11).
