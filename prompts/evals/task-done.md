# Eval: Task Completion Workflow

Evaluates whether the `done` workflow correctly marked a task complete across all files.

## Inputs Expected

- `todo.txt` before — state before the done workflow ran
- `todo.txt` after — state after
- `today.md` before — state before
- `today.md` after — state after
- `task_pattern` — the pattern used to find the task
- `completion_date` — the date the task was completed (YYYY-MM-DD)

## Evaluation Criteria

### 1. todo.txt — correct completion format

The matched task must:
- Have its priority stripped (no `(A)`, `(B)`, etc.)
- Have `x YYYY-MM-DD` prepended (using the completion date)
- Retain all other fields unchanged (description, +project, @context tags, original creation date)
- Be moved to the end of todo.txt

### 2. today.md — checkbox update

The corresponding task in today.md must:
- Have its checkbox changed from `- [ ]` to `- [x]`
- Retain the description text unchanged

### 3. Log entry

A log entry must have been added to the Log section of today.md:
- Format: `- HHMM - <description>` (24-hour time, no colons)
- The description should reference the completed task

### 4. No other modifications

No other tasks in todo.txt or today.md should have been modified.

## Output Format

Respond with:

```
PASS
```

or:

```
FAIL
- [criterion]: [specific explanation of what was wrong and what the correct behavior should be]
- [criterion]: ...
```
