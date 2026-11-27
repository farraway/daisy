# 0175. A role's model chosen by the issue's labels

From Step R4-L of the plan implementing the 2026-11-19 swarm roadmap (the maintainer's request of 2026-11-26: "override the model used for certain roles depending upon issue tags … fable for architect runs on architecture issues"). Built in `docs/swarm.md` sections 1, 3 (`personas.<role>.by_label` and its errors), 7.10 (The model by label) and 11, and `docs/cli.md` (`swarm status`). Amends decision [170](0170-capacity.md): a role's entries, which rule 1 takes in order at a launch, are headed on an issue by the entry its labels choose.

**Why.** One model per role, with the fallback after it, runs every job
of the role on the same model, so the maintainer who wants a stronger
model for the architect on an architecture job, and the cheaper one on
a routine job, had to change the persona by hand between jobs.

**The rule.** `personas.<role>.by_label` is an ordered list of entries,
each a fallback entry's fields after a `label`. A run on an issue
launches on the role's entries in order (decision 170, rule 1), and the
first `by_label` entry whose label the issue carries, as the launch reads
it, takes the persona's place at their head; the persona and its
`fallback` follow it unchanged, so pacing, capacity and the slots apply
to it as to any entry. A run with no issue, the owner's batch and the
QA's, never matches. The label is any label: the job class is already
one (`swarm:class-architecture`). The maintainer's ruling of
2026-11-26, in his words: keyed by any label, an ordered list, the first
match wins; the override heads the role's own chain, its `fallback`
after it.

**Alternatives.** Keying the override by the job class alone named one
of its uses and not the label it already is. An override with its own
fallback list, or one that waits for its own model rather than falling
back to the persona's chain, each made a second chain to declare and to
read in `status`; the ruling keeps one chain per role. An entry the
same as the persona or as an earlier `by_label` entry (by provider,
model and effort) was first accepted, the chain holding it once; the
maintainer ruled on 2026-11-26 that it is refused, as a fallback
duplicate is: it chooses nothing the persona or the earlier entry does
not, and `launchedEntry`, which knows a run's entry by those three
fields alone, could stamp the other's `co_author`. One the same as a
`fallback` entry stays valid, the chain holding it once, at its head.

**Left as it was.** The capacity note compares a run's entry with the
head of its issue's chain, so an architecture job on its override gets
no note, and one passed over to the persona gets the note as for a
fallback. A label set on the issue after a run's launch changes the
model of the role's next run only.

**As built (Step R4-L).** `ByLabel`, `Persona.Override`, `Persona.Chain`
and `Persona.All` in `internal/swarm/declare/config.go`, the `by_label`
checks in `checkPersona` and the credential check over `All` in
`internal/swarm/declare/validate.go`, `Format` writing `by_label` after
`fallback`; the wizard keeps the file's, asking nothing. Tests:
`TestValidateByLabel`, `TestFormatByLabel`, `TestWizardKeepsByLabel`.

**As built (Step R4-L, the launch and `status`).** `(*Swarm).chain` in
`internal/swarm/reconcile/capacity.go`, the persona's `Chain` for the
issue's labels, none for a run with no issue, which `runsOn` and
`capacityNote` read; `launchedEntry` over `All`, so a by_label entry's
`co_author` stamps and checks the worker's commits and the notes'
(from Step CA, decision [176](0176-the-co-author-is-the-models.md), no
`co_author` and `launchedCoAuthor`, derived from the run's `persona`
line, in its place).
`status`: `StatusPersona.ByLabel` and a `Personas:` line per entry;
`capacityOf` a line per entry after its role's, `StatusRoleCapacity.Label`
and `StatusBottleneck.Label`, each issue and member counted on the line
its labels choose, its why over that line's chain. Tests:
`TestSwarmArchitectRunsTheLabelsModel` (the first carried label wins:
an entry earlier in the list whose label the issue lacks is passed over,
one later whose label it carries too is not reached),
`TestSwarmByLabelFallsBackToTheRolesChain` (to the persona, then to
the fallback, the capacity note naming the override),
`TestSwarmWorkerCommitsCarryTheLabelsCoAuthor` (`TestSwarmByLabelCommitsCarryItsModel` since Step CA),
`TestSwarmStatusShowsByLabel`.

**As built (Step R4-L, the maintainer's ruling of 2026-11-26).**
`checkPersona` refuses a `by_label` entry with the provider, model and
effort of the persona, `personas.<role>.by_label[<i>]: the same as
personas.<role>`, or of an earlier `by_label` entry, `the same as
personas.<role>.by_label[<j>]`, after its fields and slots. Test:
`TestValidateByLabel`'s two rows and its valid case (an entry the same
as a fallback entry, and the same model at another effort).

**Amended by decision [176](0176-the-co-author-is-the-models.md)** (Step CA of the plan implementing the 2026-11-19 swarm roadmap): a `by_label` entry, as a fallback entry and the persona, carries no `co_author`; its commits' co-author is derived from its provider and model, so an entry the same as a fallback entry stamps the same trailer, and `launchedEntry` is replaced by `launchedCoAuthor`.
