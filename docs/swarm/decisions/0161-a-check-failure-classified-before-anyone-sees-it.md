# 0161. A check failure classified before anyone sees it

From Step 10 of the plan implementing the 2026-11-19 swarm roadmap (R3c, 2026-11-19). Specified ahead of its build: `docs/swarm.md` section 7.9, The classification, with the changes it makes marked in sections 2, 7.5 to 7.8 and 11.

**The maintainer's ruling of 2026-11-19 (R3c):** a check failure in a
package the change does not touch, or one matching an infrastructure
signature (`i/o timeout`, `server selection error`, `connection
refused`, `network not found`, `too many open files`), is rerun once
automatically; one that persists goes to the maintainer with cause
`environment` (the vocabulary of decision
[153](0153-a-cause-on-every-member-stop.md); Daisy's own stops' causes
are decision [154](0154-daisys-own-stops-carry-a-cause.md)'s); neither
kind reaches the worker or the reviewers. Every infrastructure failure
of the swarm's first ten days was diagnosed by hand (intervention I6):
a port-reuse race (#155), a 5 s deadline in a test the change did not
touch (#177), "too many open files" (#119), leaked CI containers; each
cost the worker a run and the reviewers a round on a fault that was
not the change's.

**Where it runs.** In the collection's step 4 (section 7.6), once the
check, or the dependency step whose failure is the check's result
(decision [122](0122-a-dependency-step-and-the-swarms-cache.md)), has
exited non-zero or timed out, its test services removed, and before the push, the
post and the workspace's `checks/` (decision
[158](0158-the-jobs-workspace-is-the-memory.md)): so nothing of the
first failure is posted, pushed for CI to report or written where a
member reads. A check, or dependency step, that timed out at the wall
clock is a check failure too, classified by the signatures alone in the
output it wrote before it was stopped, since `go test` names no failing
package for a run it did not end. (*Amended at Step R3-7, on the maintainer's answer of 2026-11-21: for a Go head, one holding a `go.mod`, a `go.work` or a `.go` file, an output naming no failing package is not classified, whatever signature it holds; this decision's As built for that step, below.*) One Docker could not run (a fault
already) and a test services file refused (the product's file, which
the worker fixes) are not classified, and are settled as before.

**A package the change does not touch, for Go.** The swarm's products
are Go, and `go test` names each failing package on a line of its own,
`FAIL<tab><import path>`, after its output. A failure is in untouched
packages when its output names at least one failing package and every
one is untouched; a failure that names none (a `gofmt`, `vet` or
lint step, another language's runner) is the change's unless a
signature matches. A failing package is mapped to its directory by the
module path of the root `go.mod` at the head; one outside that module
counts as touched. It is touched when a file the change alters (step
3's `git diff --name-only <base>...<fetched>`) lies in its directory,
below it (its `testdata`, an embedded tree, a nested package, so that a
root package, whose tests may walk the whole tree, is touched by every
change) or above it (a `go.mod`, `go.sum` or `Makefile` at the root
touches every package), or when it imports, directly or through other packages of
the module, test files' imports included, a package so touched. The
imports are read on the host from the head's `.go` files in the mirror
(Go's parser, imports only), every file whatever its build tags, so an
edge only a tag enables still counts: the reading errs toward touched,
since a failure counted the change's reaches the worker as before,
while one wrongly counted untouched would be withheld from it. Running
`go list -deps -test` in the image was the alternative: exact for the
default tags, but a container per classification, and blind to the
`integration` tag gridapp's lane builds with. **The limit:** for any
other language Daisy knows no package, so only a signature classifies
its failure; a Go module below the root, or a `go.work`, counts its
packages touched.

**The signatures** are the ruling's five, matched in the failed step's
whole output (`check.out`, or `deps.out` for the dependency step),
ignoring case (mongod writes "Too many open files", Go's syscall error
"too many open files"); no sixth is added here. Matching only within a
failing package's lines was the alternative: `go test` without `-v`
prints a passing package's output not at all, so the two differ only
for a verbose check, at the cost of a reading of `go test`'s layout
for every language.

**The one rerun.** A classified failure is run again once, as it ran:
the check alone on a clone made again, with its services made again;
the dependency step's failure, the step and then the check. The first
run's output is kept as `<run>/check-1.out` (`deps-1.out`), and
`<run>/result` says why (`check rerun: <reason>`). The rerun's result
is the check's: green, it is posted as any green check, the first
failure in the result and on stdout alone; failed and not classified,
it is the change's, posted as any failing check; failed and classified
again, the failure persists. The check started again after the
maintainer lifts the block is not a rerun (below), so no worker run's
check is rerun twice.

**What persists goes to the maintainer** as a failure of the work
(section 7.7's T17, as a second failed fetch from a worker's clone
does): `failed: check environment: <reason>`, the reconciler's note on
the issue and `swarm:blocked`, the run's row with cause `environment`,
its commits kept as its run's ref and its slice's `kept`; nothing is
pushed, nothing is posted on the pull request, no `checks/<sha>.log` is
written, so neither the worker nor the reviewers see it. The reason
names the failing packages or the signature, the two outputs are in
the run's directory, and the note is on the issue, which no worker's or
reviewer's prompt shows. (*Amended by decision
[162](0162-ci-builds-the-declared-image.md): a failure in a package the
change touches is the worker's, whatever signature its output holds;
the next sentence is the decision as first taken.*) A signature in a
package the change touches persists to the maintainer too, as the
ruling reads; the maintainer who
finds it the change's says so in the comment that lifts the block and
retags the cause (`swarm unblock <n> --cause <word>`, decision 154).
Holding the collection itself at step 4 until the block is lifted, and
then running the check again without a worker run, was the alternative:
cheaper after the fix, but the stop would leave no row until the run
was collected (the cause written late or superseded, and `unblock`'s
retag finding no collected run), and a run uncollected for hours holds
the issue by section 6's rule 2 in a state no other run is in.

**After the block is lifted** the role rule launches the next worker
run, which starts from the held run's kept commits (section 7.8, The
start). A run that starts from kept commits not yet pushed, and adds
none, has those commits to push: it is not *no commits* (section 7.6,
step 1), whose rule was written when every start was pushed or the
base, and means nothing to push. That reading also covers a slice
continued from a worker's `BLOCKED:` whose kept work its next run
finds complete, which R1's start left falling into *no commits*.

**What the maintainer sees:** a rerun on `apply`'s stdout and in the
run's `result`, whatever its end; a failure that persists as the
issue's block, its note, `status`'s Waiting for you (`blocked`) and the
row's cause, which R3's measure counts (stops with cause
`environment`: 0).

**Amended by decision [162](0162-ci-builds-the-declared-image.md)
(Step 10b of the plan implementing the 2026-11-19 swarm roadmap, R3),
specified ahead of its build.** On the maintainer's answer at the
checkpoint, a signature classifies a failure as the environment's only
when no package the change touches fails: a failure whose output names
a touched failing package is never classified and goes to the worker,
whatever signature it holds. An output naming no failing package is
classified by a signature alone, as before, and a timeout likewise
unless its output already names a touched failing package. (*Amended at Step R3-7, on the maintainer's answer of 2026-11-21: for a Go head, one holding a `go.mod`, a `go.work` or a `.go` file, an output naming no failing package is not classified, whatever signature it holds; this decision's As built for that step, below.*)

**As built: the classification as a function (Step R3-6 of the plan
implementing the 2026-11-19 swarm roadmap).** `reconcile.Classify`
classifies from what it is given alone (the failed step's output, the
change's paths, the head's Go facts and whether the step timed out),
and `Git.GoFacts` reads the facts from the mirror with `ls-tree` and
`cat-file`, Go's parser reading imports only; the rerun and the
environment failure that call them are Step R3-7's. Where section 7.9
is silent, each reading errs toward touched, as this decision's import
reading does. Imports resolve through every module of the head and
every `vendor` directory, not the root module's alone (the alternative:
a root package importing a module below the root through a `replace`
or a `go.work`, or a vendored package, that the change alters would
have been untouched). A package's test files' imports count for it
alone (counting them for its importers was the alternative: exact in
no direction and only more often touched, since Go builds a package's
tests into no other). A path at or below a `go.work`'s directory is
outside the root module, so a `go.work` at the root makes every
failing package touched (the maintainer's answer of 2026-11-20 that a
module under a `go.work` counts as touched; reading the `go.work`'s
`use` lines was the alternative, which a module it names outside the
repository would have defeated). A failing package naming no directory
of the head that holds a `.go` file, and a directory whose file Go's
parser refuses, are touched (untouched was the alternative, the
dangerous direction). A directory with no `.go` file at the head is touched
by a changed file in it or below it, so a package the change deletes
or moves away is touched and its importers with it (reading touched
packages among the head's alone was the alternative: an importer
failing on the deleted package would have been untouched, as the
review of the step found); a module below the root whose path is not
read, and a submodule (whose files the mirror does not hold, its
change a changed path naming its directory), are held imported by
every package with an import (their importers otherwise unknown;
reading the root `go.mod`'s `replace` lines was the alternative, which
a submodule's module path, unread, would still have defeated). A local
`replace` of a `go.mod` or a `go.work`, and a symbolic link naming a
directory of the repository, resolve an import as Go does (reading
modules by their declared paths alone was the alternative: a
`replace` whose directory declares another path, or an import through
a linked directory, would have left its importer untouched, as the
review of the step found). A `FAIL<tab>` line's path ends at its first
blank or tab, whatever follows it (reading only the time and the
bracketed notes `go test` writes was the alternative: a form not
foreseen would hide a touched package and let a signature classify
it); a head with no `go.mod`, `go.work` or `.go` file reads none, as
another language's. A signature is matched ignoring ASCII case alone
(Unicode's folding was the alternative, a Kelvin sign then reading as
`k`), and the reason names the one that comes first in the output (the
list's first was the alternative). Tests: `TestClassify`,
`TestClassifyFailLines`, `TestClassifySignatures`,
`TestClassificationReason`, `TestGoFacts`.

**As built: the rerun and the environment failure (Step R3-7 of the
plan implementing the 2026-11-19 swarm roadmap).** The collection calls
the classification in step 4, before the push, and runs a classified
failure again once; one classified on its rerun is `failed: check
environment: <reason>`, cause `environment`, nothing pushed, posted on
the pull request or written to `checks/`. On the maintainer's answer
of 2026-11-21 ("don't classify"), an output that names no failing
package is not classified when the head is Go's, whatever signature it
holds, since `go test -json`, gotestsum or a prefixer hides the
`FAIL<tab>` lines a touched package's failure would be read from. A
head is Go's by its facts alone: a `go.mod`, a `go.work` or a `.go`
file, the test that already decides whether its `FAIL<tab>` lines are
read. Recognising Go's output by its text (`=== RUN`, `--- FAIL:`,
`"Action":`, a `FAIL<tab>` not beginning its line) was the
alternative: it keeps a Go product's dependency step, `gofmt` step and
a check stopped before `go test` named a package classified by a
signature, but a form not foreseen (another runner, another prefixer)
would let a signature classify a touched package's failure, the
dangerous direction; with the head's reading those failures reach the
worker, as before R3. The change and the head's facts are read only
for an output holding a `FAIL<tab>` line or a signature, the only ones
that can be classified (reading them for every failure was the
alternative, the facts costing one `cat-file` per `.go` file of the
head). The dependency step runs again only when it failed before the
rerun, so a check's rerun is the check's alone (the step and then the
check, as at first, was the alternative: a second dependency step that
the first's success makes needless). Tests:
`TestSwarmCheckRerunOnceThenPosted`, `TestSwarmCheckEnvironmentHeld`,
`TestSwarmCheckNotRerun`, `TestSwarmCheckRerunRestartedAfterItsApplyDied`,
`TestWorkspaceReplayRerunCheck`,
`TestClassify`.
