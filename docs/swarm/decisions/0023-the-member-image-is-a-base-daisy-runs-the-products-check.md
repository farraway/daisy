# 0023. The member image is a base; Daisy runs the product's check

Moved verbatim from `docs/swarm.md` section 14, item 23, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The member image is a base; Daisy runs the product's check.**
Revision 1's image held no product toolchain, so the worker could not
run the checks its prompt asked for and the read-only reviewers could
run nothing: every check was a model reading text (*What It Actually
Takes to Build a Software Factory*: validators that run lint, types
and tests; *Harness Engineering*: an inner loop the agent runs before
the pull request). Now a product image `FROM`s the base, `check` is
one argv the Task texts name, Daisy runs it in a keyless, networkless
container on each fetched head and posts the result, and the reviewers
get a writable throwaway clone. Reading the forge's CI status for a
head is recorded, not built: it would be a new `Adapter` method, read
into the reviewers' prompt and `status`. The base pins `node:24.11.0-bookworm-slim` by tag
and digest (`@sha256:76d0ed0e…`, added at Step 21 of the plan that
clears the Deferred list from the build host's image cache, the image
`daisy-member:1` is built on), installs the CLIs at
versions given as required build arguments, and adds `ca-certificates`
for `codex`. Step 39 built it (Claude Code 2.1.318, Codex 0.166.0)
and `codex` reached its API with the certificates present; whether it
needs them was not tried, so they stay, for `grok` too, a native
binary as `codex` is.
