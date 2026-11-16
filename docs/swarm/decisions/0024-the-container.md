# 0024. The container

Moved verbatim from `docs/swarm.md` section 14, item 24, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**The container:** run as the host user's uid (`--user`), `--pull
never`, a per-run `--name`, the prompt files and output in `<run>/io`
mounted at `/work/.swarm`, resources at `/resources/<path>` (at
`/work/.resources/<path>` since Step 25a, item 87); the
worker's commits authored
`worker (Daisy swarm) <worker@daisy-swarm.invalid>` (from Step 5,
`<role> (Daisy swarm) <commit_email>`, item 52). Confirmed at
Step 39 for a uid the image holds (checklist item 6); an unknown uid, served by
the entrypoint (item 41), is *to confirm at Step 28*.
