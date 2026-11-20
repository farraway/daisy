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
package for a run it did not end. One Docker could not run (a fault
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
reviewer's prompt shows. A signature in a package the change touches
persists to the maintainer too, as the ruling reads; the maintainer who
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
