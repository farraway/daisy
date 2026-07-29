# Prompts vs. Skills

Daisy has two kinds of instruction file: **prompts** (`prompts/`) and
**skills** (`skills/`). They look similar — both are markdown with a trigger
— but they occupy different layers, and putting something in the wrong one
either strands craft guidance inside Daisy where it can't help you, or
scatters Daisy's own invariants across artifacts that have to stand alone.

## What actually differs

Less than it looks. A skill's `description` and a lazy prompt's `## Trigger`
are the same construct — a context pointer with an on-demand body. Both are
progressive disclosure; both keep a short summary in front of the agent and
load the rest when it becomes relevant. Nor is scope a difference: `daisy
install` copies root skills to `~/.claude/skills/`, `daisy init` copies a
home's skills into the workspace's `.claude/skills/`, and prompts compile
per-home, so either format can be narrow or broad.

**The distinction is governance, not mechanism.** It is a decision about who
maintains a thing and what it is allowed to assume — not about how it
reaches the agent.

Two residual differences follow from that governance choice rather than
motivating it:

- **Provenance.** A skill is *copied* into its install location — a
  distribution of a source that lives elsewhere, which is why refreshing one
  means re-running `daisy install` or `daisy init`. A prompt is compiled
  from `include.txt` into the per-home `AGENTS.md`; Daisy owns the source
  outright and there is no copy to drift from it.
- **Dispatch.** A skill is a discrete unit the agent surfaces by matching
  its `description`, so it is written to stand alone — it may be invoked
  with no Daisy persona in context. A prompt *is* part of that persona, free
  to reference `todo.txt`, `today.md`, `journal.md`, `projects/`, `PLAN.md`.

## The criterion: situation or artifact?

Ask this before anything else:

**Does this capability need to know the user's situation, or only the
artifact in front of it?**

- **Situation** — it needs to know what your projects are, what you did
  yesterday, how you work, what state the day is in → **prompt**. This is
  what the persona layer exists for.
- **Artifact** — it needs to know what is in the file, and nothing else →
  **skill**. It should work in a repo that has never seen Daisy.

The test is about coupling, so it stays stable as the tooling changes. An
earlier version of this document decided the split by asking who owned a
capability's evolution, and rested that on the fact that `daisy optimize`
only rewrites `## Rules` sections in `prompts/`. That is a property of the
current script — the feedback loop is intended to reach skills and scripts
too — so it could not carry the argument for *placement*. Ownership remains
a live question with a different job, treated as its own axis below.

Coupling also gets the historical cases right. `css` looked portable,
became a skill, was converted back to a prompt sixteen days later, and was
finally returned to a skill in 2026-07: it references no Daisy file and
never did. The reversion was justified by the loop's reach, not by the
capability's shape, and it cost real utility — as a prompt it was
unavailable in every workspace that had not been `daisy init`-ed.

## The second axis: what the learning loop must reach

Coupling is not the only question worth asking about a capability. The other
is **who refines it over time** — and the two are independent.

- **Coupling decides the layer.** What does this need to know? A property of
  the capability itself, stable as the tooling around it changes.
- **Ownership decides loop coverage.** Who improves this? A property of the
  maintenance workflow, indifferent to which directory the file sits in.

Collapsing them is the mistake to avoid, in either direction. Placing a
capability in the craft layer does not hand its evolution to someone else:
`css`, `js`, `tdd`, and `agents-md` are skills because they need nothing but
the artifact in front of them, and they remain yours to refine. Nothing
about being a skill makes a capability upstream-owned.

**This creates an obligation.** Because placement follows runtime coupling,
the feedback loop has to follow content wherever coupling puts it. Four
capabilities the user actively refines now live in the craft layer, and
`daisy optimize` currently reaches only prompts — so extending it to skills
is a requirement this architecture created, not an optional enhancement.

Two constraints on that work, recorded here because they shape it:

- **There is no rewrite anchor yet.** `optimize` resolves a prompt file and
  rewrites its `## Rules` section. The migrated skills have no such section
  — those preambles were removed as duplication of the body sections they
  restated. Extending the loop therefore requires deciding what it rewrites
  in a skill: a `## Rules` convention for Daisy-authored skills, a
  whole-body pass, or an overlay leaving the skill intact.
- **Coverage is per-skill, not blanket.** A skill carrying `LICENSE` and
  `NOTICE.md` tracks an upstream source; rewriting Daisy's copy from local
  feedback would fork it and make future sync a reconciliation problem.
  Vendored skills stay outside the loop's write path. Track them; don't
  rewrite them.

## Worked example: splitting `git`

The original `git.md` mixed two things:

- **Resolving a merge or rebase conflict** — find the primary sources for
  each hunk, preserve both intents, never `--abort`, run the project's
  checks. This needs the conflict and the repository history. It needs
  nothing from Daisy.
- **The GitHub PR workflow** — `user-github-*` operations, never merging
  without green CI, opening a `todo.txt` entry when a PR opens, logging the
  merge when it lands.

The tempting cut is craft-versus-obligation: make the MCP operations and the
CI rule portable, and leave only the two task-tracking rules behind. That
splits one coherent workflow across two layers. How PRs are opened, gated,
tracked, and logged is a single situated practice — the `todo.txt` entry is
not an obligation bolted onto the operations, it is part of what "open a PR"
means here.

So the cut follows the coupling, and it lands on the git/GitHub line:
`skills/git/` for conflict resolution, `prompts/github.md` for the PR
workflow entire, pointing at the skill when a conflict appears.

**The general move:** find the seam where situational knowledge stops being
needed. Split there — not at the boundary that produces the neatest-looking
pieces.

## Deciding for something new

Ask the coupling question first. If it isn't immediately obvious:

- **Does it read or write any file Daisy owns** (`todo.txt`, `today.md`,
  `journal.md`, `projects/`, `PLAN.md`)? If yes, that part is prompt-shaped.
- **Would it read correctly in a repo with no `.daisy/` present?** If yes,
  it's skill-shaped. Mentions of `/daisy` commands are the tell — check
  whether they're incidental phrasing or the actual coupling.
- **Does its deliverable feed a Daisy workflow?** `socrates` produces a
  confirmed want or a tested thesis, and both branches hand off to
  `/daisy plan`. That handoff is the coupling; it stays a prompt despite
  technique that would otherwise travel fine.
- **Is there an upstream source that would maintain this even if Daisy
  never existed, and should Daisy's copy track it?** If yes, skill — and
  outside the loop's write path.
- **Then, separately: will you refine this from your own feedback?** That
  answer doesn't move the file. It decides whether the learning loop needs
  to reach wherever coupling has placed it.

If a capability turns out to be a hybrid, don't force one verdict — find the
seam, as in the `git` example.

## The resulting shape

A **situated layer** that knows the user — `personal`, `daisy`, `plan`,
`retrospective`, `socrates`, `github` — and a **craft layer** that knows the
work — `agents-md`, `css`, `js`, `tdd`, `git`, `writing-great-skills`.
`praxis` sits with the prompts: mostly portable implementation guidance, but
its step-tracking rule writes `PLAN.md`.

## See also

- Root `AGENTS.md` — "Creating a New Prompt" and "Creating a New Skill"
- [`prompts/daisy.md`](../../prompts/daisy.md)
