# Agents in Complex Codebases

**Source:** Dex Horthy, HumanLayer — "No Vibes Allowed: Solving Hard Problems in Complex Codebases", AI Engineer World's Fair
**Applied in Daisy:** Session 2026-06-09 — step granularity rule, RPI calibration rule, intentional compaction guidance added to plan.md

---

## The Core Problem

AI works well on greenfield code. It fails on brownfield. The reason is context quality: the model is stateless — all it can do is predict well or badly based on what's in the context window. Give it bad tokens (stale docs, incorrect assumptions, noise, a correction history) and you get bad tokens back.

The solution is not better models. It's better context engineering.

---

## The Dumb Zone

Context fill degrades quality. Around 40% fill, you start seeing diminishing returns — the "dumb zone." Loading too many MCPs, too much onboarding documentation, or letting a session accumulate corrections all push you deeper in.

Practical implication: the amount of context engineering needed scales with task complexity. Simple changes need none. Complex multi-repo changes need aggressive management. Calibrate.

---

## Intentional Compaction

When a session goes off track (or when approaching context limits), the right move is not to keep correcting — it's to compact:

1. Ask the agent to compress what's been learned into a markdown handoff: exact files and line numbers that matter, what was tried, what failed, what the constraints are.
2. Review and tag the handoff.
3. Start a fresh context with the handoff as input.

The new agent starts with curated, compressed truth rather than a correction history that conditions it to keep making mistakes.

**Why corrections compound:** The model sees a history of "I did X wrong; human corrected; I did Y wrong; human corrected." The trajectory primes the next output to also be wrong. Compaction resets the trajectory.

---

## Sub-Agents as Context Isolators

Sub-agents are not for anthropomorphizing roles (frontend agent, backend agent). They're for controlling context. Send a sub-agent to explore the codebase; it returns a succinct result (exact files, line numbers); the parent agent stays lean and does the work.

The sub-agent burns context on exploration so the parent doesn't have to.

---

## Research–Plan–Implement

Three phases, each with compaction (context reset) between them:

**Research:** Understand how the system works. Find relevant files. Stay objective. Output: a document of compressed truth — "here's what exists, here's what connects to what, here's what your change will affect." This replaces exploratory codebase wandering during implementation.

**Plan:** Outline exact steps with file names and line snippets. Include actual code showing what changes. The plan is detailed enough that the least capable model could follow it. Review: errors here compound. A misunderstanding in the plan is not one wrong line — it's a whole implementation going the wrong direction.

**Implement:** With clean research and a clear plan, implementation is mechanical. The context stays small and focused.

**Calibrate to task complexity:**
- Button color change → just do it; no plan needed
- Single-file feature → one research pass, then implement
- Cross-file or cross-repo work → full RPI with a written plan and human checkpoint
- Complex refactor → multiple research iterations before planning

---

## Plan Granularity

Plans with vague steps ("refactor auth module") provide little execution guarantee. Plans with specific steps ("extract `validate_token()` from `auth.py:203` into `token_validator.py`; update callers at `api.py:45` and `middleware.py:78`") allow even a weak model to succeed.

The principle: as plans get more specific, execution reliability goes up and readability goes down. Find the sweet spot for the task. For complex changes, include actual before/after code snippets. For simple changes, file + line range is enough.

---

## Mental Alignment

Plans aren't only for execution — they're for human comprehension. A technical lead can read plans and stay current on how the system is evolving without reading every line of code. This is how you maintain understanding at speed: review research and plan, not just the diff.

"A bad line of code is a bad line of code. A bad line in a plan could be a hundred bad lines of code. A misunderstanding in research could hose the whole implementation."

---

## Don't Outsource the Thinking

AI amplifies the thinking you've done — or the lack of it. There is no prompt that substitutes for understanding. When AI fails at complex problems, the failure is usually in the research or plan, not in the implementation step. That's where human attention is highest leverage.

---

## Daisy Architecture Assessment

### Already implemented

- Research phase in `/daisy plan` before scaffolding
- Human checkpoint between research and planning ("confirmed — proceed?")
- Human checkpoint between plan and execution ("Ready to execute? yes/no")
- Essential vs. accidental complexity framing in plan.md and praxis.md
- Praxis.md Rule 1: "Read before inventing"

### Gaps addressed in this session

**1. RPI calibration** — plan.md now includes a rule that calibrates how much process to apply based on task complexity. Previously the workflow was always invoked at the same depth regardless of task size.

**2. Step granularity** — plan.md now includes a rule that steps must name specific files and, for complex changes, include code snippets showing what will change. The blank `- [ ] Step 1` template was insufficient.

**3. Intentional compaction guidance** — plan.md now includes guidance on recognizing when corrections have accumulated and surfacing this to the user rather than continuing to correct in-place.

### Not applicable to Daisy

- Sub-agents as context isolators — a Claude Code architecture concern, not a Daisy workflow concern
- "Dumb zone" / MCP tool loading — system-level concern, not addressable in prompts

---

## Daisy Application

| Insight | Daisy change |
|---|---|
| RPI calibration by task complexity | Added Rule 10 to plan.md |
| Step granularity: files + code snippets | Added Rule 11 to plan.md |
| Intentional compaction on repeated corrections | Added Rule 12 to plan.md |
| Research → Plan → Implement with checkpoints | ✅ Already implemented |
| Don't outsource the thinking | ✅ Human checkpoints already enforce this |
| Mental alignment via readable plans | ✅ Human review before execute already serves this |
