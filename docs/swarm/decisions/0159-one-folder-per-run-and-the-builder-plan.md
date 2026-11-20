# 0159. One folder per run, `/job`, and the builder plan

From Step 9b of the plan implementing the 2026-11-19 swarm roadmap (R1, the job's workspace is the memory, 2026-11-19). Specified ahead of its build, as decision [158](0158-the-jobs-workspace-is-the-memory.md) is: `docs/swarm.md` section 7.8, with the changes it makes marked in sections 1, 2, 7.1 to 7.6, 8, 9 and 11.

Decision 158 put the job's workspace at `/work/.job`, beside the
checkouts in a `/work` that held them, and asked the architect for a
`## For the builder` section inside its plan, a `### Slice <s>`
subsection per slice, from which each `slice.md` was cut. The
maintainer's answers at the stop of Step 9 (2026-11-19) replace both,
add a refusal and widen the start rule:

1. **One folder per run, `/job`**, with the checkout or checkouts at
   `/job/work`, and the resources, the notes, the run's io and the
   job's workspace as `/job`'s other children; `grok`'s working folder
   is `/job`, so it sees everything the run is given; the other CLIs
   start in their checkout and are given `/job` by `--add-dir`.
2. **The builder plan is a document of its own**, which the architect
   writes after its normal plan, consolidated for the builder; no
   section is required inside the plan.
3. **`swarm merge` refuses** a job with no workspace yet whose plan is
   cut on the forge and whose `out.md` is gone (the rollout window).
4. **A revised plan's `Start: base` line** resets the slice's start, as
   the architect's `ANSWER:` and the maintainer's unblock do.

**The folder.** Every member's container mounts `<run>/job` at `/job`,
read-write and the run's own, holding only the mount points `apply`
makes in it, and in it `/job/work` (`<run>/work`, the checkouts at
`/job/work/<name>`), `/job/io` (`<run>/io`, which was `/work/.swarm`),
`/job/notes` and `/job/resources` (which were `/work/.notes` and
`/work/.resources`) and, on an issue, `/job/memory`, the workspace
`<state>/jobs/<n>`, read-only. `/job` is a host directory of the run's,
not a folder Docker makes in the image: the container's own tree is
root's, which `--user` could not write, so a confined CLI's working
folder would stop being one it may write in, as `/work` is now; and
Docker makes a missing mount point inside a bind source on the host as
root, which is why Daisy makes every mount point first, as it makes
`.swarm`'s today. A `tmpfs` at `/job` was the alternative: writable and
gone with the container, but the bind mounts nested in it untried, and
nothing left for audit.

**The workspace in a child, `/job/memory`.** The alternative, the
workspace mounted read-only at `/job` itself with the writable `work`
and `io` mounted over it, puts writable paths inside a read-only mount,
their mount points made inside the workspace that Daisy alone writes
(and in every job's), and leaves a batch run, which has no workspace,
with another `/job`. As a child, no read-only mount (`memory`,
`resources`, a reader's `notes`) holds another mount or anything
writable, and the layout is the same for every run.

**The other CLIs start in `/job/work/<name>`, for every role.** The
alternative, `/job/work` as the worker's and a reviewer's checkout,
would keep two layouts (one checkout for them, a folder of checkouts
for the owner, the architect and the QA, which hold one per declared
repository), the split decision 158 already removed; with one, a path
names its repository as a plan's slice does (`1. <name>: …`), and the
repository's own instructions are read from the working directory as
now. Their `D` is `/job` alone, which holds the checkout too: one
argument for every role in place of a list per role. The check and the
dependency step, which are not members and have no job, keep `/work`
as their clone: R3 decides whether the check's paths follow the
worker's.

**The builder plan reaches Daisy as a second file**, `<run>/io/builder.md`
(`/job/io/builder.md`), beside `out.md`, read only from a run whose text
is a plan. The alternative, a second marked part of `out.md`, would
split the text before its secret check, its classing by first lines and
its post, and a plan that quotes the marker would be cut in the wrong
place; a second file leaves `out.md` the one text the collection reads
as before. Each `## Slice <s>` section of the builder plan, after the
slice's item under the plan's `## Steps`, is that slice's `slice.md`; a
plan with no builder plan gives each slice its step alone. The builder
plan is secret-checked as `out.md` is and not posted: the forge's
human record holds the plan the maintainer accepts, and posting the
builder plan would double each plan's post and give the thread a
second architect text that "the architect's newest plan" (7.3) would
have to tell apart. Whether the maintainer should see it at G2 is open.

**Kept consolidated by re-issue.** `builder.md` is never edited or
appended to: a revised plan comes with its own builder plan, which
replaces it, and an amendment (R5) leaves it as it is unless the
amendment's run, the architect's, writes a re-issued builder plan
beside its `out.md`, which replaces it the same way; each replacement
cuts every slice's `slice.md` again. When an amendment calls for a
re-issue is R5's rule; this decision fixes only the slot. Applying the
amendments to the builder plan in order, as to `plan.md`, was the
alternative: the builder plan would then stop being consolidated, the
one property the maintainer asked of it.

**The rollout refusal.** For a job with no workspace, `swarm merge`
builds it in memory to count the slices; a plan taken cut from its
post, its `out.md` gone, would count short and could close a
multi-slice job early, so the verb refuses it before any write, and the
next pass of `apply` builds the workspace and blocks the issue as
section 7.8's Rollout says.

**The start.** `Start: base` alone on a line, outside a fenced block,
in a plan the architect posted after the kept run ended, resets the
next worker run's start as an answer's does, by the same rule (7.8,
The start).

**Also settled here**, where decision 158's text left gaps: the
collection's step 1 reads the worker's clone at `<run>/work/<name>`;
`branches.md` is written for, and named in `# Job` to, the worker, the
reviewers and the architect, and read by no part of Daisy; a review
whose round can no longer complete is written to `texts/` at the first
collection on the issue after that, so that `texts/` holds every
member text and a reviewer still never finds its pair's review of the
round it reviews.
