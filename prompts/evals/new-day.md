# Eval: New Day Workflow

Evaluates whether the `new-day` workflow produced a correct `today.md`.

## Inputs Expected

- `todo.txt` — state before new-day ran (source of tasks)
- `today.md` — the newly generated file (output to evaluate)
- `date` — the date of the new day (YYYY-MM-DD)

## Evaluation Criteria

Evaluate each criterion and assign pass or fail with a specific explanation if it fails.

### 1. Task carry-forward

- Priority A tasks from todo.txt must appear in the **Now** section of today.md
- Priority B tasks from todo.txt must appear in the **Next** section
- Tasks with no priority (inbox) must appear in the **Inbox** section
- Cancelled tasks (`z` prefix in todo.txt) must NOT appear in today.md
- Completed tasks (`x` prefix in todo.txt) must NOT appear in today.md

### 2. Format

- today.md must contain exactly these H4 headings: `#### Agenda`, `#### Tasks`, `#### Log`, `#### Retrospective`
- Each task must appear as `- [ ] <description>` (not `- [x]`, not bare text)
- The Log section must be present and empty (a blank line after `#### Log`, then the next heading)
- No blank lines within the Log section between entries (there are no entries yet)

### 3. Default inbox checklist

The Inbox section must include these default items (in any order):
- Check calendar for upcoming events
- Workout
- Check that todo.txt is up-to-date
- Plan day
- Retrospective

### 4. Date

The file header or agenda section should reflect the correct date.

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

Be specific: name the exact task or section where the issue occurred.
