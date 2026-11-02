# Testing

This document is the long form of the `testing` skill. The skill, which an
agent loads when it writes or reviews tests, is reproduced first and
verbatim. The rest of the document is what a reader needs in order to
understand it: the vocabulary the skill uses, the literature each idea
comes from, where this methodology agrees with that tradition and where it
departs, two implementations of the design at different scales, and one
step worked end to end.

The canonical copy of the skill is `skills/testing/SKILL.md` in the Daisy
root. If the two disagree, the skill wins and this document is stale.

---

## Part 1 — The skill


A test is an instrument for falsifying a claim. It earns its place when it
has been seen to fail for the right reason, and it stays only while it can
still fail. Everything below follows from that: what a claim is, where the
tests that check it live, what stands in for the world while they run, how
to prove a green suite is more than green, and when to stop.

Read the project's `AGENTS.md` and any contract document (`docs/cli.md` or
its equivalent) before writing a test. The contract is the oracle; the test
is its executable form.

### The claim

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

### Seams: where tests go

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

### Ops: one boundary for every side effect

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

#### The layered form, for larger codebases

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

### The loop

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

### Oracles and expected values

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

### Anti-patterns

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

### Rules

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

---

## Part 2 — The vocabulary

The skill uses a small set of terms in fixed senses. Each is defined here
once, with the source it is taken from where there is one.

**Claim.** One sentence per unit of work that a reviewer can refute without
reading the implementation. It names what will be observed (cases pass, a
file is byte-identical, a status is unchanged) and against what (the
oracle). The word is chosen over "acceptance criteria" because a claim is
something you can be wrong about. Popper's term is *conjecture*.

**Oracle.** The independent source of expected values. A contract
document, a known-good file, a literal, or, when porting, the bytes the
previous implementation produced. The term is standard in the testing
literature (Howden, 1978); an oracle problem is a test whose expected value
cannot be known independently of the code.

**Seam.** Feathers (2004): a place where behaviour can be altered without
editing in that place. Every seam has an *enabling point*, the place where
you choose which behaviour runs. In Daisy the enabling point for the
program seam is the `Ops` argument to `Run`; for a domain seam it is the
package's public function.

**Functional test.** A test that drives the whole program through its entry
and asserts on exit code, streams and the state left behind. The term is
used in its plain sense, a test of function as a user would observe it, not
in the sense of functional programming.

**Unit test.** A test of one domain package through its public surface, with
simple values in and out. Not a test of one function or one class; the unit
is the module behind a seam.

**Ops.** The one value through which every side effect passes. In the
tradition this is a *port* (Cockburn) with its *adapters*; Go code often
calls the same thing an *Env* or *World* object. The name `Ops` is
deliberate: it is the set of operations the program may perform, and
handing a narrower `Ops` to a command is how the program is kept from
performing the others.

**Real, ReadOnly, Fake.** The three implementations of `Ops`. `Real` is
production. `ReadOnly` wraps `Real` and refuses writes, which makes the
contract "this command writes nothing" a type rather than a promise. `Fake`
is a working implementation with a shortcut, in Meszaros's sense below.

**Test double.** Meszaros (2007), popularised by Fowler (2004): the generic
term for anything standing in for a production object in a test. Five
kinds: a *dummy* fills a parameter and is never used; a *stub* returns
canned answers; a *spy* records calls for later inspection; a *mock* is
pre-programmed with expectations and fails the test itself if they are not
met; a *fake* has a working implementation that takes a shortcut (an
in-memory database, a temp directory, a fixed clock). The skill uses fakes
and forbids mocks above the `Ops` layer.

**Probe.** A deliberate break in the implementation, made with the suite
green, to confirm the test that owns that behaviour fails. The literature's
term is *mutation testing* (DeMillo, Lipton and Sayward, 1978); a test that
fails under the mutation *kills* it, one that stays green lets it *survive*.
The skill's probes are hand-chosen mutations at the seams under test rather
than the exhaustive mechanical kind.

**Sweep.** On finding a defect, search for its siblings before closing it.
The defect is treated as an instance of a class; the fix and the test cover
the class.

