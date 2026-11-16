# 0087. `grok` sees only its working folder

Moved verbatim from `docs/swarm.md` section 14, item 87, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**`grok` sees only its working folder (Step 25a of the swarm
build, 2026-11-09).** The first `xai` run, a sympathetic reviewer
started in `/work` with two resources, exited 1 before any work:
`Error: Couldn't create session: Invalid params:
"additionalDirectories is not supported: a session works only in its
cwd"`. `grok` 1.0.70 refuses `--add-dir` in headless mode whatever
`--trust` says, refuting section 9's *to confirm at the first `xai`
run*; its owner and architect, started in `/work/<first>`, reached
`/work/.swarm` and the sibling clones only by `--add-dir`, so they
failed too. **The layout:** the resources are mounted at
`/work/.resources/<path>` instead of `/resources/<path>`, for every
provider (one layout rather than one per provider; a `claude` or
`codex` member started in `/work` now needs no `--add-dir` for
them, and their owner and architect still pass one); the mount point
is made in `<run>/work` before the launch, as `.notes` is (an empty
file for a resource that is a file), and
`/.resources/` joins `/.swarm/` and `/.notes/` in a clone's
`.git/info/exclude`, so a worker's `git add -A` cannot commit a
resource. **The working directory:** the adapter table's new
`Confined` marks a CLI that reads nothing outside its working
directory; for such a provider (`xai`) the owner and the architect
start in `/work`, which holds every clone, `/work/.swarm`, the notes
and the resources, and the prompt's `# Repository` gains the line
`- Working directory: /work, holding every clone; the repository is
/work/<first name>`. The alternative, starting them in the first
clone and mounting the others inside it, would put foreign trees in
a repository's working tree; and moving every provider's owner and
architect to `/work` would lose the first repository's own
instructions (`CLAUDE.md`, `AGENTS.md`) that `claude` reads from its
working directory, so the change is `grok`'s alone. The cost: a
`grok` owner or architect does not read the first repository's
project instructions from its working directory either. `grok`'s
argv passes no `--add-dir` whatever it is given. The default persona
prompts name `/work/.resources`; a swarm's own persona prompts
written before this step still name `/resources`, which the
maintainer edits. *To confirm at the next `xai` run* (and by the
opt-in `TestRealGrokTakesTheArgv` with a scratch copy of a
logged-in home): the session is created and the member reads a
resource under its `--cwd`.
