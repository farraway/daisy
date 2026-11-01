# The Infinite Software Crisis

**Source:** Jake Nations, Netflix — AI Engineer World's Fair keynote
**Applied in Daisy:** Session 2026-06-08 — research phase in plan workflow, essential/accidental rule in praxis.md

---

## The Pattern That Keeps Repeating

Every generation of software engineers hits a wall where complexity outpaces their ability to manage it. The '60s had the original software crisis (Dijkstra: "bigger computers, bigger programming problems"). The '70s–2000s cycled through C, OOP, agile, cloud. Today we have AI — the same pattern at infinite scale.

Fred Brooks (1986, *No Silver Bullet*): there is no single innovation that gives an order-of-magnitude improvement in software productivity. The hard part was never the mechanics of coding (syntax, typing, boilerplate). It was always understanding what to build and designing the solution. Tools eliminate the mechanical work. The core difficulty remains.

---

## Simple vs. Easy (Rich Hickey, *Simple Made Easy*, 2011)

These words are not synonyms.

- **Simple** — one fold, no entanglement. Each piece does one thing and doesn't intertwine with others. About *structure*.
- **Easy** — adjacent, within reach, low friction. Copy-paste, install a package, generate with AI. About *proximity*.

You can't make something simple by wishing it. Simplicity requires thought, design, and untangling.
You can always make something easier. Just put it closer.

**AI is the ultimate easy machine.** It makes the easy path so frictionless that we stop considering the simple one. Why think about architecture when code appears instantly?

The old trade-off (easy now, complexity later) used to work because complexity accumulated slowly enough to refactor. AI destroyed that balance. Complexity now compounds faster than we can comprehend it.

---

## Essential vs. Accidental Complexity (Fred Brooks)

Every system has both:

- **Essential complexity** — the fundamental difficulty inherent to the problem itself. Users must pay. Orders must be fulfilled. This is why the software exists.
- **Accidental complexity** — everything added to make the code work: workarounds, defensive code, frameworks, abstractions that made sense years ago, shims between old and new systems.

In a real codebase these are tangled together. Separating them requires context, history, and experience.

**AI makes no such distinction.** Every pattern in the codebase is preserved equally. Technical debt doesn't register as debt — it's just more code. The authentication check on line 47 and the gRPC-acting-like-GraphQL code from 2019 are both "patterns to preserve."

---

## The Three-Phase Approach

Nations' solution to the AI complexity spiral: structure work as three distinct phases, each with a human checkpoint before proceeding.

### Phase 1: Research

Feed everything relevant up front: architecture diagrams, documentation, design docs, key interfaces. Use the agent to map components and dependencies — but probe it, correct it, iterate. This is not a one-shot process.

**Output:** A research document. "Here's what exists. Here's what connects to what. Here's what your change will affect." Hours of exploration compressed into minutes of reading.

**Human checkpoint (highest-leverage moment in the process):** Validate the analysis against reality. Catch errors here — prevent disasters later. This is where you tell the AI which complexity is essential and which is accidental. It cannot make that distinction without you.

### Phase 2: Plan

With valid research in hand, create a detailed implementation plan: real code structure, function signatures, type definitions, data flow. Detailed enough that a junior engineer could follow it line by line.

Architectural decisions happen here, not during implementation. Clean service boundaries, correct business requirements, prevention of unnecessary coupling — all before a line of code is written.

**Human review speed:** The plan can be validated in minutes. You know exactly what will be built.

### Phase 3: Implement

With clear research and a clear plan, implementation should be simple. That's the point. The context stays clean and focused. Three focused outputs, each validated before proceeding. No abandoned approaches, no "wait actually" moments, no dead code from context drift.

A background agent can run this phase while you work on something else — because you've already done the thinking.

---

## What This Preserves

"We're not using AI to think for us. We're using it to accelerate the mechanical parts while maintaining our ability to understand."

- Research is faster
- Planning is more thorough  
- Implementation is cleaner
- **Thinking, synthesis, and judgment remain with us**

"It works" isn't enough. There's a difference between code that passes tests and code that survives production. Between systems that function today and systems that can be changed by someone else in the future.

---

## The Deeper Risk: Atrophy

Every time we skip thinking to keep up with generation speed, we're not just adding code we don't understand — **we're losing our ability to recognize problems.**

The instinct that says "this is getting complex" atrophies when you don't understand your own system. Pattern recognition comes from experience: you spot dangerous architecture because you were the one up at 3am dealing with it. AI doesn't encode lessons from past failures.

The three-phase approach bridges this gap — it compresses understanding into artifacts we can review at the speed of generation.

---

## Sometimes You Have to Do It by Hand First

Nations' team tried to refactor a heavily coupled auth system with AI — the agent spiraled. The accidental and essential complexity were too tangled to untangle algorithmically.

Solution: do the first migration manually. No AI — just read the code, understand dependencies, make changes to see what broke. Painful, but it revealed the hidden constraints and invariants. Then feed that manual PR into the research process as a seed example. The AI could finally see what a clean migration looks like.

**"We had to earn the understanding before we could code it into our process."** The three-phase approach isn't magic — it only works because you already understand the system well enough to write the spec.

---

## Daisy Application

| Lesson | Daisy change |
|---|---|
| Research phase before planning | `/daisy plan` step 3: research phase with human checkpoint ("confirmed — proceed?") |
| Human checkpoint at phase boundary | Plan rule 1: "confirm research summary before scaffolding" |
| Essential vs. accidental distinction | `praxis.md` Rule 9: "ask whether complexity is inherent or artifact" |
| Review for accidental complexity | `praxis.md` Fit criterion updated to include essential/accidental question |
| Research section in artifacts | `templates/plan.md` now includes `## Research` section |
| Spec-driven development | The `/daisy plan` → `/daisy execute` flow is exactly Research → Plan → Implement with checkpoints |