**Hermetic.** A test that depends on nothing outside what it builds for
itself: no real home directory, no real registry, no inherited environment
variable, no network. Google's engineering book (Winters, Manshreck and
Wright, 2020) uses the word for tests whose inputs are fully controlled.

**Characterization test.** Feathers again: a test written against existing
code to pin what it does, right or wrong, so that a change can be seen. The
skill uses them as the first move of any port.

**Contract.** The written specification of a command's interface:
arguments, exit codes, output, what it writes, what it commits. In Daisy
this is `docs/cli.md`. The cases are the contract's executable form; a
contract row and a case that disagree is a defect in one of them.

---

## Part 3 — The tradition

The ideas in the skill are old. This section gives each its source, in
roughly the order a reader would need them, so that the skill's choices
can be seen as choices.

### 3.1 Test-driven development

Beck, *Test-Driven Development: By Example* (2003), gives the loop its
canonical form: write a failing test (red), make it pass with the simplest
thing that could work (green), remove duplication (refactor). Two points
from Beck are often dropped and the skill keeps both. First, the failing
test must be *seen* to fail, and to fail for the expected reason, before
any code is written. Second, the tests are meant to drive design: the
interface a test needs is the interface the code gets. The skill keeps the
first and departs from the second, as Part 4 explains.

Beck's later "Test Desiderata" (2019) lists properties a good test has:
isolated, composable, fast, inspiring, writable, readable, behavioural,
structure-insensitive, automated, specific, deterministic, predictive. The
skill's anti-patterns are mostly violations of *structure-insensitive* (a
test that breaks under a refactor) and *predictive* (a test that passes
without the code being right).

### 3.2 Two schools

The split between the *Detroit* (or Chicago, or classicist) school and the
*London* (or mockist) school is about what a test asserts on. The Detroit
school asserts on state: run the operation, inspect the result and the
world. The London school asserts on interactions: the object under test
should have sent these messages to its collaborators, and the test
pre-programs mocks to check that it did. Mock objects were introduced by
Mackinnon, Freeman and Craig in "Endo-Testing" (2000) and the London
approach is worked out in Freeman and Pryce, *Growing Object-Oriented
Software, Guided by Tests* (2009).

Fowler's "Mocks Aren't Stubs" (2004) is the clearest account of the
difference and its consequences. Interaction tests couple the test to the
implementation's choice of collaborators; state tests couple it only to
the observable outcome. The skill is Detroit throughout and says so in
rule 5. One London idea survives in it: Freeman and Pryce's *walking
skeleton*, a thin end-to-end slice built first so that every later test
can run through the real entry. Daisy's `Run` is that skeleton.

### 3.3 Test doubles

Meszaros, *xUnit Test Patterns* (2007), gives the taxonomy in Part 2. The
practical rule the skill draws from it is that a fake is the double you
want at a boundary you own, because it behaves, and a mock is the double
you want only when the interaction itself is the behaviour under test,
which above the `Ops` layer it never is. Meszaros also names the smell the
skill calls *Fake richer than Real*: a double whose behaviour diverges from
production so that tests pass against a world that does not exist. The
mechanical parity check (a compile-time interface, or a signature check at
import) is the skill's answer.

### 3.4 Seams and legacy code

Feathers, *Working Effectively with Legacy Code* (2004), supplies three
things the skill leans on. The *seam* and its *enabling point*. The
*characterization test*, written to learn and pin what code does before
changing it. And the observation that most code is untestable not because
it is complex but because its dependencies are hard-wired; the remedy is to
introduce a seam, and the cheapest seam is a parameter. `Ops` passed to
`Run` is Feathers's *object seam* placed once, at the top, so that every
module below gets it for free.

### 3.5 Architecture that makes testing possible

A design in which every side effect passes through one value is not a
testing trick; it is a known architecture under several names.

Parnas, "On the Criteria To Be Used in Decomposing Systems into Modules"
(1972), is the root: modules should hide the decisions most likely to
change, and the operating system and third-party clients are such
decisions. Cockburn's *hexagonal architecture*, or *ports and adapters*
(2005), gives the shape: the application talks to the outside only through
ports it defines, and adapters implement the ports for each real
technology, test adapters included. Bernhardt's talk "Boundaries" (2012)
names the same split *functional core, imperative shell*: push every effect
to the edge, and the core becomes testable with values alone.

