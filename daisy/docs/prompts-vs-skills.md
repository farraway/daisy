# Prompts vs. Skills

Daisy has two kinds of instruction file: **prompts** (`prompts/`) and
**skills** (`skills/`). They look similar — both are markdown files with a
trigger — but they exist for opposite reasons, and mixing them up produces a
system that either bloats Daisy's always-active persona with things that
didn't need to be there, or scatters Daisy's own invariants into files that
can silently go unloaded.

## The distinction

A **prompt** (a hat) is identity-scoped behavior bound to state Daisy owns.
You wear it — once triggered it's active for the whole engagement, with no
completion condition — and it's meaningless without the surrounding identity's
state already in place: `todo.txt`, `today.md`, `journal.md`, `projects/`,
`PLAN.md`.

A **skill** is a portable, bounded procedure with a trigger and a completion
condition. You call it — it does a bounded thing and returns — and it works
identically cold, with no Daisy persona loaded at all.

## The portability test

Would this file's content behave identically if invoked by a bare agent that
had never loaded Daisy's persona or touched any `.daisy/` file?

- **Yes** → it's a skill. Put it in `skills/`, regardless of where the idea
  came from.
- **No** → it's a hat-gesture. Put it in `prompts/`.

## Worked example 1: `css` (a skill)

`css`'s content — CUBE CSS and Every Layout conventions — has zero
dependency on anything Daisy owns. It would give exactly the same guidance
whether or not Daisy has ever been loaded in the workspace; it doesn't read
or write `todo.txt`, doesn't reference `+project` tags, doesn't care whether
a home is even configured. Applying the test: portable → skill. It now
lives at `skills/css/SKILL.md`, installed natively into `~/.claude/skills/`
at `daisy install` (it's a root skill), callable independent of Daisy
entirely.

## Worked example 2: `github.md` (a hybrid — skill plus hat-bound obligations)

The original `github.md` mixed two different things:

- A portable procedure: use the `user-github-*` MCP tools for public GitHub,
  never merge without green CI. This part doesn't care that Daisy exists.
- Two hat-bound obligations: add a `todo.txt` entry when a PR opens, log the
  merge when it lands. These only make sense because Daisy owns `todo.txt`
  and `today.md` — a bare agent with no Daisy persona has nowhere to put
  either of them.

Applying the test to the whole file gives two different answers for its two
halves, which is the signal to split rather than force one verdict. The
portable procedure became `skills/github/SKILL.md`. `prompts/github.md`
shrank to just the two hat-bound rules, which now name the skill for the
underlying GitHub mechanics rather than re-describing it.

**The general move:** when a capability has both a portable core and
Daisy-specific obligations wrapped around it, split it — the portable core
becomes a skill, the obligations stay behind as a short prompt that invokes
the skill by name.

## Deciding for something new

Ask the portability test question first. If the answer isn't immediately
obvious, these narrower questions usually settle it:

- **Does it read or write any file Daisy owns** (`todo.txt`, `today.md`,
  `journal.md`, `projects/`, `PLAN.md`)? If yes, at least part of it is
  hat-bound.
- **Would you be comfortable running this in a completely unrelated,
  non-Daisy repo, with no `.daisy/` present at all?** If yes, it's a skill.
- **Does it have a natural "done" state** where the agent finishes and hands
  control back, or does it just keep informing behavior indefinitely once
  active? A skill completes; a hat doesn't.

If the capability turns out to be a hybrid, don't force a single verdict —
split it along the `github.md` pattern above.

## See also

- Root `AGENTS.md` — "Creating a New Prompt" and "Creating a New Skill"
- [`prompts/daisy.md`](../../prompts/daisy.md)
