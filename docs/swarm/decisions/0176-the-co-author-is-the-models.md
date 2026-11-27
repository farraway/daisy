# 0176. The co-author is the model's

From Step CA of the plan implementing the 2026-11-19 swarm roadmap (the maintainer's ruling of 2026-11-26: "Co-author should always match the model"). Built in `docs/swarm.md` sections 3 (`personas.<role>.co_author`, retired, and its error), 7.1 (step 3, the hook), 7.6 (The trailer, The notes), 7.10 (The fallback, The model by label) and 11 (`init`'s questions), and `docs/cli.md` (`swarm init`). Amends decision [52](0052-every-role-names-its-model-the-commits-author-and-co-author.md): the co-author is no longer declared, and decision [175](0175-a-roles-model-by-label.md): a `by_label` entry carries no `co_author` either.

**Why.** Decision 52 had each persona declare its `co_author`, the whole
`Co-Authored-By:` value, stamped verbatim. A value declared apart from
the model can name another model: a persona whose model was changed by
hand kept the old trailer, and a `by_label` entry with the provider,
model and effort of a fallback entry but another `co_author` stamped the
fallback's (`launchedEntry` knew a run's entry by those three fields
alone; R4-L's deferred item). The trailer is a record of which model
made the commit, so it follows from the model, never from a second
field.

**The rule.** The trailer of a run's commits is derived from the
provider and the model it launched with: `<model>
<noreply@anthropic.com>` (anthropic), `<model> <noreply@openai.com>`
(openai) or `<model> <noreply@x.ai>` (xai), the value `init` wrote by
default before. It is the same on the persona, a fallback entry and a
`by_label` entry, so the entry a run launched on needs no finding: the
collection's unstamped check and the notes' commit read the run's
`persona` line (or its argv, or the persona) and derive it. The hook is
written into every role's clone; there is no `""`, and no way to turn
the trailer off. `co_author` leaves swarm.json on the persona, a
fallback entry and a `by_label` entry alike; a file still holding it is
refused naming the field, `swarm.json: personas.<role>.co_author: no
longer declared: the Co-Authored-By trailer is the launched model's;
run 'daisy swarm init' to drop it, keeping the rest`, and `init` reads
such a file with the field dropped and writes it back without it,
saying so first (`✓ Dropped co_author from <k> entries: the co-author
trailer is now the model's`; the maintainer's ruling of 2026-11-26 on
the step's open question). The
wizard asks no co-author: questions 22–26 are retired, and the
questions after them keep their numbers.

**Alternatives.** Deriving the trailer by default while keeping `""` to
turn it off, or keeping the field and validating it against the
derived value, were the maintainer's alternatives; he ruled that the
trailer always matches the model. Dropping the field silently on every
read was not taken: a file the maintainer edited by hand would keep a
line that does nothing. Renumbering the wizard's questions after 21
was not taken, as decision 79 kept them for the notes' question: the
numbers stay the ones the decisions name.

**As built (Step CA).** `declare.CoAuthor(provider, model)` in
`internal/swarm/declare/config.go`, the `co_author` fields gone from
`Persona`, `ByLabel` and `Entry`, and `retired`, which `Parse` refuses
by name and `LoadDropping`, `init`'s read, drops; `checkCoAuthor` and the
wizard's co-author question removed; `Format` writes each persona line
ending at `effort`. `(*Swarm).launchedCoAuthor` in
`internal/swarm/reconcile/capacity.go` replaces `launchedEntry`, derived
from `launched`; the launch's clone is given the launched entry's
derived value, and `Git.Clone` writes the hook always. Tests:
`TestCoAuthorFromTheModel`, `TestParseRefusesCoAuthor`,
`TestSwarmInitDropsCoAuthor`, `TestSwarmFallbackCommitsCarryItsModel`,
`TestSwarmByLabelCommitsCarryItsModel` (an entry the same as a
fallback entry, R4-L's case), `TestWizardAsksInOrder` without the
co-author question.

**As built (Step CA, the maintainer's ruling on its open question).**
`LoadDropping` returns the objects each retired field was dropped from,
by name, which `Wizard.Dropped` carries to `writeDeclaration`; it prints
`declare.DroppedLine` for each before `✓ Wrote swarm.json`, its reason
the `retired` entry's. Tests: `TestSwarmInitDropsCoAuthor` (three
entries, the line before `✓ Wrote swarm.json`), `TestParseRefusesCoAuthor`
(one entry counted).