Evans, *Domain-Driven Design* (2003), contributes the *anti-corruption
layer*: a translation layer between your model and a foreign system so that
the foreign system's names and shapes never enter your code. Fowler's
*Gateway* pattern (*Patterns of Enterprise Application Architecture*, 2002)
is the smaller version: one object that wraps access to an external system.
In the layered design of Part 5, each typed `Ops` wrapper is a Gateway and
the `Ops` layer as a whole is an anti-corruption layer.

Ousterhout, *A Philosophy of Software Design* (2018), supplies the test
for whether a layer is worth having: a *deep module* offers a lot of
behaviour behind a small interface; a *shallow* one has an interface as
complex as what it hides. A pass-through layer fails the test and should
go. The skill's "Ops is thin" and Daisy's praxis *deletion test* are
applications of this.

The `ReadOnly` wrapper has a separate lineage. Saltzer and Schroeder (1975)
state the *principle of least privilege*; Miller's work on *object
capabilities* (2006) makes it a programming discipline: an object can
affect only what it has been handed a reference to. Handing a command a
`ReadOnly` instead of a `Real` is that discipline applied to a CLI.

### 3.6 Where tests should sit

Cohn's *test pyramid* (*Succeeding with Agile*, 2009), restated by Fowler
(2012), says: many fast unit tests, fewer service tests, few end-to-end
tests, because tests higher up are slower, more brittle and less precise
about where a failure lies. Google's *Software Engineering at Google*
(2020) classifies tests by *size* (small, medium, large) according to the
resources they may touch, and requires the small ones to be hermetic.
Spotify's *testing honeycomb* (2018) argues the opposite emphasis for
service-shaped systems: most tests at the integration level, few unit tests.

The skill takes a position between these. It names two seams, the program
entry and the domain packages, and expects most of the pinning to happen
at the entry, because that is where the contract is stated and where a
`Fake` makes the test both hermetic and fast. The pyramid's reason for
avoiding end-to-end tests, that they are slow and non-deterministic, does
not apply when the whole program runs in-process against a Fake.

### 3.7 Mutation testing and seeing the test fail

DeMillo, Lipton and Sayward, "Hints on Test Data Selection" (1978),
proposed judging a test suite by seeding the program with small faults
(*mutants*) and counting how many the suite catches. Tools such as PIT for
the JVM and mutmut for Python automate it. The skill's *probe* is the same
idea done by hand at the seams a step touched, and recorded with the claim.
It is also Beck's "see it fail" step applied after the fact, which matters
most for characterization tests, where the test was written green and has
never been observed red.

### 3.8 Specification by example

