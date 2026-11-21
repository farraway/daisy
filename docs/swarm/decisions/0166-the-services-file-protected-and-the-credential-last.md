# 0166. The test services file protected, and the credential last

From Step R3-3c of the plan implementing the 2026-11-19 swarm roadmap, on the maintainer's answers of 2026-11-21 to its Open Questions 29, 32 and 35. Built in that step: `docs/swarm.md` sections 3 (`repos[].services`, `repos[].dockerfile`, `repos[].protected`), 7.2 (the member's argv), 7.4 (the protected paths), 7.6 (step 3) and 7.9 (its rulings, The classification). Amends decisions [160](0160-one-declared-check-environment.md), [161](0161-a-check-failure-classified-before-anyone-sees-it.md) and [162](0162-ci-builds-the-declared-image.md).

**The gap.** Step R3-3 gave the reviewers the test services file read
at the head they review, its `x-daisy-env` given as `--env` lines after
the credential's. The file was not protected, so a worker could change
it, and a later `--env` of a name wins in Docker: a worker's commit
could redirect a credentialed run that reads the file at its head (a
proxy, a provider's base URL, a credential's home). The maintainer
chose two of the four options put to him (Open Question 32): (c) the
file protected by default, as the declared Dockerfile is, so no worker
changes what the runs beside it are given; and (b) the credential
after every declared variable, so no declared variable overrides a
credential, a principle rather than a list of names. (a), a list of
names `x-daisy-env` refuses (base URLs, proxies, homes), was the
alternative to (b): every new provider or tool would add a name. (d),
the reviewers reading the file at `<base>`, was the alternative to
(c): it closes the reviewers' path but not the worker's next run.

**The file protected.** The file `repos[].services` names, or
`compose.test.yaml` when the field is absent, joins the declared
Dockerfile in the protected check (7.6, step 3), matched as written,
whole, whether or not the commit holds it: a worker creating
`compose.test.yaml` where there was none would give the next runs
services and variables as surely as one changing it. With another path
declared, `compose.test.yaml` is a file as any. The Task texts name it
after the patterns and the Dockerfile, unless a pattern matches it, so
every worker's and architect's text, and the batch texts, now carry
the protected sentence. Protecting the default path only when the
commit holds the file was the alternative: it leaves the creation
open.

**The credential last.** A slot's `--env <home>=/credential` moves
after the `(variables)` and `(services)` lines, so the last value of
its name is the slot's. An `env` credential is given by `--env-file
/dev/stdin`, which Docker ranks below every `--env` whatever their
order (the CLI reads the files, then appends the `--env` values, the
later winning), so ordering cannot protect it: a member's declared
line naming that variable is left out of the argv, and the check,
which holds no credential, is given it. Passing the `env` credential
as `--env <VAR>` with its value in the docker CLI's environment, last
on the argv, was the alternative: it changes how the credential
reaches Docker (section 7.2's `--env-file` row, confirmed live at Step
39) and puts the value in a process environment Daisy otherwise never
writes. Refusing a services file whose `x-daisy-env` names a secret's
variable, as `repos[].env` refuses one, was the other: the check would
lose a variable it may use, and a refusal would stop the reviewers'
services for a name the credential's own placement makes harmless.

**The paths refused** (Open Question 29). `dockerfile` and `services`
refuse a NUL, a `.git` element and a space at either end, one widening
of "a file's path in the repository": none of them names a file git
tracks. A `.git` element anywhere is refused, not only a path under
the top `.git/`, since git refuses the element at any depth; a path
beginning `.git` without being that element (`.gitea/`, `.github/`) is
a path as any. Spaces inside a path are kept, and only a space is
refused at the ends (not a tab or another blank); refusing every
blank at the ends was the alternative.

**The dependency step classified by a signature whatever the head**
(Open Question 35). The dependency step builds nothing of the change,
so a network signature in its output (`go mod download`'s `i/o
timeout`) is the environment's: its failure, exited non-zero or timed
out, is classified by the five signatures alone, whatever the head,
and neither the change nor the head's Go facts are read for it. Step
R3-7's Go-head rule (an output naming no failing package is not
classified) applies to the check alone. Keeping the package rules for
the dependency step's output (a `FAIL<tab>` line naming an untouched
package classifying it, a touched one never) and lifting only the
Go-head rule was the alternative: a dependency step runs no package's
tests, so its output's `FAIL<tab>` lines name nothing of the change.
