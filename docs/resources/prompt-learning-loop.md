# Build a Prompt Learning Loop

**Source:** SallyAnn DeLucia & Fuad Ali, Arize — AI Engineer World's Fair workshop
**Applied in Daisy:** Session 2026-05-xx — implemented `feedback.sh`, `optimize.sh`, eval infrastructure

---

## Why Agents Fail

Failures are rarely the model's fault. The environment and instructions are weak:

- No rules section in the system prompt (just a role description)
- Static planning or no planning
- Missing tool guidance (which tool to pick and when)
- Poor context engineering (missing or unconfirmed data)

**The three root causes:** adaptability/self-learning, static vs. flexible planning, context engineering.

---

## Prompt Learning vs. Prior Approaches

| Approach | Signal used | Limitation |
|---|---|---|
| Reinforcement learning | Scalar reward | Can't update LLM weights directly |
| Metaprompting | Eval score | Optimizes a number, not the actual failure |
| **Prompt learning** | Failures + natural language WHY | Uses the text domain the agent operates in |

**The key insight:** The explanation of *why* something failed is the most valuable signal. Not just correct/incorrect — but "it failed to follow instruction X" or "it missed this context." That text is rich enough to drive targeted rule improvements.

---

## How the Loop Works

1. **Collect failures** — humans annotate incorrect outputs with natural-language explanations (or use LLM-as-judge with reasoning, not just labels)
2. **Train/test split** — 80% train, 20% test; prevents overfitting to idiosyncratic examples
3. **Optimize** — feed failures + explanations to an LLM; it updates the `## Rules` section of the system prompt
4. **Evaluate** — run the new prompt against the test set; compare scores
5. **Ship or iterate** — accept improvements, discard regressions

The loop is continuous, not one-shot. As new failures accumulate, rerun. Think of it as building expertise specific to your codebase, not generic ability.

---

## The Rules Section is the Target

Adding a well-structured rules section to a bare system prompt drove **+15% improvement** on SWE-bench light with no fine-tuning, no tool changes, no architecture changes. A smaller model (4.1) reached near-parity with a larger one (4.5) after prompt optimization — at two-thirds the cost.

The optimizer should touch only the rules section. Context, tool definitions, and other sections stay static.

---

## Two Co-Evolving Loops

The agent loop is only as good as its evals. Maintain both:

1. **Agent loop** — collect agent failures → optimize system prompt rules
2. **Eval loop** — collect eval failures (wrong labels, low-confidence judgments) → optimize eval prompt

Evals grabbed off the shelf and never improved will give unreliable signal. The same optimization discipline applies to both.

---

## Practical Guidance

- **Don't let evals block you.** Start scrappy with manual review or off-the-shelf evals. Refine both agent and eval prompts as failures accumulate.
- **Multi-agent systems:** optimize each agent's prompt independently; the per-agent improvement propagates to the whole system.
- **Start with success criteria**, then convert to evals. "What would this look like if it worked?" → specific check.

---

## Daisy Implementation

| Component | Location |
|---|---|
| Failure collection | `daisy/scripts/feedback.sh` → `home/{home}/feedback/feedback.md` |
| Optimization loop | `daisy/scripts/optimize.sh` — patches `## Rules` of a named prompt file |
| Eval cases | `home/{home}/feedback/eval/` |
| Usage | `daisy feedback --workflow <name> "<description>"` then `daisy optimize <prompt-name>` |

The `## Rules` section in each prompt file is the target. Every prompt should have one; optimize.sh expects it.
