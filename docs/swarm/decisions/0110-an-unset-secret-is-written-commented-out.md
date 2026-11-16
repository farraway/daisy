# 0110. An unset secret is written commented out

Moved verbatim from `docs/swarm.md` section 14, item 110, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**An unset secret is written commented out (Step 6a of revision 6's
plan, as built, 2026-11-10; the maintainer's choice, from Step 6's
Deferred item).** `init` wrote a variable unset in its environment
to `<state>/env` as `export <VAR>=''`; an assignment in the file wins
over the environment (section 3, Secret values), so that empty line
shadowed the value a unit's `Environment=` or the shell would supply,
the fallback's purpose (item 6). It now writes `# <VAR>=` (the
`export` left out: the file is read, never run, and either form reads
the same once filled in), a comment the reader skips, so the
variable is the environment's until the maintainer fills the line
in. The warning stays, reworded to say what to do: `⚠ <state>/env:
$<VAR> is not set; its line is commented out: fill it in, or set
$<VAR> in apply's environment`. An empty value counts as unset, as
every verb reads it. The reader is unchanged: a comment was always no
assignment, so `check-secrets` reports the variable as the
environment's, else `Not configured`. Tests: `init`'s claim test
leaves `<state>/env` as written and sets the architect's token in
`apply`'s environment, and the pass launches it with that value (red
while `init` wrote the empty assignment); `check-secrets` in a clone
reads a commented line as no assignment.
