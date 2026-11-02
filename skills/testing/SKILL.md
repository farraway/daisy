---
name: testing
description: Testing as falsification — a claim per step, tests at pre-agreed seams, every side effect behind one Ops boundary with a Fake, a probe that breaks the code to prove the test can fail, and the review that attacks the claim. Use when writing tests, building a feature or fixing a bug test-first, porting code against an oracle, designing where tests will go, choosing between a fake and a mock, or when a step's claim has to be stated.
short_description: Claims, seams, fakes, probes: tests that can fail
---

A test is an instrument for falsifying a claim. It earns its place when it
has been seen to fail for the right reason, and it stays only while it can
still fail. Everything below follows from that: what a claim is, where the
tests that check it live, what stands in for the world while they run, how
to prove a green suite is more than green, and when to stop.

Read the project's `AGENTS.md` and any contract document (`docs/cli.md` or
its equivalent) before writing a test. The contract is the oracle; the test
is its executable form.

## The claim

Every unit of work, a step, a fix, a port, carries one claim: a sentence a
reviewer can refute without reading the implementation. "Cases 54 and 55
pass" is a claim. "`init` then `clean` leaves `git status` and the three
config keys as before" is a claim. "Improved error handling" is not.

- The claim is written before the code, in the plan or the issue, next to
  the seams it will be tested at.
- It names the oracle: a contract row, a known-good file, a literal value,
  the old implementation's bytes.
- It is met when the tests at the named seams pass **and** the probes (below)
  show those tests would have failed without the change.

## Seams: where tests go

A **seam** (Feathers) is a place where behaviour can be observed or altered
without editing in that place. Tests live at seams and nowhere else.

**Seams are agreed before the first test.** Write down which seams a step
tests at and confirm them with the user. An unconfirmed seam gets no test.
This is how effort lands on the paths that matter instead of every edge.

**Design the seam, then test-drive inside it.** The architecture, which
modules exist, what crosses each boundary, is decided before the red/green
loop starts. The loop fills a shape; it does not discover one. A test that
reaches past the agreed seam into internals is wrong even when it passes.

**The two seams every program has:**

1. **The program entry.** One function, `Run(ops, stdin, stdout, stderr,
   args) -> exit code` or the equivalent, that `main` calls and that tests
   call with a `Fake`. Functional tests drive the whole program through it
   and assert on exit code, streams, and the files or records left behind.
2. **The domain packages.** Each has a public surface a unit test can
   exercise with simple values. Tests here cover the logic the functional
   tests would need too many cases to pin.

## Ops: one boundary for every side effect

Everything the program does to the world, file system, processes, clock,
environment, network, terminal prompt, goes through one value, `Ops`,
passed in at the entry. Nothing above `Ops` touches the operating system
or a third-party client directly.

**Three implementations, and only three kinds:**

| Implementation | Role |
|---|---|
| `Real` | the production adapters |
| `ReadOnly` | wraps `Real`; refuses every write. A command whose contract says it writes nothing receives this and cannot break the promise |
| `Fake` | an in-memory implementation with a shortcut: a temp directory, a fixed clock, a recorded prompt answer |

**The Fake is a working implementation, not a mock.** It returns the same
types `Real` returns, never richer, never a stand-in object. Tests assert
on what the program left behind, the files, the records, the output, not
on which `Ops` methods were called. A call log may exist for debugging and
for testing `Ops` itself; a test of a command or workflow that asserts on
it is coupled to implementation and will break on a refactor that changes
nothing.

**Fake and Real must agree.** Enforce interface parity mechanically: a
compile-time interface in a typed language, or a check at import that every
public method of `Real` exists on `Fake` with the same signature and
annotations. A Fake that drifts fails the whole suite at once, not one test
at a time.

**Ops is thin.** Its methods take simple values (strings, ints, small typed
records) and return simple values or small dedicated result types. Decomposing
a domain object into those values is the job of the layer that calls `Ops`,
never of `Ops` itself. The caller stays at one level of abstraction.

### The layered form, for larger codebases

For a program with several backends (a chat API, a ticketing system, a pager,
a key-value store), `Ops` is a bundle of one typed wrapper per backend, and
the layers above it are fixed:

```
Controllers        parse input, build a Job, dispatch
  └─ Workflows     orchestrate; speak only domain language; use domain objects via the Job
       └─ Domain objects   own the logic and records; decompose themselves into simple values for Ops
            └─ Ops         one typed wrapper per backend; translate domain names to API names; return small typed results
                 └─ Clients   third-party SDKs and HTTP; never seen above Ops
```

