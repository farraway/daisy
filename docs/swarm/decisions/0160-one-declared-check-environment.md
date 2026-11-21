# 0160. One declared check environment

From Step 10 of the plan implementing the 2026-11-19 swarm roadmap (R3, one check, one environment, failures classified, 2026-11-19). Specified ahead of its build: `docs/swarm.md` section 7.9, with the changes it makes marked in sections 1 to 3, 5, 7.2, 7.5 to 7.8 and 11.

**The maintainer's ruling of 2026-11-19 (R3a, R3b):** the product
declares its check environment once (its image, its dependency step,
its test services and its environment variables); the worker's run,
Daisy's check and CI all use it, and Daisy verifies the CI workflow
against the declaration; gridapp adopts `compose.test.yaml`, so that its
integration lane runs in Daisy's check before review. Resources and
notes mount outside the checkout. "The check" ran in four places that
disagreed: the worker's container (image `:9`, with `PATH` and
`GOMODCACHE` workarounds in the swarm's notes about the repository),
Daisy's check (a dependency cache and no network), CI (setup-go,
MongoDB and Redis, an integration lane) and the maintainer's machine.
MongoDB-only tests passed review and failed CI (#48); the resources
mounted inside the checkout broke every worker's `make check test`
(#183, intervention I14) while Daisy's check and CI stayed green.

**The declaration is the one that exists, with what was missing
added.** A repository's `image`, `check` and `deps` in `swarm.json` and
its test services file in the product's repository (decision
[134](0134-a-products-test-services-beside-its-check.md)) already
declared four of the five parts once; what was missing was variables
for every run that executes the product's code (`deps.env` reaches the
dependency step and the check alone, by decision
[122](0122-a-dependency-step-and-the-swarms-cache.md), since it names
the cache's place) and one resource limit CI gives gridapp's MongoDB
that the services subset could not say. So `repos[].env`, a mapping of
variable to value, is given to the worker's run, the reviewers' runs,
the dependency step and the check; a name in both `env` and `deps.env`
is refused, so a variable has one declared value; and the services
subset gains `ulimits` (Compose's own key, one integer or `soft` and
`hard`), run as `--ulimit`, because gridapp's CI runs `mongo:7` with
`nofile=64000` after mongod aborted at Docker's default ("too many
open files", gridapp#201) and the file would otherwise declare an
environment CI's integration lane does not have. A new declaration
file holding all five parts was the alternative: it would move fields
the maintainer edits today and split the services from the product,
where the maintainer placed them ("it should be attached to gridapp
rather than the swarm", decision 134); `env` in the product's file, as
an `x-daisy-env` allowed without `services`, was the other, but
`x-daisy-env` is the services' address book, given only beside them,
and a variable such as `GOTOOLCHAIN` is the swarm's choice of how the
image runs the product, as `image` is.

**The dependency step stays the check's.** Daisy's check runs offline,
so the dependency step fills the swarm's cache for it (decision 122);
the worker and the reviewers are on the network and fetch with their
tool into the image's own cache, which the product's image makes
writable by the run's uid (gridapp's is not yet: a fix to its image,
the maintainer's). Giving the members the swarm's cache read-only was the
alternative: the tool's first fetch of a module the change adds would
fail; read-write would let a member write what other jobs' checks
read. The cache's place decides no result (Go verifies every module
against `go.sum`), so the environment is one in what does: the image,
the variables, the services, the argv and, below, the paths.

**One set of paths: the check and the dependency step move to
`/job/work/<name>`.** Decision
[159](0159-one-folder-per-run-and-the-builder-plan.md) gave every
member one folder, `/job`, its checkouts at `/job/work/<name>`, the
resources, the notes and the job's workspace beside them as children
of `/job` and never in a checkout, and left the check and the
dependency step their clone at `/work`, for R3 to decide. R3 adopts
0159's layout for the resources and the notes as it stands (R3b asks
nothing more of it), and moves the check's clone and the dependency
step's to `/job/work/<name>`, their working directory, with nothing
mounted beside it: a worker's `make check` and Daisy's check then see
the same absolute paths, which a test that prints or compares a path
(a panic's stack, a golden file, a `go vet` position) reads. Keeping
`/work` was the alternative, as 0159 left it: the check has no job,
but the paths are part of the environment the ruling makes one, and
the move costs a mount point Docker makes in the container's own
layer, outside any bind source. The move follows the members' (R1's
step that moves them to `/job`): before it, the worker's checkout is
still `/work`.