Adzic, *Specification by Example* (2011), and the earlier
acceptance-test-driven development literature, hold that the specification
should be a set of concrete examples that execute. The skill's contract
document and its cases are that, with one addition: the contract is prose
and the cases are its executable form, kept in step by review. The *golden
master* or *approval test* (Falco's ApprovalTests) is the specific
technique of pinning an output file and diffing against it; Daisy's byte
equality against the bash implementation during the port is a golden
master whose master is the old program.

### 3.9 Why falsification

Dijkstra's remark at the 1969 NATO conference, that "testing shows the
presence, not the absence, of bugs", is the limit every test suite lives
under. Popper, *The Logic of Scientific Discovery* (1934, English 1959),
turned the same asymmetry into a method: a theory is scientific when it
forbids something, and it is tested by trying to make it fail. The skill's
*claim*, *probe* and *review that attacks the claim* are that method
applied to a change. Green is the absence of a refutation so far, and the
work of testing is to make refutation as likely as possible before the
code ships.

Nygard's *Architecture Decision Records* (2011) are the memory this
requires. When a probe or a review shows that the oracle was wrong, the
departure has to be written down with its reason, or the next reader will
restore the bug.

### 3.10 Property-based testing

Claessen and Hughes, *QuickCheck* (2000), test a property over generated
inputs rather than an example. The skill does not require it, but it fits
one place well: pure bounding functions (a slug limited to 100 bytes, a
file name limited to 255) where "for all inputs, the output satisfies the
bound" is the claim. A generated test there is a probe that writes itself.

---

## Part 4 — Where this methodology stands

The skill agrees with most of the tradition and departs from it in five
places. Each departure has a cost, named here.

| The tradition says | The skill says | Why |
|---|---|---|
| Tests drive the design (Beck) | Design the seam first; test-drive inside it | Letting tests discover architecture produces one that is testable and little else. The architecture is a decision; the tests verify it |
| Mock at the boundaries of each unit (London) | One Fake at `Ops`; nothing mocked above it | Interaction tests break on refactors that change no behaviour. A single Fake keeps every test state-based and the whole program runnable in-process |
| Many unit tests, few end-to-end (pyramid) | Two seams; most pinning at the entry | With a Fake the entry test is as fast and hermetic as a unit test, and it is the only place the contract can be checked whole |
| Green is done | Green is no regression; probe, then review | Dijkstra's limit. A test that has not been seen to fail is not evidence |
| Refactor inside the loop | Refactor at review | The loop's job is to reach a probed green; shape is judged against the whole step, not one cycle |
| Characterization tests are transitional | They are the oracle for a port, and byte equality is the claim | A port with no oracle is a rewrite. The cost is below |

**What the departures cost.**

- *Pre-agreed seams can miss the unanticipated.* A test written only where
  the plan said one would go cannot catch a defect in a place no one
  named. The sweep rule and the reviewer's attacker list are the
  mitigations, and they depend on someone asking.
- *Characterization enshrines accidents.* An oracle pins the old program's
  mistakes as faithfully as its intent. During Daisy's port the review
  found several cases where the old implementation was wrong and the suite
  would have defended it. Each needed a recorded decision to overrule the
  oracle. The question "was the old behaviour right?" has to be asked
  explicitly on every characterization case.
- *Falsification is expensive.* Review passes that attack the claim cost
  more than the implementation did on every step of Daisy's port where
  the figures were kept, sometimes by three to one. The cap on passes
  (six for code, three for documents) exists because the search for a
  refutation has no natural end.
- *A single Fake must be faithful.* Everything rests on the Fake behaving
  like Real at the seam. The parity check catches signature drift, not
  behavioural drift; a Fake file system that permits a write Real would
  refuse is a false pass. Testing the Fake against Real at the primitive
  level, where it is cheap, is part of the cost.

---

## Part 5 — The design at two scales

The skill's rules about `Ops` have been applied in two codebases of
different shape. Both are summarised here with their names changed to
the generic ones the skill uses.

### 5.1 The small form: a CLI

Daisy is a single binary with a few dozen commands over files and git. Its
`Ops` is one interface at the level of the operating system: a file
system (`ReadFile`, `WriteFile`, `Stat`, `Symlink`, `Rename`, ...), process
execution, a clock, the environment, the working directory, an HTTP client
and a terminal prompt. `Real` wraps the standard library. `ReadOnly` wraps
`Real` and returns an error from every write. `Fake` is a temp directory, a
fixed clock and scripted answers.

Above `Ops` sit domain packages (`gitx`, `journal`, `todo`, `identity`,
`workspace`, `plan`, `feedback`, `health`), one function per command in a
`commands` package, and a `cli` table that maps words to commands and also
generates the help and the documentation's command reference. The
dependency rule is `cli → commands → domain → ops`; nothing above `ops`
imports the operating system.

The entry is `Run(ops, stdin, stdout, stderr, args) int`. `main` calls it
with `Real`; the functional tests call it with a `Fake` built around a temp
"machine" (root, home, registry, workspace, every environment variable
pinned). An acceptance suite of shell cases drives the built binary the
same way from outside the process, so the contract is checked both
in-process and through the real executable.

Because the port came from an older implementation, the acceptance cases
were characterization tests first: each pinned the old bytes, the suite was
made red on exactly the cases a step owned, and the new code made it green.
Where the old program was wrong the departure is a numbered decision in the
plan.

What this scale does *not* need: typed result objects per `Ops` method, a
bundle of wrappers, or a call log. The domain packages return Go tuples,
and the thinness of `Ops` is what makes `ReadOnly` a wrapper rather than a
policy.

### 5.2 The larger form: a chat-ops bot

A bot that drives several backends (a chat API, a ticketing system, two
paging providers, a service catalogue, a key-value store, email) cannot
have one `Ops` at the operating-system level; each backend has its own SDK.
The design that keeps the skill's rules is layered:

```
Controllers        parse the command, build a Job, dispatch
  └─ Workflows     one function per command; speak domain language only
       └─ Domain objects   Person, Space, Incident, Service, Timeline ...
            └─ Ops         one typed wrapper per backend, bundled
                 └─ Clients   SDKs and HTTP; never seen above Ops
```

Each layer calls only the layer below. The rules that make it testable, in
the form they take there:

- **`Ops` is a bundle.** A dataclass with one slot per backend
  (`teams`, `pager`, `tickets`, `store`, `email`, ...). Each slot is a
  typed wrapper whose methods take domain-named keyword parameters and
  return small dedicated result types, `NamedTuple`s such as
  `CreatedRoom(room_id, title)` or `TicketRef(number, sys_id)`. A raw SDK
  object never crosses the boundary; a `**fields` pass-through is
  forbidden, because it would let API field names leak upward.
- **Translation lives in the wrapper.** If the domain says `mitigation`
  and the ticketing API wants `u_mitigation`, the mapping is inside
  `TicketOps.update`. The domain object calls it with the domain name.
- **The domain object decomposes itself.** `Incident.set_commander(person)`
  pulls `person.email` and `person.name` out and passes strings to `Ops`.
  `Ops` never receives an `Incident` or a `Person`. This is the rule that
  keeps the caller at one level of abstraction and keeps `Ops` thin.
- **The Fake is the whole bundle.** `Ops.fake(seed)` builds every wrapper's
  in-memory counterpart from one seed dictionary: records in the store,
  people and their display names, on-call rosters, catalogue entries,
  tickets. Counters give deterministic identifiers (`mock-room-1`,
  `INC0000001`). The seed is the fixture.
- **Parity is checked at import.** A decorator on each fake class walks
  the real class's public methods and raises if any is missing, differs in
  signature, or lacks a type annotation on either side. The suite fails
  before the first test runs.
- **The bundle keeps a call log**, behind a proxy that records every call
  with its arguments. The log exists for debugging and for testing the
  wrappers themselves. No workflow test asserts on it.

The testing strategy that follows, layer by layer:

| Layer | Test kind | Stands in | Assert on |
|---|---|---|---|
| Controllers | unit | the dispatcher | the arguments handed to the workflow |
| Workflows | functional, the primary surface | `Ops.fake(seed)` | domain-object state and the notifications produced |
| Domain objects | unit | `Ops.fake(seed)` | records, formatting, lookups |
| `Ops` wrappers | unit, only where there is logic | the client | the translation |

A workflow test builds a `Job` over a seeded `Ops.fake`, sets the room and
the author, runs the workflow function, and asserts that the incident now
has a commander with the expected address and that the last notification
is of the expected level and mentions the right word. A refusal test
asserts that the requirement error is raised, that no notification was
sent, and that the state did not change. Nothing in those tests names a
backend.

### 5.3 What the two have in common

- One entry that `main` and the tests share.
- One boundary for every side effect, with three kinds of implementation.
- A Fake that is a working implementation returning what Real returns, and
  a mechanical check that the two agree.
- Tests that assert on what was left behind, not on what was called.
- A written contract the tests execute, and a numbered decision for every
  place the tests were allowed to disagree with the previous behaviour.

Where they differ is only in how thick `Ops` is. The CLI's `Ops` is the
operating system, so the domain packages do all the translation. The bot's
`Ops` is a set of backend gateways, so translation is split: field mapping
in the wrapper, decomposition of domain objects in the layer above. In
both, a reader can find every effect the program can have by reading one
file.

---

## Part 6 — One step, worked

The step is Daisy's port of `init` and `clean`, chosen because its claim
turned out to be false of the old implementation.

**Claim, as planned.** "`init` then `clean` on a scratch repository leaves
`git status` and the three config keys (`user.name`, `user.email`,
`core.sshCommand`) as before."

**Seams agreed.** Identity resolution (default, named, ambiguous); the
symlink set; the idempotence of the ignore-file and permissions-file
edits (applying twice equals once); and init-then-clean restoring `git
status --porcelain` and the three keys.

**Red.** A Go test that sets `user.name` by hand on a scratch repository,
runs `init` then `clean`, and compares the three keys to their values
before. It failed: the old implementation only *unset* the keys on
`clean`, so a value set by hand before `init` was lost. The claim was
true of nothing yet.

**Decision.** The step's claim was kept and the behaviour changed: `init`
now records the keys' prior values in a file the workspace owns, and
`clean` puts them back. The departure from the old behaviour is numbered
in the plan with this reason.

**Green.** The implementation was written; the test passed, as did an
idempotence test that applies `init` twice and compares every written
file, link, mode and key byte for byte.

**Probes.** With the restore step skipped, the restore test fails. With a
non-idempotent ignore block, the idempotence test fails. With `init`
allowed to run `git init` before its refusal checks, the "a refused init
writes nothing" test fails. Each probe is recorded in the step's report.

**Sweep.** The acceptance case for the previous feature had been leaving
hand-set `user.name` values behind, which the restore would now bring
back; its cleanup was fixed as a sibling of the same class.

**Review.** Six passes. The first found writes that happened before a
refusal, and a fix that checks every path before the first write. Passes
two to five circled one seam, which skills belong to `init`, through
three patches, until the seam was reshaped to a single rule (a skill is
`init`'s exactly when its rule file is byte-for-byte the stub `init`
writes) and verified by a test rather than a seventh pass. Pass six's one
finding fell under the same rule.

**What was accepted as is.** Four edges where `clean` is not a byte-exact
inverse, each the old behaviour, each listed in the plan's deferred items
with the record that would close them.

The step took about 490 thousand tokens of implementation and 510
thousand of review. That ratio is the method's price, and the reason the
pass cap exists.

---

## Part 7 — Reading list

In the order a reader new to the subject would take them.

1. Beck, *Test-Driven Development: By Example* (2003). The loop, and the
   insistence on seeing the test fail.
2. Fowler, "Mocks Aren't Stubs" (2004). The two schools and the cost of
   interaction testing, in twenty pages.
3. Meszaros, *xUnit Test Patterns* (2007). The vocabulary of doubles and
   the catalogue of test smells.
4. Feathers, *Working Effectively with Legacy Code* (2004). Seams,
   enabling points, characterization tests.
5. Cockburn, "Hexagonal Architecture" (2005); Bernhardt, "Boundaries"
   (2012). Two short accounts of the shape that makes a single `Ops`
   possible.
6. Ousterhout, *A Philosophy of Software Design* (2018). Deep modules;
   the test for whether a layer earns its keep.
7. Evans, *Domain-Driven Design* (2003), the anti-corruption layer; Fowler,
   *Patterns of Enterprise Application Architecture* (2002), Gateway.
8. Winters, Manshreck and Wright, *Software Engineering at Google* (2020),
   the testing chapters. Test sizes and hermeticity at scale.
9. DeMillo, Lipton and Sayward, "Hints on Test Data Selection" (1978).
   The origin of mutation testing.
10. Adzic, *Specification by Example* (2011). Contracts as executable
    examples.
11. Popper, *The Logic of Scientific Discovery* (1959), chapter 1 and
    chapter 4. Why a claim must forbid something to be worth making.
12. Nygard, "Documenting Architecture Decisions" (2011). The record that
    keeps an overruled oracle from being restored.
13. Claessen and Hughes, "QuickCheck" (2000). Property tests, for the
    bounding functions.
14. Parnas, "On the Criteria To Be Used in Decomposing Systems into
    Modules" (1972). The root of all of it.