Each layer calls only the layer below. The rules that make this testable:

- An `Ops` method accepts domain-named keyword parameters and returns a
  dedicated result type (`CreatedRoom(room_id, title)`), never a raw SDK
  object or an untyped dict, and never `**fields` pass-throughs. The field
  mapping to the API lives inside the method.
- Every public `Ops` method is fully typed, and the Fake is validated
  against the Real at import.
- `Ops.fake(seed)` builds the whole bundle from one seed dictionary. The
  seed is the fixture: records, people, on-call lists, catalogue entries.
- Workflows are the primary test surface: functional tests build a Job
  over `Ops.fake(seed)`, run the workflow, and assert on domain-object
  state and the notifications produced. Domain objects get unit tests with
  the same Fake. `Ops` wrappers get tests only where they hold logic
  (multi-account search, escalation); thin delegation is not tested.
- No workflow test asserts on the call log.

## The loop

```
claim → seams agreed → red → green → probe → sweep → review
```

- **Red before green.** Write the failing test at the agreed seam; see it
  fail with the message you expect; then write only enough to pass it.
- **One slice at a time.** One seam, one test, one minimal implementation.
  Vertical slices: each test responds to what the last cycle taught. Never
  all tests first, then all code; bulk tests pin imagined behaviour.
- **Probe.** With the suite green, break the implementation on purpose,
  remove the bound, invert the condition, skip the rollback, and confirm
  the test that owns that behaviour fails. Revert. A test that stays green
  under its own mutation is not a test; fix or delete it. Record the probes
  with the claim: "with `Repoint` returning its input unchanged, case 54
  fails on the repointed-link assert".
- **Sweep.** A defect found once is a class. Search for its siblings before
  closing it, and fix them under the same test.
- **Refactoring is not part of the loop.** It belongs to review.

**Green means no regression, not correctness.** Dijkstra: testing shows the
presence of defects, never their absence. The probe is the only evidence a
green test is load-bearing, and review is the only check on the claim
itself.

## Oracles and expected values

- Expected values come from an independent source: a literal, a worked
  example, the contract. A test whose expected value is computed the way
  the code computes it is tautological and passes by construction.
- **Porting against an oracle.** When replacing an implementation, the old
  one is the oracle: pin its bytes (output, files, exit codes) in
  characterization tests first, make the suite red on exactly the cases the
  port owns, then make it green. Where the oracle is wrong, do not reproduce
  it: record the deliberate difference as a decision, with the reason.
- **Hermetic always.** A test builds its own machine: temp root, temp home,
  temp registry, pinned environment, pinned working directory. No test may
  drive the program through `Real` with the process's own environment; the
  one that did wrote junk into a live data file. Every variable the program
  reads from the environment is set by the test.

## Anti-patterns

- **Implementation-coupled**: mocks internal collaborators, tests private
  functions, asserts on call counts or order, or verifies through a side
  channel. Tell: it breaks under a refactor that changed no behaviour.
- **Tautological**: the expected value is recomputed the code's way.
- **Horizontal slicing**: all tests, then all code.
- **Unprobed green**: a test never seen to fail. Treat as absent.
- **Fake richer than Real**: a Fake returning fields or types the Real does
  not; the tests pass against a world that does not exist.
- **Reading the developer's machine**: a test that resolves the real home,
  registry, `$HOME` or working directory.
- **The claim as adjective**: "better", "robust", "cleaned up". Not
  refutable, so not a claim.

## Rules

1. One refutable claim per step, written before the code, naming its oracle.
2. Tests only at seams the user has agreed; design the seam, then test inside it.
3. Every side effect goes through `Ops`; `main` and the functional tests call the same `Run`.
4. Three `Ops` kinds only: `Real`, `ReadOnly`, `Fake`; the Fake returns what Real returns, and parity is checked mechanically.
5. Assert on outcomes, never on which `Ops` methods were called.
6. Red before green, one slice at a time, refactoring outside the loop.
7. Probe every green test by breaking its subject; a test that survives its mutation is fixed or deleted.
8. A defect is a class: sweep its siblings under the same test.
9. Expected values are independent literals or the contract; when porting, the old bytes are the oracle and every deliberate departure is a recorded decision.
10. Hermetic: a test builds its own machine and pins every environment variable the program reads.
