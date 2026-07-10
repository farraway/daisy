# Eval: Log Entry Workflow

Evaluates whether a log entry was added correctly to today.md.

## Inputs Expected

- `today.md` before — state before the log workflow ran
- `today.md` after — state after
- `message` — the message that should have been logged
- `timestamp` — the expected approximate time (HHMM, 24-hour)

## Evaluation Criteria

### 1. Entry placement

The new entry must appear in the `#### Log` section — not in Tasks, Agenda, or Retrospective.

### 2. Entry format

The entry must follow: `- HHMM <message>`
- Time is 4-digit 24-hour format (no colons): `1430`, not `14:30` or `2:30 PM`
- One space after the time before the message (no second dash — verified against `log.sh`'s actual output and every real entry in this home's journal)
- No trailing blank lines between log entries

### 3. Chronological order

The new entry must appear after all existing log entries (oldest first, newest at bottom).

### 4. Message fidelity

The logged message should accurately reflect the provided message. Minor rewording for log style is acceptable; omitting key details is not.

### 5. No other modifications

No sections outside the Log should have been modified.

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
