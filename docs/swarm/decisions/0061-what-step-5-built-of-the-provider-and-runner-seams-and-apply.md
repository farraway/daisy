# 0061. What Step 5 built of the provider and runner seams and `apply`

Moved verbatim from `docs/swarm.md` section 14, item 61, on 2026-11-16. In the text, "item N" is decision N of this directory and "section N" is `docs/swarm.md` section N.

**What Step 5 built of the provider and runner seams and `apply`.**
`provider.Providers` holds each provider's adapter, variables, slot
and co-author address, and `declare` validates against it; `Command`
takes the model and effort rather than the persona, and the
directories outside `<W>` (section 9, `D`). `D` holds the resources
as well as `/work/.swarm` and the sibling clones: the maintainer's
container check found Claude Code refusing reads outside its working
directory, and the resources are mounted outside it for every role
(amended by items 87 and 107: one mount, `/work/.resources`, outside
`<W>` only for an owner or architect started in `/work/<first
name>`; section 9, `D`), so the cause is the
same; the alternative, starting the member at `/work`, was not
taken, since the check runs in the repository and
its agent instructions load from there. A clone's address is
`forge.commit_email` and its co-author the hook's, as item 52
decided; the scan for a commit without it reports and never
rewrites (7.6, The trailer), and is built with the worker's
collection, the first that fetches commits. `<run>/result` gains a
`label` line, so a collection retried by a later pass relabels from
the stage the run was launched on, not from the one the issue holds
then. The secret check of `out.md` is built with the first
collection that posts, ahead of the worker's commits and the check
tail. `Ops` gains `After` (item 35) and `FS().Lock` (item 32), the
lock taken by `apply` and by each slot. `# Earlier pull requests`
holds every pull request when the worker starts the next slice
(7.3), as Step 29's row reads it. The forge's retries are built
(section 10). `apply` refuses to run without `--once` until the
lifecycle's step builds the loop (section 11, As built).
