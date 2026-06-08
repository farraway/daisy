# Eval: Retrospective Quality

Evaluates whether a daily retrospective is substantive and follows the Daisy format.

## Inputs Expected

- `retrospective_section` — the Retrospective section of today.md
- `completed_tasks` — the list of tasks completed today (from today.md `[x]` checkboxes)
- `log_entries` — the log entries from today's Log section

## Evaluation Criteria

### 1. Required sections present

The retrospective must address all three core questions:
- **Successes** — what went well
- **Misses** — what could have been better
- **What Would a Sage Do Next** — strategic next action

### 2. Successes tied to actual work

Successes should reference completed tasks or log entries from today. Generic statements like "made good progress" without specifics are a weak signal.

### 3. Misses are constructively framed

Misses must be framed as learnings or process improvements, not complaints. 
- Acceptable: "Underestimated complexity of X — need to scope time-boxes for exploratory tasks"
- Not acceptable: "Wasted time on X"

### 4. Sage question is strategic

The Sage answer should reflect long-term thinking (what would be most valuable?) rather than just the next immediate task.

### 5. Specificity

At least one entry per section should name a specific task, decision, or event from today — not generic platitudes.

## Output Format

Respond with:

```
PASS
```

or:

```
FAIL
- [criterion]: [specific explanation of what was wrong and what a better response would look like]
- [criterion]: ...
```
