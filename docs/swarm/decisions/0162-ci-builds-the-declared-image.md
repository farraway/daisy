# 0162. CI builds the declared image, and a touched package's failure is the worker's

From Step 10b of the plan implementing the 2026-11-19 swarm roadmap (R3, the maintainer's answers at the checkpoint of 2026-11-19). Specified ahead of its build: `docs/swarm.md` section 7.9 (CI builds the image, The comparison with CI, The classification), with the changes it makes marked in sections 1, 3, 7.6 and 7.7. Amends decisions [160](0160-one-declared-check-environment.md) and [161](0161-a-check-failure-classified-before-anyone-sees-it.md).

**The maintainer's answers of 2026-11-19.** Decision 160 left CI out
of the image ("CI can run in it only from a registry its runner
pulls") and compared CI's workflows with the declaration as a warning.
The maintainer's answer: CI builds the declared image itself, and runs
the check's targets in it; the comparison checks that it does. And
decision 161 read the ruling's "or" so that a signature in a package
the change touches went to the maintainer; the maintainer's answer, to
the question put to him as a rule: a signature classifies a failure as
the environment's only when no package the change touches fails, and a
failure in a touched package always goes to the worker.

**CI builds the image.** The product's CI builds the image the swarm
runs, from the same Dockerfile, and runs the check's targets in it.
What made it possible is where gridapp's CI runs: a Gitea runner on the
swarm's host, its jobs given the host's Docker socket, so a job's
`docker build` runs on the daemon that holds the swarm's images,
`daisy-member-gridapp:1` among them. Four facts fix the form:

- *The base is local.* The Dockerfile builds `FROM` the member base or
  a product base built from it, which no registry holds; a `docker
  build` without `--pull` takes a local `FROM` image as it is.
- *The image is built in a step, never a job's `container:`.* The
  runner's `force_pull` (on for this runner) pulls by name each image
  the runner itself starts, a job's `container:`, its `services` and a
  Docker action, before the job's steps, and a failed pull fails the
  job (read in act's source, its Docker pull executor; the Gitea
  fork's, *to confirm live*); a step's `docker build` is the docker
  CLI's own call to the daemon, which `force_pull` does not reach. A
  local-only tag named as `container:` would be pulled and fail, and a
  tag a step builds does not exist when the job's container is made.
  Naming the image as `container:` with the runner's `force_pull` off,
  or with the image pushed to a registry first, was the alternative:
  the first changes the runner for every repository it serves (public
  images would no longer be refreshed), the second needs a registry the
  host's Docker trusts, which nothing has yet.
- *The cache is the daemon's.* CI's builds and the maintainer's share
  the daemon's layer cache, so an unchanged base, Dockerfile and copied
  file reuse every layer; each job tags its build with the run's own
  tag, so two pull requests that change the Dockerfile at once never
  run each other's image, and removes the tags, leaving the layers.
- *The checkout is copied in, not mounted.* The job's workspace lives
  inside the job's container, which a sibling container cannot bind by
  its path; `--volumes-from` would give the check the job's Docker
  socket too. A second build copies the checkout to `/job/work/<name>`,
  the path Daisy's check and a worker's checkout have (decision 160),
  owned by the uid Daisy's check runs as, and the check's container
  joins the job container's network (`--network container:$HOSTNAME`),
  where the job's `services` answer by their names. That `$HOSTNAME` is
  the job container's id holds because the runner sets no hostname
  (act's source; *to confirm live*). Running the services in steps
  with `docker run`, on a network the step makes, as Daisy's check
  does, was the alternative: it depends on no runner's internals, but
  the comparison would then have to read `docker run`'s every option
  to find a service's image, where the job's `services` are YAML it
  already reads.

**Where the Dockerfile lives: in the product, declared as
`repos[].dockerfile`.** The Dockerfile is the product's, at a path its
`swarm.json` entry names, with the repository's root at the commit
built as its context, so the one file builds the swarm's `image` (the
maintainer's build at `<base>`, `git archive` piped to `docker build
-f <dockerfile> -`) and CI's (at each head, from its checkout), and the
module cache it warms comes from the product's own `go.mod`, not from
archives copied beside it by hand. Keeping it in the swarm repository
and having CI fetch it was the alternative: the product's CI would
need a credential for the swarm repository and would change when the
swarm's did, and decision 134 put the product's environment with the
product ("it should be attached to gridapp rather than the swarm"). A
`{"dockerfile", "context"}` pair was the other: a context other than
the root was not needed, and one path is one field to compare. Daisy
builds nothing from the field in this revision: the maintainer builds
`image` from it, CI builds it at each head, and the comparison reads
it. Daisy building `image` itself at `<base>`, so the swarm's image
never lags the Dockerfile, is not specified. The maintainer protects
the path, as the workflows are protected, by listing it in the
repository's `protected`; a default that protects the declared
Dockerfile would be a new stop for a worker's change, which is the
maintainer's to make.

**What the comparison now checks.** With `dockerfile` declared, a
workflow job is an *image job* when a step of it runs `docker build`
with `-f <dockerfile>` (or `--file`) and the checkout's root as its
context; none differs (`no workflow job builds the image from
<dockerfile>`). The targets, services and variables are then read
from the image jobs alone: the check's targets from the words after
`make` in each `docker run` that follows the build, in its step
or a later one (a target the
check runs that no such run runs differs, as one a step runs that the
check does not); the services from each image job's `services`, and a
run of the check's targets that is not on the job's network
(`--network container:<id>`) differs, since its check reaches none of
them; the variables as given to that run, by its `--env <VAR>=<value>`
or passed through by `--env <VAR>` from the step's, job's or
workflow's `env`. Without `dockerfile`, CI has no declared image to
build, which is one difference, and the rest is read from every job as
decision 160 had it. The comparison does not trace the image a `docker
run` names through the shell's variables, nor compare a build's other
options: it reads a workflow, not CI's results, and whether the image
builds and the check passes in it are CI's to report. A difference
stays a warning, decision 145's form; a block would be a new stop.

**The limit.** CI can build the image only where its runner's Docker
holds the base: a runner sharing the swarm's host's Docker, as here,
or one that builds the base itself first, which is not specified. A
hosted runner (GitHub's) fails at `FROM`, which CI reports and the
comparison cannot see; a registry holding the base would lift the
limit, and is not specified. The swarm is meant to be set up on any
forge and host: this is the gap a swarm on GitHub's hosted runners
meets first.

**A touched package's failure is the worker's.** A failure whose
output names a failing package the change touches is never
classified, whatever signature the output holds; any other is
classified when it is in untouched packages or holds a signature. An
output that names no failing package (another language, a `gofmt` or
`vet` step, the dependency step, a timeout before any package ended)
has none touched, so a signature alone classifies it as before; a
timeout whose output already names a touched failing package is the
worker's. A Go module below the root, or a `go.work`, names packages
outside the root module, which count as touched, so its failures,
signature or not, are the worker's: the reading errs toward the
worker, as decision 161's import reading does. The note a persisting
failure sends no longer says that a signature in a touched package
reaches the maintainer; the maintainer may still find a classified
failure the change's (an untouched package failing through a coupling
no import shows) and retag the row when lifting the block.
