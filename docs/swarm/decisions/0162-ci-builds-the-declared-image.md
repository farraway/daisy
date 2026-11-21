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

**Amended: the declared Dockerfile is protected by default (the
maintainer's answer of 2026-11-20 to Open Question 8 of the plan
implementing the 2026-11-19 swarm roadmap; as built at its Step
R3-1b).** Where "Where the Dockerfile lives" left the path to the
maintainer's `protected` list, a default being a new stop for a
worker's change, the maintainer has made that stop: a worker's change
to the file `repos[].dockerfile` names (changed, deleted, renamed
away, or created where there was none) is the failure *protected path
changed*, as a change to a `protected` path is (7.6, step 3), its note
naming the path among the others, each once, in the diff's order. The
path joins the same check rather than being written into the file's
`protected`, so `init` keeps the maintainer's list as written and a
`protected` of `[]` still protects the declared Dockerfile. It is
matched as written, whole: `protected`'s rule for an entry without `/`
(its last element, at any depth) would make a root `Dockerfile` protect
every file of that name in the repository, which the field does not
declare. The Task texts name it among the protected paths (7.4),
after the patterns and only when none of them matches it already, so a
worker is told before its change is refused rather than after. As
built, the field is read and written: a relative, clean path, not `.`
nor ending `/`, one line, as `services` is (whose rule now refuses `.`
too, the repository's root being no file's path), written after
`image` and kept by `init`. Alternatives: copying the path into
`protected` when `init` writes the file (a list the maintainer edits
would then lose it silently); matching it as a `protected` pattern
(the reach above); leaving the Task texts as they were (a worker
refused for a path it was never told of). Tests:
`TestDeclaredDockerfileIsProtected`, `TestValidateNamesTheField`,
`TestFormatDeps`, `TestWizardKeepsTheFilesDeps`.

**As built: the comparison as a function (Step R3-4 of the plan
implementing the 2026-11-19 swarm roadmap).** `declare.CompareCI`
compares, from what it is given alone (the forge's kind, the
repository's declaration, its test services file or none, and the
workflow files' paths and contents as read), and returns section
7.9's lines; telling them (`apply`, `status`, notify, `init`) and
reading the files from the mirrors at `<base>` are Step R3-5's. Which
files the kind reads is `declare.WorkflowFiles`, so the step that
reads them can read those alone. Where 7.9 is silent, the smallest
reading consistent with it, each written into 7.9's *As built*: a
step's `run` split as the shell splits it (quotes hold a blank, a cut
and a line break; `\` takes the next character), not by lines first,
so a quoted `&&` cuts nothing (splitting at blanks before removing
quotes was the alternative: `printf 'FROM %s\n…'` would be words of
the quote's text); a workflow expression `${{ … }}` one word's text,
since the reference form's tag holds one with blanks and would
otherwise read as five arguments, so as no build; a redirection a
word, since the reference form's `-f <dockerfile>` would otherwise read
as a redirection; `sh -c`'s string not read as commands (7.9's
"nothing expanded"; reading it was the alternative, a second shell
inside the first). Docker's options read as its flag parser reads
them, every option taking a value but a fixed list of those that take
none, so that a build's context is found among options in any order
and a run's command begins after its image (the last word as the
context was the alternative: `docker build . -t x` is common). A build
without `-f` builds `Dockerfile`, as docker does (requiring `-f` was
the alternative: a declared `Dockerfile` built the default way would
have been warned of as not built); the paths compared clean (`./x` is
`x`). A `defaults.run.working-directory` of the job or the workflow is
a step's `working-directory`, as the runner applies it. The network's
line only with `dockerfile` declared: absent, the targets run on the
runner in the job's container, which is on the job's network (applying
it there was the alternative, a line on every such job). A CI limit
without `:<hard>` is `<n>:<n>`, as docker reads it. A variable given
otherwise names each run of the check given it otherwise (one line for
the first was the alternative, hiding a second job's). The lines in a
fixed order, each once; a file refused, when no file is compared, its
line alone (the image's and the others' would describe a file not
read). Kept as 7.9 has them: with no image job, the targets, the
services and the variables are read from none, so the build's line
comes with the check's targets, the file's services and the
variables, each as run by none. Tests: `TestCompareCIReference` (the
reference job, verbatim from 7.9, gives none), `TestCompareCIMutations`,
`TestCompareCICommands`, `TestCompareCIWorkflows`,
`TestCompareCITwoImageJobs`, `TestCompareCICheckNotMake`,
`TestCompareCIRealistic`.

**As built: the comparison told (Step R3-5 of the plan implementing
the 2026-11-19 swarm roadmap).** `reconcile.CIDiffers` reads, for each
declared repository, its mirror at `<base>` with git's read commands
alone (the kind's workflow directories, `declare.WorkflowDirs`, and
the test services file at the same commit) and is the one source of
`apply`'s stderr lines, `status`'s `ci differs <name>`
(`waiting.ci_differs`, and `connect`'s), notify's item and `init`'s
readiness lines. It fetches nothing: `apply` reads it in step (8), so
as the pass's own fetch left the mirrors when the pass fetched, else
as an earlier fetch did (fetching for it in a pass that fetches
nothing, a paused one, was the alternative: 7.9's "writing nothing"
and a paused swarm's quiet weighed against a line a fetch late). The
lines last printed are the process's memory, never a file (7.9's "the
process's first pass", as for a merge with no check, decision
[145](0145-a-merge-with-no-check-is-warned-of.md)), and the set is
every repository's lines together, printed whole when it changes
(each repository's lines apart was the alternative: the precedent's
one set kept). Notify's record is `<state>/notified`'s `ci differs
<name>`, by name, as 7.9 has it. Where 7.9 was silent: a test services
file refused at `<base>` is its own line,
`<file>: not compared: <reason>`, first, and the comparison made
without it (`declare.CompareCIRefused`; leaving the repository
uncompared until the file reads was the alternative, hiding CI's
differences behind a file the check refuses anyway), told only when a
workflow runs on a pull request, as every line is; a mirror git cannot
read is a warning on `apply`'s and `init`'s stderr with nothing told
by notify that pass, and `status`'s error, as its other reads are.
Tests: `TestSwarmCIDiffersTold`, `TestSwarmCIDiffersServicesFile`,
`TestSwarmInitReadinessCIDiffers`.
