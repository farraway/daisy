# 0025. An `env` credential reaches the container through `--env-file /dev/stdin`

Moved verbatim from `docs/swarm.md` section 14, item 25, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**An `env` credential reaches the container through `--env-file
/dev/stdin`.** Chosen: `Ops.Start`'s stdin carries `<VAR>=<value>`,
so the value is never on an argv nor in any file Daisy writes; Step
39 confirmed it for `claude` (checklist item 7). Docker keeps it in
the container's configuration while it lives, which `docker inspect`
would show: *to confirm at Step 28*, as Step 39 did not look.
Alternative: `Ops.Exec`/`Start` gain an environment argument and the
argv says `-e <VAR>`.