**The test services beside a reviewer are in** (a feature decision
134 left out, "not asked for", which the roadmap folds into R3): a reviewer's clone
of the head may build and test it, and without the services and the
variables a reviewer that runs the integration lane to probe a claim
reads a failure the worker and the check do not have. The owner, the
architect and the QA, which hold a clone of every declared repository,
get neither: each repository's services and variables would meet in
one container, and none of them runs the check.

**What "Daisy verifies the CI workflow" checks, and when.** The
comparison reads, at each repository's `<base>` as the pass fetched it,
the workflows its forge runs on a pull request (`.gitea/workflows/`,
`.forgejo/workflows/` or `.github/workflows/`, the first that holds
one, for kind `gitea`; `.github/workflows/` for `github`), and
compares three things a head's CI result turns on: the targets (each
`make <target>` the check runs that no workflow step runs, and each
one a step runs that the check does not; a check that is not `make`,
as its whole argv), the services (the file's against the jobs'
`services`, by name, image, `--tmpfs` and `--ulimit`) and the variables
(each declared `env` and `x-daisy-env` variable against what a job
running the check's targets sets). It does not compare the toolchain:
CI's setup-go and the image's Go are each pinned by the product, and
Daisy cannot read an image's toolchain without running it. A mismatch
is a warning, never a block: a block would be a new stop, which only
the maintainer makes, and none was asked for. It is told as decision
[145](0145-a-merge-with-no-check-is-warned-of.md) tells a repository
with no check: `apply`'s stderr at the process's first pass and again
only when the set of lines changes, `status`'s Waiting for you (`ci
differs <name>`), notify once, and `init`'s readiness. A verb of its
own was the alternative, run when the maintainer remembers to; each
pass alone, the unit's log would repeat it every interval. Workflows
are YAML that uses block scalars (`run: |`), which the services
reader refused by line, so the reader gains literal and folded block
scalars, for both files: one reader, one subset; a workflow it still
cannot read is named, not compared.

*Amended by decision [162](0162-ci-builds-the-declared-image.md): CI
builds the declared image and runs the check's targets in it; the
paragraph below is the decision as first taken.* **CI in the product's
image** is not specified: the image is local to
the swarm's host (`--pull never`), and CI can run in it only from a
registry its runner pulls. The ruling's "CI uses it" holds for the
targets, the services and the variables, which the comparison keeps
equal; the toolchain is the product's to pin in both.

**gridapp adopts the file** as the product's own change, made by its
maintainer: `compose.test.yaml` with `mongo:7` (its data on a `tmpfs`,
`nofile` at 64000) and `redis:7` and the two `GRIDAPP_TEST_*`
variables, and a check that runs the integration lane
(`make test-integration`) beside them. Then the workarounds in the
swarm's notes about gridapp, a `PATH` and a `GOMODCACHE` to set by
hand, an image to rebuild for each module, and the rule to say in a
pull request that only CI runs a MongoDB-only test, are deleted (R3d).

**Amended by decision [162](0162-ci-builds-the-declared-image.md)
(Step 10b of the plan implementing the 2026-11-19 swarm roadmap, R3),
specified ahead of its build.** On the maintainer's answer at the
checkpoint, CI builds the declared image itself, from the product's
Dockerfile, which `repos[].dockerfile` names, with the repository's
root as its context, in a step on the runner's Docker (never as a
job's `container:`, which `force_pull` would pull), and runs the
check's targets in it, the checkout copied in at `/job/work/<name>`
and the job's network joined; the comparison checks that a job builds
from the declared Dockerfile and reads the targets, the services and
the variables from those jobs' runs in the image. A mismatch stays a
warning. The limit: a runner whose Docker does not hold the base
cannot build the image.

**As built: `repos[].env` (Step R3-1 of the plan implementing the
2026-11-19 swarm roadmap).** The worker's run, the dependency step and
the check are given the repository's `env`, one `--env` per name in
name order, before `deps.env`'s (the dependency step and the check) and
`x-daisy-env`'s (the worker and the check, beside the services); a name
in `env` and `x-daisy-env` is given twice, the file's last, which Docker
keeps. Without `env` each argv is as before, byte for byte. Validation
reads each name in order and names the field: not a variable name, a
secret's (`forge.token_env`, a `keys` `env`, `DAISY_MAINTAINER_TOKEN`,
the names `<state>/env` holds), also in `deps.env`, or a value holding a
newline or a carriage return (`\r`, as `ssh_command` and `services`
refuse it). `init` keeps the file's `env`, as it keeps `deps`, and
writes a `{}` as absent, which means the same. Narrowed here, each the
spec's least: a provider's credential variable that `keys` does not
name (an `ANTHROPIC_API_KEY` beside a `CLAUDE_CODE_OAUTH_TOKEN` key) is
not refused, since only the swarm's own secrets are named by the spec
and Daisy holds no other; the reviewers' runs are built with their
services by the step that builds those (R3-3), so until it a reviewer
is launched as before; the comparison with CI is R3-4's. Alternatives:
refusing every provider variable (a rule the spec does not make);
giving the reviewers `env` now without their services (half of 7.9's
Beside a reviewer, and a second change to the same launch at R3-3).
Tests: `TestSwarmEnvGivenToEveryRunOfTheRepository`,
`TestSwarmEnvWithoutDepsOrServices`, `TestValidateNamesTheField`,
`TestFormatDeps`, `TestWizardKeepsTheFilesDeps`.

**As built: `ulimits` and block scalars (Step R3-2 of the plan
implementing the 2026-11-19 swarm roadmap).** A test service's
`ulimits` is a mapping of a limit's name (`^[a-z]+$`) to one number,
its soft and hard limit, or to exactly `soft` and `hard`; it is run as
`--ulimit <name>=<soft>:<hard>`, one per limit in name order, after
`--tmpfs` and before `--env`, and `{}` is none. Narrowed here, where
the spec says "integers": a number is digits alone, 0 to
9223372036854775807 (what Docker parses), without a leading `0`
(Compose's YAML, go-yaml v3, reads `064000` as the octal 26624, so the
one file would give Compose and Daisy different limits), so `-1`, `+1`,
`64k` and `064000` are refused; a
`soft` above its `hard` is refused by the file, naming the limit, as
`docker run` would refuse it at the service's start, a swarm fault
rather than the file's reason; a name `^[a-z]+$` that Docker does not
know is not refused here and fails at the start, the spec naming the
shape alone. Alternatives: `-1` for "unlimited", which Docker takes
for some limits and which would put a rule the spec does not make on
which; Docker's list of limit names, which is Docker's to change. The
YAML subset reads literal and folded block scalars, `|` and `>`, alone
or with `-` or `+`, after a key or a list item's dash: the lines after
the header that are blank or indented past the key or the dash, at the
first such line's indentation, a comment, a `%` or a tab past that
indentation being text, chomped and folded as YAML does, and the
values PyYAML 6 gives for the same documents (22,927 generated ones
compared on the code as built, none differing). Where YAML 1.2's text refuses a
leading blank line deeper than the block's first line, the reader
does as libyaml, and so Compose, does: the deeper line sets the
indentation and the first line, less indented, ends the block. An
indentation indicator (`|2`), or any other text after the indicator,
is refused naming its line, and so is a tab before a block line's
indentation (as PyYAML refuses it). A line the reader refuses (a tab
in an indentation, a `%`, a second `---`) is refused only when it is
read as YAML, not when it is a block scalar's text. Also fixed in the
same reader: a flow mapping left open after `{` or a `,` at the end of
its line (`mongo: {`) panicked; it is refused as any flow collection
not closed on its line, and `FuzzParseServices` keeps the reader
without a panic. Tests: `TestParseServices` (section 7.9's draft of
gridapp's file), `TestParseServicesRefused`,
`TestParseYAMLBlockScalars`, `TestServicesArgs`,
`FuzzParseServices`.
