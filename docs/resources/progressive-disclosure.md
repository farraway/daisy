# Progressive Disclosure in Claude Code

**Source:** Developers Digest — "Progressive Disclosure in Claude Code"
**Applied in Daisy:** Session 2026-06-09 — architecture confirmed; trigger stub cleanup; personal.md heading clarified

---

## The Core Principle

Industry convergence (Cloudflare, Anthropic, Cursor) on a single pattern: **load only what you need, when you need it.** Don't pre-load tool schemas, context, or instructions that may never be used in a given session. Every token in the static context costs quality — the model operates worse as the context fills.

The numbers cited:
- Anthropic's tool search vs. always-loaded tools: 85% token reduction; MCP eval accuracy Opus 4 49% → 74%
- Cloudflare's code-execution approach vs. JSON tool calls: 98.7% reduction
- Cursor's agent token reduction: 46.9%

---

## What This Looks Like in Practice

**Tools as files.** Instead of loading all MCP schemas upfront, give the agent a filesystem and bash. It can grep/find/read what it needs. Every agent knows how to use bash. The MCP server becomes a TypeScript API in an isolated sandbox; the agent generates code to call it rather than consuming static schema.

**Skills as lazy-loaded prompts.** Only the frontmatter/trigger is static. Full skill content loads on demand. Skills can reference sub-skills hierarchically — the agent reads down the chain only as far as the current task requires.

**Memory as simple markdown.** No embeddings, no vector DB, no complex retrieval. Just files the agent can read, edit, and search. Simple for humans = simple for agents. Works better in practice than embeddings for the workloads these agents handle.

**Progressive context eviction.** Old tool results can be cleared from context as they become less relevant. Context is not append-only.

---

## Daisy Architecture Assessment

### Already implemented — design validated

Daisy's lazy loading system (`~` prefix in `include.txt`) implements the core progressive disclosure pattern exactly:

- `~daisy` → only the `## Trigger` stub is embedded in AGENTS.md; full prompt loads on demand
- `~retrospective`, `~github`, `~plan`, `~socrates` — same
- The generated `home/{home}/AGENTS.md` for a live home is 130 lines — lean

The 2026-07 two-layer split took this further: craft capabilities (`css`,
`js`, `tdd`, `git`, `agents-md`) became skills, which cost nothing at all
until the agent reaches for one — a rung below lazy stubs on the same
ladder.

The build pipeline (`build-prompt.sh`) extracts only the `## Trigger` section for lazy stubs. This is the correct pattern. The resource confirms this design is sound and is what the industry is converging on.

**Daisy's file-based memory** (today.md, todo.txt, journal.md, projects/) also validates the "memory as simple markdown" principle. No embeddings needed.

### `personal.md` — full inclusion is correct

`personal` has no `~` prefix: it is fully included on every session. This is the right call — communication style and personal context should always be present, not conditional on a trigger. The resource supports this: progressive disclosure applies to task-specific context. Session-invariant context (who the user is, how to communicate) belongs in the static load.

Note: `personal.md` contains a `## Trigger` section, which renders inline as a heading in the generated AGENTS.md rather than functioning as a lazy-load trigger. This is harmless but slightly misleading. The heading could be renamed to `## When to apply` or `## Context` to make it clear it's not a lazy-load trigger.

### Hierarchical prompt chaining — not yet implemented

The resource describes skills referencing sub-skills: the agent reads a top-level stub, finds a reference to a sub-skill, reads that, and so on — loading context incrementally. Daisy's prompts are flat: each prompt is independent, loaded only when its own trigger fires.

A future extension could support `@include <prompt-name>` references within prompts, enabling a prompt like `plan.md` to pull in `praxis.md` only when execution begins rather than loading it separately. Not a current gap with real cost — just an architectural ceiling.

### Trigger stub length

The `daisy.md` trigger stub (~14 lines) is the longest. The others are 4–6 lines. These are all within the static context window. They're already fairly tight; no changes needed.

---

## Daisy Application

| Insight | Daisy status |
|---|---|
| Lazy-load context on demand | ✅ Implemented — `~` prefix in include.txt |
| Static context should be minimal stubs only | ✅ Only `## Trigger` sections in AGENTS.md |
| Memory as simple markdown files | ✅ Core design (today.md, todo.txt, journal.md) |
| Always-needed context fully included | ✅ `personal.md` without `~` |
| Hierarchical sub-prompt chaining | Not implemented — future extension |
| `personal.md` trigger heading is misleading | Minor — rename `## Trigger` → `## Context` |
