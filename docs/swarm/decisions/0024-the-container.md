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

**Amended by decisions [158](0158-the-jobs-workspace-is-the-memory.md)
and [159](0159-one-folder-per-run-and-the-builder-plan.md) (built at
Steps R1-5 to R1-10 of the plan implementing the 2026-11-19 swarm
roadmap).** A member's container mounts its run's own folder,
`<run>/job`, at `/job`, and in it the prompt files and output in
`<run>/io` at `/job/io` (not `/work/.swarm`), the resources at
`/job/resources/<repository>/<path>` (not `/work/.resources/<path>`),
the clones at `/job/work/<name>`, the notes at `/job/notes` and, for a
run on an issue, the job's workspace `<state>/jobs/<n>` read-only at
`/job/memory`, which the prompt's `# Job` points into (section 7.8, The
run's folder and The prompt). The user, the pull policy, the name and
the commits' author are as above.

**Amended by decision [170](0170-capacity.md)** (R4, specified ahead of its build, Step 12 of the plan implementing the 2026-11-19 swarm roadmap): every container Daisy starts runs `docker run --detach`, its `sh` writing its streams and its exit status to files in its run's directory, so any `apply` collects it (`docs/swarm.md` section 7.10).
